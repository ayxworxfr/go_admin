package handler

import (
	"errors"

	"github.com/ayxworxfr/go_admin/pkg/context"
)

func HelloHandler(c *context.Context) *context.Response {
	// c.String(200, "Hello, World!")
	return context.Success("Hello, World!")
}

func HelloJSONHandler(c *context.Context) {
	c.JSON(200, map[string]string{
		"message": "Hello, World!",
	})
}

func HelloWithErrorHandler(c *context.Context) {
	// 模拟一个错误
	err := errors.New("simulated error")
	if err != nil {
		c.JSON(500, map[string]string{
			"error": "An error occurred: " + err.Error(),
		})
		return
	}

	c.String(200, "Hello, World with Error!")
}
