// Package cron 提供基于 robfig/cron 的定时任务管理能力：支持任务的增删、
// 启停/暂停恢复，以及从 YAML 配置声明式加载任务。
package cron

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// TaskStatus 描述任务当前的调度状态。
type TaskStatus string

const (
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusPaused   TaskStatus = "paused"
	TaskStatusNotExist TaskStatus = "not_exist"
)

// TaskInfo 是任务状态的对外展示结构。
type TaskInfo struct {
	Name     string     `json:"name"`
	Status   TaskStatus `json:"status"`
	NextRun  time.Time  `json:"next_run"`
	CronExpr string     `json:"cron_expr"`
}

// Logger 是 TaskManager 依赖的日志能力，只声明用到的格式化方法，
// 不直接依赖 pkg/logger 的具体实现，方便替换或在测试中注入。
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// defaultLogger 转发到 pkg/logger 的全局日志器。
type defaultLogger struct{}

func (defaultLogger) Infof(format string, args ...any) {
	logger.Infof(context.Background(), format, args...)
}

func (defaultLogger) Warnf(format string, args ...any) {
	logger.Warnf(context.Background(), format, args...)
}

func (defaultLogger) Errorf(format string, args ...any) {
	logger.Errorf(context.Background(), format, args...)
}

// managedJob 是任务在调度器中的运行态信息。
//
// disabled 用 atomic.Bool 承载，而不是靠 TaskManager 的 mutex 保护：
// 任务触发时只做一次原子读取即可决定是否执行，不会在执行用户任务体期间
// 持有任何锁，这样 PauseTask/RemoveTask 等管理操作不必等待正在运行的任务结束。
//
// schedule 在 AddTask 时解析一次并缓存，避免 ListTasks 每次调用都重新解析
// cron 表达式字符串，同时也让「下次运行时间」的计算不依赖调度器是否已 Start。
type managedJob struct {
	entryID  cron.EntryID
	schedule cron.Schedule
	cronExpr string
	disabled atomic.Bool
}

// TaskManager 定时任务管理器。
type TaskManager struct {
	scheduler *cron.Cron
	logger    Logger

	mu    sync.RWMutex
	tasks map[string]*managedJob
}

// NewTaskManager 创建定时任务管理器，logger 为 nil 时使用默认日志器。
func NewTaskManager(logger Logger) *TaskManager {
	if logger == nil {
		logger = defaultLogger{}
	}
	return &TaskManager{
		scheduler: cron.New(),
		logger:    logger,
		tasks:     make(map[string]*managedJob),
	}
}

// Start 启动调度器，开始触发已添加的任务。
func (tm *TaskManager) Start() {
	tm.scheduler.Start()
	tm.logger.Infof("All scheduled tasks started")
}

// Stop 停止调度器，并等待正在运行的任务结束。
func (tm *TaskManager) Stop() {
	ctx := tm.scheduler.Stop()
	<-ctx.Done()
	tm.logger.Infof("All scheduled tasks stopped")
}

// AddTask 添加一个定时任务，name 需要在管理器内唯一，cronExpr 为标准 5 段 cron 表达式。
func (tm *TaskManager) AddTask(name, cronExpr string, task func()) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tasks[name]; exists {
		return fmt.Errorf("task %s already exists", name)
	}

	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return fmt.Errorf("failed to add task %s: %w", name, err)
	}

	job := &managedJob{cronExpr: cronExpr, schedule: schedule}
	wrappedTask := func() {
		if !job.disabled.Load() {
			task()
		}
	}

	entryID, err := tm.scheduler.AddFunc(cronExpr, wrappedTask)
	if err != nil {
		return fmt.Errorf("failed to add task %s: %w", name, err)
	}
	job.entryID = entryID

	tm.tasks[name] = job
	tm.logger.Infof("Task %s added, expression: %s", name, cronExpr)
	return nil
}

// RemoveTask 移除一个定时任务，任务不存在时仅记录警告。
func (tm *TaskManager) RemoveTask(name string) {
	tm.mu.Lock()
	job, exists := tm.tasks[name]
	if exists {
		delete(tm.tasks, name)
	}
	tm.mu.Unlock()

	if !exists {
		tm.logger.Warnf("Attempt to remove non-existent task %s", name)
		return
	}
	tm.scheduler.Remove(job.entryID)
	tm.logger.Infof("Task %s removed", name)
}

// PauseTask 暂停一个定时任务：调度不受影响（仍会按时触发），但触发时不执行任务体。
func (tm *TaskManager) PauseTask(name string) {
	job, ok := tm.lookup(name)
	if !ok {
		tm.logger.Warnf("Attempt to pause non-existent task %s", name)
		return
	}
	job.disabled.Store(true)
	tm.logger.Infof("Task %s paused", name)
}

// ResumeTask 恢复一个已暂停的定时任务。
func (tm *TaskManager) ResumeTask(name string) {
	job, ok := tm.lookup(name)
	if !ok {
		tm.logger.Warnf("Attempt to resume non-existent task %s", name)
		return
	}
	job.disabled.Store(false)
	tm.logger.Infof("Task %s resumed", name)
}

// GetTaskStatus 返回任务当前状态。
func (tm *TaskManager) GetTaskStatus(name string) TaskStatus {
	job, ok := tm.lookup(name)
	if !ok {
		return TaskStatusNotExist
	}
	if job.disabled.Load() {
		return TaskStatusPaused
	}
	return TaskStatusRunning
}

// ListTasks 返回当前所有任务的状态信息。
func (tm *TaskManager) ListTasks() []TaskInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	infos := make([]TaskInfo, 0, len(tm.tasks))
	for name, job := range tm.tasks {
		status := TaskStatusRunning
		if job.disabled.Load() {
			status = TaskStatusPaused
		}

		infos = append(infos, TaskInfo{
			Name:     name,
			Status:   status,
			NextRun:  job.schedule.Next(time.Now()),
			CronExpr: job.cronExpr,
		})
	}
	return infos
}

func (tm *TaskManager) lookup(name string) (*managedJob, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	job, ok := tm.tasks[name]
	return job, ok
}

// TaskConfig 是 YAML 配置中单个任务的结构。
type TaskConfig struct {
	Name     string `yaml:"name"`
	CronExpr string `yaml:"cron_expr"`
	Disabled bool   `yaml:"disabled,omitempty"`
}

// TaskHandlerFunc 是任务处理函数类型。
type TaskHandlerFunc func()

// TaskRegistry 维护任务名称到处理函数的映射，供 LoadTasks 系列方法按名装配。
type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]TaskHandlerFunc
}

// NewTaskRegistry 创建任务注册表。
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{tasks: make(map[string]TaskHandlerFunc)}
}

// Register 注册一个任务处理函数。
func (tr *TaskRegistry) Register(name string, handler TaskHandlerFunc) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.tasks[name] = handler
}

func (tr *TaskRegistry) lookup(name string) (TaskHandlerFunc, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	handler, ok := tr.tasks[name]
	return handler, ok
}

// LoadTasksFromYAML 从 YAML 文件加载任务配置并注册到调度器。
func (tm *TaskManager) LoadTasksFromYAML(filePath string, registry *TaskRegistry) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %w", err)
	}
	return tm.LoadTasksFromYAMLBytes(data, registry)
}

// LoadTasksFromYAMLBytes 从 YAML 字节数据加载任务配置并注册到调度器。
// 使用严格解析（KnownFields），遇到未知字段时返回错误。
func (tm *TaskManager) LoadTasksFromYAMLBytes(data []byte, registry *TaskRegistry) error {
	var taskConfigs []TaskConfig

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&taskConfigs); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return tm.LoadTasks(taskConfigs, registry)
}

// LoadTasks 按配置列表加载任务：已禁用的任务跳过，未注册处理函数的任务仅记录警告，
// 单个任务添加失败（如 cron 表达式非法、名称重复）不会影响其余任务的加载。
func (tm *TaskManager) LoadTasks(taskConfigs []TaskConfig, registry *TaskRegistry) error {
	for _, cfg := range taskConfigs {
		if cfg.Disabled {
			tm.logger.Infof("Skipping disabled task: %s", cfg.Name)
			continue
		}

		handler, exists := registry.lookup(cfg.Name)
		if !exists {
			tm.logger.Warnf("Task %s has no registered handler", cfg.Name)
			continue
		}

		if err := tm.AddTask(cfg.Name, cfg.CronExpr, handler); err != nil {
			tm.logger.Errorf("Failed to load task %s: %v", cfg.Name, err)
			continue
		}
	}
	return nil
}
