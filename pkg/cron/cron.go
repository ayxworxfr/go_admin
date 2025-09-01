package cron

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"
)

// TaskManager 定时任务管理器
type TaskManager struct {
	scheduler *cron.Cron
	tasks     map[string]*managedJob
	mu        sync.RWMutex
	logger    Logger
}

// managedJob 自定义任务结构
type managedJob struct {
	entryID  cron.EntryID // cron 任务ID
	handler  func()       // 任务处理函数
	cronExpr string       // cron 表达式
	disabled bool         // 任务是否禁用
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// DefaultLogger 默认日志实现
type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, args ...any) {
	logger.Infof(context.Background(), msg, args...)
}

func (l *DefaultLogger) Warn(msg string, args ...any) {
	logger.Warnf(context.Background(), msg, args...)
}

func (l *DefaultLogger) Error(msg string, args ...any) {
	logger.Errorf(context.Background(), msg, args...)
}

// NewTaskManager 创建定时任务管理器
func NewTaskManager(logger Logger) *TaskManager {
	if logger == nil {
		logger = &DefaultLogger{}
	}

	// 移除秒级精度支持，使用标准cron
	return &TaskManager{
		scheduler: cron.New(),
		tasks:     make(map[string]*managedJob),
		logger:    logger,
	}
}

// Start 启动所有任务
func (tm *TaskManager) Start() {
	tm.scheduler.Start()
	tm.logger.Info("All scheduled tasks started")
}

// Stop 停止所有任务
func (tm *TaskManager) Stop() {
	ctx := tm.scheduler.Stop()
	<-ctx.Done() // 等待所有任务完成
	tm.logger.Info("All scheduled tasks stopped")
}

// AddTask 添加定时任务
func (tm *TaskManager) AddTask(name, cronExpr string, task func()) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tasks[name]; exists {
		return fmt.Errorf("task %s already exists", name)
	}

	// 包装任务以支持禁用功能
	wrappedTask := func() {
		tm.mu.RLock()
		defer tm.mu.RUnlock()

		if job, ok := tm.tasks[name]; ok && !job.disabled {
			task()
		}
	}

	// 添加任务到调度器
	entryID, err := tm.scheduler.AddFunc(cronExpr, wrappedTask)
	if err != nil {
		return fmt.Errorf("failed to add task %s: %w", name, err)
	}

	// 保存任务信息
	tm.tasks[name] = &managedJob{
		entryID:  entryID,
		handler:  wrappedTask,
		cronExpr: cronExpr,
		disabled: false,
	}

	tm.logger.Info("Task %s added, expression: %s", name, cronExpr)
	return nil
}

// RemoveTask 移除定时任务
func (tm *TaskManager) RemoveTask(name string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if job, exists := tm.tasks[name]; exists {
		tm.scheduler.Remove(job.entryID)
		delete(tm.tasks, name)
		tm.logger.Info("Task %s removed", name)
	} else {
		tm.logger.Warn("Attempt to remove non-existent task %s", name)
	}
}

// PauseTask 暂停定时任务
func (tm *TaskManager) PauseTask(name string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if job, exists := tm.tasks[name]; exists {
		job.disabled = true
		tm.logger.Info("Task %s paused", name)
	} else {
		tm.logger.Warn("Attempt to pause non-existent task %s", name)
	}
}

// ResumeTask 恢复定时任务
func (tm *TaskManager) ResumeTask(name string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if job, exists := tm.tasks[name]; exists {
		job.disabled = false
		tm.logger.Info("Task %s resumed", name)
	} else {
		tm.logger.Warn("Attempt to resume non-existent task %s", name)
	}
}

// GetTaskStatus 获取任务状态
func (tm *TaskManager) GetTaskStatus(name string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if job, exists := tm.tasks[name]; exists {
		if job.disabled {
			return "paused"
		}
		return "running"
	}
	return "not exist"
}

// ListTasks 获取所有任务信息
func (tm *TaskManager) ListTasks() []map[string]any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasksInfo []map[string]any
	for name, job := range tm.tasks {
		// 解析cron表达式获取下次运行时间
		spec, err := cron.ParseStandard(job.cronExpr)
		if err != nil {
			tm.logger.Error("Failed to parse cron expression for task %s: %v", name, err)
			continue
		}

		nextRun := spec.Next(time.Now()).Format(time.RFC3339)

		status := "running"
		if job.disabled {
			status = "paused"
		}

		tasksInfo = append(tasksInfo, map[string]any{
			"name":      name,
			"status":    status,
			"next_run":  nextRun,
			"cron_expr": job.cronExpr,
		})
	}
	return tasksInfo
}

// TaskConfig YAML配置中的单个任务结构
type TaskConfig struct {
	Name     string `yaml:"name"`
	CronExpr string `yaml:"cron_expr"`
	Disabled bool   `yaml:"disabled,omitempty"`
}

// TaskHandlerFunc 任务处理函数类型
type TaskHandlerFunc func()

// TaskRegistry 任务注册表，用于映射任务名称到处理函数
type TaskRegistry struct {
	tasks map[string]TaskHandlerFunc
}

// NewTaskRegistry 创建任务注册表
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]TaskHandlerFunc),
	}
}

// Register 注册任务处理函数
func (tr *TaskRegistry) Register(name string, handler TaskHandlerFunc) {
	tr.tasks[name] = handler
}

// LoadTasksFromYAML 从YAML文件加载任务
func (tm *TaskManager) LoadTasksFromYAML(filePath string, registry *TaskRegistry) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	return tm.LoadTasksFromYAMLBytes(data, registry)
}

// LoadTasksFromYAMLBytes 从YAML字节数据加载任务
func (tm *TaskManager) LoadTasksFromYAMLBytes(data []byte, registry *TaskRegistry) error {
	var taskConfigs []TaskConfig

	// 使用UnmarshalStrict进行严格解析，遇到未知字段时返回错误
	if err := yaml.UnmarshalStrict(data, &taskConfigs); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return tm.LoadTasks(taskConfigs, registry)
}

// LoadTasks 从任务配置列表加载任务
func (tm *TaskManager) LoadTasks(taskConfigs []TaskConfig, registry *TaskRegistry) error {
	for _, config := range taskConfigs {
		// 如果任务未启用，跳过加载
		if config.Disabled {
			tm.logger.Info("Skipping disabled task: %s", config.Name)
			continue
		}

		handler, exists := registry.tasks[config.Name]
		if !exists {
			tm.logger.Warn("Task %s has no registered handler", config.Name)
			continue
		}

		if err := tm.AddTask(config.Name, config.CronExpr, handler); err != nil {
			tm.logger.Error("Failed to load task %s: %v", config.Name, err)
			continue
		}
	}
	return nil
}
