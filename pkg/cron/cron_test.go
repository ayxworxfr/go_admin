package cron

import (
	"os"
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
	assert.Equal(tasks[0]["name"], "test_task")
}

func TestTaskManager_RemoveTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.RemoveTask("test_task")
	tasks := manager.ListTasks()
	assert.Len(tasks, 0)
}

func TestTaskManager_PauseTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("test_task")
	status := manager.GetTaskStatus("test_task")
	assert.Equal("paused", status)
}

func TestTaskManager_ResumeTask(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("test_task")
	manager.ResumeTask("test_task")
	status := manager.GetTaskStatus("test_task")
	assert.Equal("running", status)
}

func TestTaskManager_StartAndStop(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("test_task", "0 0 * * *", testTask)
	assert.NoError(err)

	manager.Start()
	time.Sleep(2 * time.Second)
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

	status := manager.GetTaskStatus("test_task")
	assert.Equal("running", status)
}

func TestTaskManager_MultipleOperations(t *testing.T) {
	assert := assert.New(t)

	manager := NewTaskManager(nil)
	err := manager.AddTask("task_a", "0 0 * * *", testTask)
	assert.NoError(err)
	err = manager.AddTask("task_b", "30 0 * * *", testTask)
	assert.NoError(err)

	manager.PauseTask("task_a")
	statusA := manager.GetTaskStatus("task_a")
	assert.Equal("paused", statusA)
	statusB := manager.GetTaskStatus("task_b")
	assert.Equal("running", statusB)

	manager.ResumeTask("task_a")
	statusA = manager.GetTaskStatus("task_a")
	assert.Equal("running", statusA)

	manager.RemoveTask("task_a")
	tasks := manager.ListTasks()
	assert.Len(tasks, 1)
	assert.Equal("task_b", tasks[0]["name"])

	manager.Start()
	time.Sleep(2 * time.Second)
	manager.Stop()
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

	// 加载无效YAML
	err := manager.LoadTasksFromYAMLBytes(invalidYAML, registry)

	// 验证错误信息（包含完整类型名称）
	assert.Error(err)
	assert.Contains(err.Error(), "failed to parse YAML")

	// 验证任务未添加
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

	// 加载任务
	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err) // 应该不报错，但任务不会添加

	// 验证任务未添加
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

	// 加载任务
	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err) // AddTask会报错，但LoadTasks会忽略并记录警告

	// 验证任务未添加
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

	// 加载任务
	err := manager.LoadTasksFromYAMLBytes(yamlData, registry)
	assert.NoError(err)

	// 验证只有启用的任务被添加
	tasks := manager.ListTasks()
	assert.Len(tasks, 1)
	assert.Equal("enabled_task", tasks[0]["name"])
}
