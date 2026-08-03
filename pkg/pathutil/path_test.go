package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectRoot_FromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROJECT_ROOT", dir)

	root, err := ProjectRoot()
	assert.NoError(t, err)

	wantAbs, err := filepath.Abs(dir)
	assert.NoError(t, err)
	assert.Equal(t, wantAbs, root)
}

func TestProjectRoot_WalksUpToGoMod(t *testing.T) {
	t.Setenv("PROJECT_ROOT", "")

	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fake"), 0o644))

	nested := filepath.Join(root, "a", "b", "c")
	assert.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := ProjectRoot()
	assert.NoError(t, err)

	wantAbs, err := filepath.Abs(root)
	assert.NoError(t, err)
	assert.Equal(t, wantAbs, got)
}

func TestProjectRoot_WalksUpToConfDir(t *testing.T) {
	t.Setenv("PROJECT_ROOT", "")

	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "conf"), 0o755))

	nested := filepath.Join(root, "x", "y")
	assert.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := ProjectRoot()
	assert.NoError(t, err)

	wantAbs, err := filepath.Abs(root)
	assert.NoError(t, err)
	assert.Equal(t, wantAbs, got)
}

func TestProjectRoot_NotFound(t *testing.T) {
	t.Setenv("PROJECT_ROOT", "")

	// 一个全新的临时目录，其所有祖先目录都不应含有 go.mod / conf。
	isolated := t.TempDir()
	t.Chdir(isolated)

	_, err := ProjectRoot()
	assert.True(t, errors.Is(err, ErrProjectRootNotFound))
}

func TestAbsPath_JoinsProjectRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROJECT_ROOT", dir)

	got := AbsPath(filepath.Join("conf", "config.yaml"))

	wantRoot, err := filepath.Abs(dir)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(wantRoot, "conf", "config.yaml"), got)
}

func TestAbsPath_FallsBackToOriginalPathWhenRootNotFound(t *testing.T) {
	t.Setenv("PROJECT_ROOT", "")

	isolated := t.TempDir()
	t.Chdir(isolated)

	got := AbsPath("conf/config.yaml")
	assert.Equal(t, "conf/config.yaml", got)
}
