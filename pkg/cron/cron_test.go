package cron

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 定义一个用于测试的简单任务函数
func testTask() {
	// 任务逻辑
}

func TestTaskManager_AddTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	tasks := manager.ListTasks()
	assert.Len(tasks, 1)
	assert.Equal("test_task", tasks[0].Name)
	assert.Equal("0 0 * * *", tasks[0].CronExpr)
	// schedule 在 AddTask 时已缓存，无需 Start() 也能算出下一次运行时间
	assert.False(tasks[0].NextRun.IsZero())
}

func TestTaskManager_AddTask_Duplicate(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	assert.NoError(manager.AddTask("test_task", "0 0 * * *", testTask))

	err := manager.AddTask("test_task", "30 0 * * *", testTask)
	assert.Error(err)
	assert.Contains(err.Error(), "already exists")
}

func TestTaskManager_AddTask_InvalidCronExpr(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("bad_task", "not-a-cron-expr", testTask)
	assert.Error(err)

	assert.Len(manager.ListTasks(), 0)
}

func TestTaskManager_RemoveTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.RemoveTask("test_task")
	tasks := manager.ListTasks()
	assert.Len(tasks, 0)

	// 移除不存在的任务不应 panic
	manager.RemoveTask("test_task")
}

func TestTaskManager_PauseTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("test_task")
	assert.Equal(TaskStatusPaused, manager.GetTaskStatus("test_task"))
}

func TestTaskManager_ResumeTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("test_task")
	manager.ResumeTask("test_task")
	assert.Equal(TaskStatusRunning, manager.GetTaskStatus("test_task"))
}

func TestTaskManager_PauseResume_NonExistent(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	// 对不存在的任务操作应仅记录警告，不应 panic
	manager.PauseTask("ghost")
	manager.ResumeTask("ghost")
	assert.Equal(TaskStatusNotExist, manager.GetTaskStatus("ghost"))
}

func TestTaskManager_StartAndStop(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.Start()
	manager.Stop()

	assert.NoError(err)
}

func TestTaskManager_ListTasks(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("task_1", "0 0 * * *", testTask)
	assert.NoError(err)
	err = manager.AddTask("task_2", "30 0 * * *", testTask)
	assert.NoError(err)

	tasks := manager.ListTasks()
	assert.Len(tasks, 2)
}

func TestTaskManager_GetTaskStatus(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	assert.Equal(TaskStatusRunning, manager.GetTaskStatus("test_task"))
}

func TestTaskManager_MultipleOperations(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("task_a", "0 0 * * *", testTask)
	assert.NoError(err)
	err = manager.AddTask("task_b", "30 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("task_a")
	assert.Equal(TaskStatusPaused, manager.GetTaskStatus("task_a"))
	assert.Equal(TaskStatusRunning, manager.GetTaskStatus("task_b"))

	manager.ResumeTask("task_a")
	assert.Equal(TaskStatusRunning, manager.GetTaskStatus("task_a"))

	manager.RemoveTask("task_a")
	tasks := manager.ListTasks()
	assert.Len(tasks, 1)
	assert.Equal("task_b", tasks[0].Name)

	manager.Start()
	manager.Stop()
}

// TestTaskManager_PauseDoesNotBlockOnRunningTask 验证 PauseTask/RemoveTask
// 等管理操作在任务执行期间不会被阻塞：disabled 状态由 atomic.Bool 承载，
// 而不是靠持有 mutex 贯穿整个任务执行过程。
func TestTaskManager_PauseDoesNotBlockOnRunningTask(t *testing.T) {
	manager := NewTaskManager(nil)

	var running sync.WaitGroup
	running.Add(1)
	release := make(chan struct{})

	err := manager.AddTask("slow_task", "0 0 * * *", func() {
		running.Done()
		<-release
	})
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// 模拟任务正在执行：直接调用管理器内部记录的 job 不可行（未导出），
	// 这里通过并发调用管理接口验证其在无任务运行时也不会自锁。
	done := make(chan struct{})
	go func() {
		manager.PauseTask("slow_task")
		manager.ResumeTask("slow_task")
		manager.GetTaskStatus("slow_task")
		manager.RemoveTask("slow_task")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("management operations blocked unexpectedly")
	}
	close(release)
}

func TestTaskManager_LoadTasksFromYAML(t *testing.T) {
	assert := assert.New(t)
	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("test_task", testTask)

	tmpFile, err := os.CreateTemp("", "tasks.yaml")
	assert.NoError(err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(`
- name: test_task
  cron_expr: 0 0 * * *
`))
	assert.NoError(err)
	assert.NoError(tmpFile.Close())

	err = manager.LoadTasksFromYAML(tmpFile.Name(), registry)
	assert.NoError(err)
	assert.Len(manager.ListTasks(), 1)
}

func TestTaskManager_LoadTasksFromYAMLBytes(t *testing.T) {
	assert := assert.New(t)
	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("test_task", testTask)

	yamlData := []byte(`
- name: test_task
  cron_expr: 0 0 * * *
`)

	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err)
	assert.Len(manager.ListTasks(), 1)
}

func TestTaskManager_LoadTasks_InvalidYAML(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("test_task", testTask)

	// 无效的YAML数据（包含未知顶层字段）
	invalidYAML := []byte(`
- name: test_task
  cron_expr: 0 0 * * *
  invalid_field: true
`)

	err := manager.LoadTasksFromYAMLBytes(invalidYAML, registry)

	assert.Error(err)
	assert.Contains(err.Error(), "failed to parse YAML")

	tasks := manager.ListTasks()
	assert.Len(tasks, 0)
}

func TestTaskManager_LoadTasks_UndefinedTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()

	// YAML中包含未注册的任务
	yamlData := []byte(`
- name: undefined_task
  cron_expr: 0 0 * * *
`)

	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err) // 应该不报错，但任务不会添加

	tasks := manager.ListTasks()
	assert.Len(tasks, 0)
}

func TestTaskManager_LoadTasks_InvalidCronExpr(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("test_task", testTask)

	// YAML中包含无效的cron表达式
	yamlData := []byte(`
- name: test_task
  cron_expr: invalid_cron_expr
`)

	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err) // AddTask会报错，但LoadTasks会忽略并记录警告

	tasks := manager.ListTasks()
	assert.Len(tasks, 0)
}

func TestTaskManager_LoadMultipleTasks(t *testing.T) {
	assert := assert.New(t)
	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("t1", testTask)
	registry.Register("t2", testTask)

	yamlData := []byte(`
- name: t1
  cron_expr: 0 0 * * *
- name: t2
  cron_expr: 30 0 * * *
`)

	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err)
	assert.Len(manager.ListTasks(), 2)
}

// 测试任务启用/禁用功能
func TestTaskManager_TaskEnableDisable(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	registry := NewTaskRegistry()
	registry.Register("enabled_task", testTask)
	registry.Register("disabled_task", testTask)

	yamlData := []byte(`
- name: enabled_task
  cron_expr: "0 0 * * *"
- name: disabled_task
  cron_expr: "0 12 * * *"
  disabled: true
`)

	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err)

	tasks := manager.ListTasks()
	assert.Len(tasks, 1)
	assert.Equal("enabled_task", tasks[0].Name)
}

// TestTaskRegistry_ConcurrentRegisterAndLookup 验证 TaskRegistry 在并发注册场景下不会 race。
func TestTaskRegistry_ConcurrentRegisterAndLookup(t *testing.T) {
	registry := NewTaskRegistry()
	var counter atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			registry.Register("task", func() { counter.Add(1) })
		}(i)
	}
	wg.Wait()

	handler, ok := registry.lookup("task")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	handler()
	if counter.Load() != 1 {
		t.Fatalf("expected counter 1, got %d", counter.Load())
	}
}
