package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/ayxworxfr/go_admin/internal/platform/config"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	_ "github.com/ayxworxfr/go_admin/pkg/tests"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

// testModel 是通用仓储引擎的集成测试用模型，字段与 user 模块的 User 兼容，
// 但刻意不依赖 modules/user——db 包测试的是"仓储引擎能不能正确工作"这件事，
// 不应该关心某个具体业务模块的表结构。
type testModel struct {
	ID       uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Username string `xorm:"varchar(50) notnull unique 'username'" json:"username"`
	Password string `xorm:"varchar(100) 'password'" json:"password"`
	Email    string `xorm:"varchar(100) notnull unique 'email'" json:"email"`
}

func (testModel) TableName() string { return "user" }

var (
	once       sync.Once
	testEngine *xorm.Engine
)

func TestMain(m *testing.M) {
	setupTestDB()
	code := m.Run()
	clearTestDB()
	if testEngine != nil {
		testEngine.Close()
	}
	os.Exit(code)
}

func setupTestDB() {
	once.Do(func() {
		var err error
		testEngine, err = NewEngine(config.Get())
		if err != nil {
			panic(fmt.Sprintf("failed to initialize test database: %v", err))
		}
		clearTestDB()
	})
}

func clearTestDB() {
	testEngine.Exec("DELETE FROM user")
}

// TestTransactionConsistency 验证 pkg/repository 的事务在业务返回错误时能正确回滚
func TestTransactionConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := repository.New(testEngine)
	userRepo := repository.NewRepository[testModel](db)

	user := &testModel{Username: "tx-test", Email: "test@example.com"}

	createFun := func(raiseError bool) error {
		return userRepo.Transaction(context.Background(), func(txCtx context.Context) error {
			if err := userRepo.Create(txCtx, user); err != nil {
				return err
			}
			if raiseError {
				return errors.New("business error")
			}
			return nil
		})
	}

	t.Run("Success", func(t *testing.T) {
		err := createFun(false)
		assert.NoError(t, err, "transaction should be committed")
		count, err := userRepo.QueryBuilder().Count(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count, "user should be created")
	})

	t.Run("Rollback", func(t *testing.T) {
		clearTestDB()
		err := createFun(true)
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

	db := repository.New(testEngine)
	userRepo := repository.NewRepository[testModel](db)
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		clearTestDB()
		user := &testModel{Username: "testuser", Password: "password", Email: "test@example.com"}

		err := userRepo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if user.ID == 0 {
			t.Fatal("ID not generated")
		}

		var created testModel
		has, err := testEngine.ID(user.ID).Get(&created)
		if err != nil {
			t.Fatalf("Failed to retrieve created user: %v", err)
		}
		if !has {
			t.Fatal("Created user not found")
		}
	})

	t.Run("Retrieve", func(t *testing.T) {
		clearTestDB()
		user := &testModel{Username: "retrieveuser", Password: "pwd", Email: "retrieve@example.com"}
		if _, err := testEngine.Insert(user); err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		users, err := userRepo.QueryBuilder().Eq("username", "retrieveuser").Find(ctx)
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
		clearTestDB()
		user := &testModel{Username: "toupdate", Password: "old", Email: "old@example.com"}
		if _, err := testEngine.Insert(user); err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		updateData := &testModel{ID: user.ID, Username: "updateduser", Email: "updated@example.com"}
		if err := userRepo.Update(ctx, updateData); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		var updatedUser testModel
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
		clearTestDB()
		user := &testModel{Username: "todelete", Password: "pwd", Email: "delete@example.com"}
		if _, err := testEngine.Insert(user); err != nil {
			t.Fatalf("Failed to setup test data: %v", err)
		}

		if err := userRepo.Delete(ctx, &testModel{ID: user.ID}); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		var exists testModel
		has, err := testEngine.ID(user.ID).Get(&exists)
		if err != nil {
			t.Fatalf("Error checking existence: %v", err)
		}
		if has {
			t.Fatal("User still exists after deletion")
		}
	})

	t.Run("Count", func(t *testing.T) {
		clearTestDB()
		for i := 0; i < 5; i++ {
			user := &testModel{
				Username: fmt.Sprintf("user%d", i),
				Password: "password",
				Email:    fmt.Sprintf("user%d@example.com", i),
			}
			if _, err := testEngine.Insert(user); err != nil {
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

	t.Run("OrAndGroup", func(t *testing.T) {
		clearTestDB()
		fixtures := []testModel{
			{Username: "alice", Password: "p", Email: "alice@example.com"},
			{Username: "bob", Password: "p", Email: "bob@example.com"},
			{Username: "carol", Password: "p", Email: "carol@example.com"},
		}
		for i := range fixtures {
			if _, err := testEngine.Insert(&fixtures[i]); err != nil {
				t.Fatalf("Failed to insert fixture: %v", err)
			}
		}

		// username = alice OR username = bob
		rows, err := userRepo.QueryBuilder().
			Eq("username", "alice").
			Or().Eq("username", "bob").
			Find(ctx)
		assert.NoError(t, err)
		assert.Len(t, rows, 2)

		// email LIKE %@example.com AND (username = alice OR username = carol)
		rows, err = userRepo.QueryBuilder().
			Like("email", "@example.com").
			AndGroup(func(g *repository.QueryBuilder[testModel]) {
				g.Eq("username", "alice").Or().Eq("username", "carol")
			}).
			Find(ctx)
		assert.NoError(t, err)
		assert.Len(t, rows, 2)
	})
}
