// Package pathutil 提供项目根目录探测与项目相对路径解析能力，
// 主要用于加载 conf/ 下的配置文件：调用方给出相对于项目根目录的路径，
// 无需关心进程当前工作目录是仓库根、cmd/ 还是测试临时目录。
package pathutil

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrProjectRootNotFound 表示从当前工作目录逐级向上查找到文件系统根目录，
// 仍未发现任何 rootMarkers 标志文件/目录。
var ErrProjectRootNotFound = errors.New("pathutil: project root not found")

// rootMarkers 是判定项目根目录的标志文件/目录，命中任意一个即认为到达根目录。
var rootMarkers = []string{"go.mod", "conf"}

// ProjectRoot 返回项目根目录的绝对路径。
//
// 优先读取 PROJECT_ROOT 环境变量（例如容器化部署时显式指定）；
// 未设置时从当前工作目录逐级向上查找，直到发现 go.mod 或 conf 目录为止。
func ProjectRoot() (string, error) {
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return filepath.Abs(root)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRootFrom(wd)
}

// findRootFrom 从 dir 开始逐级向上查找，直到命中 rootMarkers 或到达文件系统根目录。
func findRootFrom(dir string) (string, error) {
	for {
		for _, marker := range rootMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrProjectRootNotFound
		}
		dir = parent
	}
}

// AbsPath 将相对于项目根目录的路径拼接为绝对路径。
//
// 当无法确定项目根目录时（如运行环境限制导致查找失败），退化为原样返回 path，
// 交由调用方按当前工作目录解释，而不是直接 panic 中断启动流程。
func AbsPath(path string) string {
	root, err := ProjectRoot()
	if err != nil {
		return path
	}
	return filepath.Join(root, path)
}
