package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	"xorm.io/xorm/contexts"
)

// XormLogger 把 xorm 的执行钩子接到项目日志与调用链追踪上：慢查询单独告警，
// 所有执行信息都作为 span event 记录，方便在 OpenTelemetry 里按请求串联 SQL。
type XormLogger struct {
	showSQL       bool
	slowThreshold time.Duration
}

// NewXormLogger 创建 xorm 日志钩子
func NewXormLogger(showSQL bool) *XormLogger {
	return &XormLogger{
		showSQL:       showSQL,
		slowThreshold: 100 * time.Millisecond, // 慢查询阈值
	}
}

func (l *XormLogger) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	return c.Ctx, nil
}

func (l *XormLogger) AfterProcess(c *contexts.ContextHook) error {
	if !l.showSQL {
		return nil
	}

	if c.ExecuteTime > l.slowThreshold {
		logger.Warnf(c.Ctx, "Slow SQL: %s, Args: %v, ExecTime: %v", c.SQL, c.Args, c.ExecuteTime)
	} else if c.ExecuteTime > 0 {
		logger.Infof(c.Ctx, "SQL: %s, Args: %v, ExecTime: %v", c.SQL, c.Args, c.ExecuteTime)
	}

	info := map[string]any{
		"sql":      c.SQL,
		"duration": fmt.Sprintf("%v", c.ExecuteTime),
	}
	if len(c.Args) > 0 {
		info["args"] = c.Args
	}
	repository.RecordDbEvent(c.Ctx, info)
	return nil
}
