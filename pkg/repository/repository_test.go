package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"xorm.io/xorm"
)

// item 是仓储关键路径测试用模型，刻意不依赖业务模块。
type item struct {
	ID    int64  `xorm:"pk autoincr 'id'"`
	Name  string `xorm:"varchar(64) notnull unique 'name'"`
	Score int    `xorm:"int 'score'"`
}

func (item) TableName() string { return "repo_item" }

func newTestRepo(t *testing.T) *Repository[item] {
	t.Helper()
	// 每个用例独立内存库；MaxOpenConns=1 避免多连接看不到 :memory: 表结构
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	engine, err := xorm.NewEngine("sqlite", dsn)
	require.NoError(t, err)
	engine.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = engine.Close() })

	require.NoError(t, engine.Sync2(new(item)))
	return NewRepository[item](New(engine))
}

func TestCRUDAndFindSemantics(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	row := &item{Name: "alice", Score: 10}
	require.NoError(t, repo.Create(ctx, row))
	require.NotZero(t, row.ID)

	got, err := repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Name)

	_, err = repo.FindByID(ctx, int64(99999))
	assert.ErrorIs(t, err, ErrNotFound)

	got.Score = 20
	require.NoError(t, repo.Update(ctx, got))

	byName, err := repo.Find(ctx, &item{Name: "alice"})
	require.NoError(t, err)
	assert.Equal(t, 20, byName.Score)

	require.NoError(t, repo.DeleteByID(ctx, row.ID))
	_, err = repo.FindByID(ctx, row.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFindPageSeparatesListAndCount(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.BatchCreate(ctx, []item{
		{Name: "a", Score: 1},
		{Name: "b", Score: 2},
		{Name: "c", Score: 3},
		{Name: "d", Score: 4},
	}))

	rows, total, err := repo.FindPage(ctx, &item{}, 2, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, int64(4), total, "Count 不应被 Limit 污染")
}

func TestQueryBuilderCountDoesNotRunFind(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.BatchCreate(ctx, []item{
		{Name: "x1", Score: 1},
		{Name: "x2", Score: 2},
		{Name: "y1", Score: 3},
	}))

	count, err := repo.QueryBuilder().Like("name", "x").Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	rows, err := repo.QueryBuilder().Like("name", "x").OrderBy("id ASC").Find(ctx)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestTransactionCommit(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.Transaction(ctx, func(txCtx context.Context) error {
		return repo.Create(txCtx, &item{Name: "commit", Score: 1})
	})
	require.NoError(t, err)

	count, err := repo.QueryBuilder().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestTransactionRollbackOnError(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, &item{Name: "rollback", Score: 1}); err != nil {
			return err
		}
		return errors.New("business error")
	})
	require.Error(t, err)

	count, err := repo.QueryBuilder().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "error 应触发回滚，库中不应有数据")
}

func TestTransactionRollbackOnPanic(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	assert.Panics(t, func() {
		_ = repo.Transaction(ctx, func(txCtx context.Context) error {
			if err := repo.Create(txCtx, &item{Name: "panic", Score: 1}); err != nil {
				return err
			}
			panic("boom")
		})
	})

	count, err := repo.QueryBuilder().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "panic 应回滚事务")
}

func TestNestedTransactionReusesOuter(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// 内层 Transaction 复用外层 session：内层失败时外层一并回滚
	err := repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, &item{Name: "outer", Score: 1}); err != nil {
			return err
		}
		return repo.Transaction(txCtx, func(innerCtx context.Context) error {
			if err := repo.Create(innerCtx, &item{Name: "inner", Score: 2}); err != nil {
				return err
			}
			return errors.New("inner fail")
		})
	})
	require.Error(t, err)

	count, err := repo.QueryBuilder().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "嵌套事务失败应回滚全部写入")

	// 内外层都成功时一并提交
	err = repo.Transaction(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, &item{Name: "outer-ok", Score: 1}); err != nil {
			return err
		}
		return repo.Transaction(txCtx, func(innerCtx context.Context) error {
			return repo.Create(innerCtx, &item{Name: "inner-ok", Score: 2})
		})
	})
	require.NoError(t, err)

	count, err = repo.QueryBuilder().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
