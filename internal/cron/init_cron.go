package cron

import (
	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/pkg/cron"
)

func InitCronTask() (*cron.TaskManager, error) {
	// 创建任务管理器
	manager := cron.NewTaskManager(nil)

	// 创建任务注册表并注册任务函数
	registry := cron.NewTaskRegistry()
	registry.Register("health_task", healthCheck)

	tasks := config.GetCronTasks()
	if tasks == nil {
		return manager, nil
	}
	if err := manager.LoadTasks(tasks, registry); err != nil {
		return nil, err
	}
	manager.Start()
	return manager, nil
}
