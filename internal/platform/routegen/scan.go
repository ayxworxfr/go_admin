// Package routegen 在构建期扫描 handler 源码中的 // @route 注释，
// 生成编译进二进制的路由表，避免运行时 ParseFile（Docker 无源码 / -trimpath 下会失效）。
// 本包仅被 cmd/routegen 与测试使用，不进入服务运行时依赖图。
package routegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var routeTagRe = regexp.MustCompile(`(?i)@route\s+(\S+)\s+(\S+)`)

// Route 一条从源码提取的路由元数据。
type Route struct {
	PkgPath    string // 完整 import path，与 reflect.Type.PkgPath() 对齐
	TypeName   string // 接收者类型名（不含 *）
	MethodName string // Go 方法名
	HTTPMethod string // GET / POST / PUT / DELETE
	Path       string // URL 路径，如 /user/assign/roles
}

// ScanModule 从模块根目录扫描 internal/modules/*/handler 下的 @route。
func ScanModule(moduleRoot string) ([]Route, error) {
	moduleRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		return nil, err
	}
	modulePath, err := readModulePath(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return nil, err
	}

	pattern := filepath.Join(moduleRoot, "internal", "modules", "*", "handler")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("routegen: no handler dirs matching %s", pattern)
	}
	sort.Strings(dirs)

	var routes []Route
	seen := make(map[string]string) // key -> source location
	for _, dir := range dirs {
		rel, err := filepath.Rel(moduleRoot, dir)
		if err != nil {
			return nil, err
		}
		pkgPath := modulePath + "/" + filepath.ToSlash(rel)
		found, err := scanDir(dir, pkgPath)
		if err != nil {
			return nil, err
		}
		for _, r := range found {
			key := r.Key()
			loc := r.TypeName + "." + r.MethodName
			if prev, ok := seen[key]; ok {
				return nil, fmt.Errorf("routegen: duplicate route key %s (%s vs %s)", key, prev, loc)
			}
			seen[key] = loc
			routes = append(routes, r)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Key() < routes[j].Key()
	})
	return routes, nil
}

// Key 与 runtime 查找键一致：PkgPath.TypeName.MethodName
func (r Route) Key() string {
	return r.PkgPath + "." + r.TypeName + "." + r.MethodName
}

func readModulePath(goMod string) (string, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", fmt.Errorf("routegen: read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("routegen: module path not found in %s", goMod)
}

func scanDir(dir, pkgPath string) ([]Route, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var routes []Route
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fileRoutes, err := scanFile(filepath.Join(dir, name), pkgPath)
		if err != nil {
			return nil, err
		}
		routes = append(routes, fileRoutes...)
	}
	return routes, nil
}

func scanFile(path, pkgPath string) ([]Route, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("routegen: parse %s: %w", path, err)
	}

	var routes []Route
	for _, decl := range af.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 || funcDecl.Doc == nil {
			continue
		}
		typeName := astReceiverName(funcDecl.Recv.List[0].Type)
		if typeName == "" {
			continue
		}
		httpMethod, routePath, ok := ExtractRouteTag(funcDecl.Doc.Text())
		if !ok {
			continue
		}
		routes = append(routes, Route{
			PkgPath:    pkgPath,
			TypeName:   typeName,
			MethodName: funcDecl.Name.Name,
			HTTPMethod: httpMethod,
			Path:       routePath,
		})
	}
	return routes, nil
}

func astReceiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// ExtractRouteTag 从注释文本提取 @route Verb /path。
// 成功时 httpMethod 已规范化为大写。
func ExtractRouteTag(comment string) (httpMethod, path string, ok bool) {
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		m := routeTagRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		method := strings.ToUpper(m[1])
		switch method {
		case "GET", "POST", "PUT", "DELETE":
			return method, m[2], true
		}
	}
	return "", "", false
}
