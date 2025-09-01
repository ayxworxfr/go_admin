package dao

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	_ "github.com/ayxworxfr/go_admin/pkg/tests"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

var (
	once       sync.Once
	testEngine *xorm.Engine
)

func TestMain(m *testing.M) {
	// 在测试开始前初始化数据库连接
	setupTestDB()

	// 运行测试
	code := m.Run()

	// 在测试结束后关闭数据库连接
	func() {
		ClearTestDB()
		testEngine.Close()
	}()

	// 退出测试
	os.Exit(code)
}

// 模拟数据库连接
func setupTestDB() {
	once.Do(func() {
		testEngine = InitDB()
		if testEngine == nil {
			initError = fmt.Errorf("failed to initialize database")
			return
		}

		ClearTestDB()
	})
}

func ClearTestDB() {
	testEngine.Exec("DELETE FROM user")
	testEngine.Exec("DELETE FROM role")
	testEngine.Exec("DELETE FROM permission")
	testEngine.Exec("DELETE FROM user_role")
	testEngine.Exec("DELETE FROM role_permission")
}

// 测试事务一致性
func TestTransactionConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 初始化仓储
	processor := repository.NewXormProcessor(testEngine)
	userRepo := repository.NewRepository[models.User](processor)

	// 准备测试数据
	user := &models.User{
		Username: "tx-test",
		Email:    "test@example.com",
	}

	createFun := func(raiseError bool) error {
		// 执行事务操作
		_, err := userRepo.Transaction(context.Background(), func(txCtx context.Context) (any, error) {
			// 操作1：创建用户
			if err := userRepo.Create(txCtx, user); err != nil {
				return nil, err
			}
			// 获取Session
			// if session, ok := txCtx.Value(repository.TransactionKeyInstance).(*xorm.Session); ok && session != nil {
			// 	session.Insert(user)
			// }

			// 操作2：故意制造错误（模拟业务异常）
			if raiseError {
				return nil, errors.New("business error")
			}

			return nil, nil
		})
		return err
	}
	t.Run("Success", func(t *testing.T) {
		err := createFun(false)
		assert.NoError(t, err, "transaction should be committed")
		count, err := userRepo.QueryBuilder().Count(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count, "user should be created")
	})

	t.Run("Rollback", func(t *testing.T) {
		// 删除测试数据
		testEngine.Exec("DELETE FROM user")
		err := createFun(true)
		// 验证事务回滚
		assert.Error(t, err, "transaction should be rolled back")

		count, err := userRepo.QueryBuilder().Count(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count, "user should not be created")
	})
}

func TestUserRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	processor := repository.NewXormProcessor(testEngine)
	userRepo := repository.NewRepository[models.User](processor)
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		user := &models.User{
			Username: "testuser",
			Password: "password",
			Email:    "test@example.com",
		}

		err := userRepo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if user.ID == 0 {
			t.Fatal("ID not generated")
		}

		// 验证创建结果
		var created models.User
		has, err := testEngine.ID(user.ID).Get(&created)
		if err != nil {
			t.Fatalf("Failed to retrieve created user: %v", err)
		}
		if !has {
			t.Fatal("Created user not found")
		}
	})

	t.Run("Retrieve", func(t *testing.T) {
		// 准备测试数据
		user := &models.User{Username: "retrieveuser", Password: "pwd", Email: "retrieve@example.com"}
		_, err := testEngine.Insert(user)
		if err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		// 使用链式查询
		users, err := userRepo.QueryBuilder().
			Eq("username", "retrieveuser").
			Find(ctx)

		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("Expected 1 user, got %d", len(users))
		}
		if users[0].Username != "retrieveuser" {
			t.Errorf("Username mismatch, got %s", users[0].Username)
		}
	})

	t.Run("Update", func(t *testing.T) {
		// 删除测试数据
		testEngine.Exec("DELETE FROM user")

		// 准备测试数据
		user := &models.User{Username: "toupdate", Password: "old", Email: "old@example.com"}
		_, err := testEngine.Insert(user)
		if err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		updateData := &models.User{
			ID:       user.ID,
			Username: "updateduser",
			Email:    "updated@example.com",
		}
		err = userRepo.Update(ctx, updateData)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		var updatedUser models.User
		has, err := testEngine.ID(user.ID).Get(&updatedUser)
		if err != nil {
			t.Fatalf("Failed to retrieve updated user: %v", err)
		}
		if !has {
			t.Fatal("Updated user not found")
		}
		if updatedUser.Username != "updateduser" {
			t.Errorf("Username not updated, got %s", updatedUser.Username)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// 准备测试数据
		user := &models.User{Username: "todelete", Password: "pwd", Email: "delete@example.com"}
		_, err := testEngine.Insert(user)
		if err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		err = userRepo.Delete(ctx, &models.User{ID: user.ID})
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// 验证已删除
		var exists models.User
		has, err := testEngine.ID(user.ID).Get(&exists)
		if err != nil {
			t.Fatalf("Error checking existence: %v", err)
		}
		if has {
			t.Fatal("User still exists after deletion")
		}
	})

	t.Run("Count", func(t *testing.T) {
		// 清空测试数据
		testEngine.Exec("DELETE FROM user")

		// 创建测试数据
		for i := 0; i < 5; i++ {
			user := &models.User{
				Username: fmt.Sprintf("user%d", i),
				Password: "password",
				Email:    fmt.Sprintf("user%d@example.com", i),
			}
			_, err := testEngine.Insert(user)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}

		count, err := userRepo.QueryBuilder().Count(ctx)
		if err != nil {
			t.Fatalf("Count failed: %v", err)
		}
		if count != 5 {
			t.Fatalf("Expected count 5, got %d", count)
		}
	})
}
