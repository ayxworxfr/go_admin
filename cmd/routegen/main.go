// Command routegen 扫描 internal/modules/*/handler 的 // @route 注释，
// 生成 internal/platform/router/routes_gen.go，把路由元数据编译进二进制。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ayxworxfr/go_admin/internal/platform/routegen"
)

func main() {
	root := flag.String("root", ".", "Go module root (directory containing go.mod)")
	out := flag.String("out", "internal/platform/router/routes_gen.go", "output file relative to module root")
	flag.Parse()

	moduleRoot, err := filepath.Abs(*root)
	if err != nil {
		fail(err)
	}

	routes, err := routegen.ScanModule(moduleRoot)
	if err != nil {
		fail(err)
	}
	src, err := routegen.Render(routes)
	if err != nil {
		fail(err)
	}

	outPath := *out
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(moduleRoot, outPath)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(outPath, src, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("routegen: wrote %d routes → %s\n", len(routes), outPath)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "routegen: %v\n", err)
	os.Exit(1)
}
