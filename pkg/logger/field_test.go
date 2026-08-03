package logger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldConstructors_DoNotPanic(t *testing.T) {
	fields := []Field{
		String("s", "v"),
		Strings("ss", []string{"a", "b"}),
		Int("i", 1),
		Int64("i64", 2),
		Uint64("u64", 3),
		Uint64s("u64s", []uint64{4, 5}),
		Bool("b", true),
		Float64("f", 1.5),
		Duration("d", time.Second),
		Any("any", map[string]int{"k": 1}),
		Err(errors.New("boom")),
	}
	require.Len(t, toZap(fields), len(fields))
	assert.Nil(t, toZap(nil))
}

func TestWithContext_BindsFieldsToSubsequentLogs(t *testing.T) {
	// 只验证调用链路不 panic、字段能挂上 context；输出内容由集成环境观察。
	ctx := WithContext(context.Background(), String("request_id", "r-1"), Int("n", 7))
	Info(ctx, "with-fields")
	Warn(ctx, "with-fields-warn", Err(errors.New("x")))
}

func TestEnabled_RespectsSetLevel(t *testing.T) {
	ensureInit()
	SetLevel("warn")
	assert.False(t, Enabled("debug"))
	assert.False(t, Enabled("info"))
	assert.True(t, Enabled("warn"))
	assert.True(t, Enabled("error"))
	// 恢复到更宽松的级别，避免影响同包其他依赖全局级别的测试顺序
	SetLevel("debug")
	assert.True(t, Enabled("debug"))
}
