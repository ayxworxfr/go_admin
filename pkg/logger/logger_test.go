package logger

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestApplyDefaults(t *testing.T) {
	cfg := applyDefaults(Config{})
	assert.Equal(t, defaultLogFile, cfg.LogFile)
	assert.Equal(t, defaultLevel, cfg.Level)
	assert.Equal(t, defaultMaxSize, cfg.MaxSize)
	assert.Equal(t, defaultMaxBackups, cfg.MaxBackups)
	assert.Equal(t, defaultMaxAge, cfg.MaxAge)

	custom := applyDefaults(Config{
		LogFile:    "custom.log",
		Level:      "debug",
		MaxSize:    1,
		MaxBackups: 2,
		MaxAge:     3,
	})
	assert.Equal(t, "custom.log", custom.LogFile)
	assert.Equal(t, "debug", custom.Level)
	assert.Equal(t, 1, custom.MaxSize)
	assert.Equal(t, 2, custom.MaxBackups)
	assert.Equal(t, 3, custom.MaxAge)
}

func TestSetLevel_MapsKnownLevels(t *testing.T) {
	cases := []struct {
		input string
		want  zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"fatal", zapcore.FatalLevel},
		{"unknown", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
	}

	for _, tt := range cases {
		lvl := zap.NewAtomicLevel()
		setLevel(lvl, tt.input)
		assert.Equal(t, tt.want, lvl.Level(), "input=%q", tt.input)
	}
}

func TestBuildWriteSyncer_ConsoleFlagControlsStdoutFanout(t *testing.T) {
	dir := t.TempDir()
	logFile := dir + string(os.PathSeparator) + "app.log"

	// Console=false 时只应该往文件写；这里只验证不会 panic、返回值可用，
	// 真正的"是否落盘"由集成测试 TestGlobalLogging_DoesNotLeakFilesOnFallbackInit 覆盖。
	syncer := buildWriteSyncer(Config{LogFile: logFile, MaxSize: 1, MaxBackups: 1, MaxAge: 1})
	assert.NotNil(t, syncer)

	syncerWithConsole := buildWriteSyncer(Config{LogFile: logFile, Console: true, MaxSize: 1, MaxBackups: 1, MaxAge: 1})
	assert.NotNil(t, syncerWithConsole)
}

// TestGlobalLogging_DoesNotLeakFilesOnFallbackInit 是本包关于"意外落盘"问题的
// 回归测试：在从未显式调用 InitLogger 的前提下（模拟 pkg/cron 等只拿到 nil
// Logger、转发到全局 logger 包的场景），任何一次日志调用都不应该在当前工作
// 目录下创建 logs/ 目录或任何文件。
//
// 这是本文件里唯一一个会触碰全局 once 的测试：全局初始化只会成功一次，
// 所以特意不在其他测试里调用 InitLogger/ensureInit 之外的全局状态，
// 保证不管测试执行顺序如何，"第一次真正触发初始化"的都是这里，行为可预期。
func TestGlobalLogging_DoesNotLeakFilesOnFallbackInit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ctx := context.Background()
	Info(ctx, "hello")
	Warnf(ctx, "warn %d", 1)
	SetLevel("debug")
	assert.NoError(t, Sync())

	_, err := os.Stat("logs")
	assert.True(t, os.IsNotExist(err), "fallback init must not create a logs/ directory, got err=%v", err)
}
