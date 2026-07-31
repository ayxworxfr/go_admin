package router

// 变更 internal/modules/*/handler 上的 // @route 后执行：
//
//	go generate ./internal/platform/router/...
//
// 或 make generate。Docker 构建也会在 go build 前重新生成。
//
//go:generate go run ../../../cmd/routegen -root ../../..
