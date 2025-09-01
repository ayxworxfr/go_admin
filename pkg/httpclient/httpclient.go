package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 错误类型定义
var (
	ErrInvalidURL        = errors.New("invalid URL")
	ErrJSONMarshal       = errors.New("JSON marshal failed")
	ErrJSONUnmarshal     = errors.New("JSON unmarshal failed")
	ErrStatusNotOK       = errors.New("HTTP status code is not successful")
	ErrEmptyResponseBody = errors.New("response body is empty")
)

func IsRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否是我们自定义的HTTP 500错误
	if strings.Contains(err.Error(), "server returned status code 5") {
		return true
	}

	// 检查常见的可重试网络错误
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "TLS handshake timeout") {
		return true
	}

	return false
}

// Client 是 HTTP 客户端的主结构体
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string
	Retries    int
	Backoff    time.Duration
}

// Option 是配置客户端的函数类型
type Option func(*Client)

// WithTimeout 设置HTTP客户端超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.HTTPClient.Timeout = timeout
	}
}

// WithRetries 设置重试次数
func WithRetries(retries int) Option {
	return func(c *Client) {
		c.Retries = retries
	}
}

// WithBackoff 设置重试退避时间
func WithBackoff(backoff time.Duration) Option {
	return func(c *Client) {
		c.Backoff = backoff
	}
}

// WithHeader 设置默认请求头
func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.Headers[key] = value
	}
}

// WithHTTPClient 使用自定义的HTTP客户端
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

// NewClient 创建一个新的 HTTP 客户端
func NewClient(baseURL string, opts ...Option) *Client {
	client := &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Headers: make(map[string]string),
		Retries: 3,                      // 默认重试3次
		Backoff: 500 * time.Millisecond, // 默认退避500毫秒
	}

	// 应用选项
	for _, opt := range opts {
		opt(client)
	}

	// 设置默认Content-Type
	if _, exists := client.Headers["Content-Type"]; !exists {
		client.Headers["Content-Type"] = "application/json"
	}

	return client
}

// SetHeader 设置一个 HTTP 头
func (c *Client) SetHeader(key, value string) {
	c.Headers[key] = value
}

// request 是发送 HTTP 请求的通用方法
func (c *Client) request(ctx context.Context, method, path string, params url.Values, body any) (*http.Response, error) {
	// 构建URL
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidURL, err)
	}

	// 添加查询参数
	if params != nil {
		u.RawQuery = params.Encode()
	}

	// 处理请求体
	var bodyReader io.Reader
	if body != nil {
		// 特殊处理：如果是io.Reader类型，直接使用
		if reader, ok := body.(io.Reader); ok {
			bodyReader = reader
		} else {
			// 否则进行JSON序列化
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("%w: %s", ErrJSONMarshal, err)
			}
			bodyReader = bytes.NewBuffer(jsonBody)
		}
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	// 添加自定义头
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	// 执行请求（带改进的重试逻辑）
	var resp *http.Response
	for i := 0; i <= c.Retries; i++ {
		resp, err = c.HTTPClient.Do(req)

		// 处理网络错误（如连接超时）
		if err != nil {
			if !IsRetriableError(err) {
				return nil, err
			}
			// 可重试的网络错误，继续循环
		} else {
			// 检查HTTP状态码是否为可重试的服务器错误
			if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				// 关闭响应体以便重试
				resp.Body.Close()
				// 标记为需要重试
				err = fmt.Errorf("server returned status code %d", resp.StatusCode)
				if !IsRetriableError(err) {
					return nil, err
				}
			} else {
				// 非500状态码，认为请求成功（或非重试错误）
				break
			}
		}

		// 重试前等待（使用指数退避）
		if i < c.Retries {
			backoffTime := c.Backoff * time.Duration(1<<i)
			select {
			case <-time.After(backoffTime):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return resp, err
}

// Get 发送 GET 请求
func (c *Client) Get(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	return c.request(ctx, http.MethodGet, path, params, nil)
}

// Post 发送 POST 请求
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.request(ctx, http.MethodPost, path, nil, body)
}

// Put 发送 PUT 请求
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.request(ctx, http.MethodPut, path, nil, body)
}

// Delete 发送 DELETE 请求
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, http.MethodDelete, path, nil, nil)
}

// GetJSON 发送GET请求并解析JSON响应
func (c *Client) GetJSON(ctx context.Context, path string, params url.Values, response any) error {
	resp, err := c.Get(ctx, path, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleJSONResponse(resp, response)
}

// PostJSON 发送POST请求并解析JSON响应
func (c *Client) PostJSON(ctx context.Context, path string, body, response any) error {
	resp, err := c.Post(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleJSONResponse(resp, response)
}

// PutJSON 发送PUT请求并解析JSON响应
func (c *Client) PutJSON(ctx context.Context, path string, body, response any) error {
	resp, err := c.Put(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleJSONResponse(resp, response)
}

// DeleteJSON 发送DELETE请求并解析JSON响应
func (c *Client) DeleteJSON(ctx context.Context, path string, response any) error {
	resp, err := c.Delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleJSONResponse(resp, response)
}

// handleJSONResponse 处理JSON响应
func (c *Client) handleJSONResponse(resp *http.Response, response any) error {
	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %d %s, body: %s", ErrStatusNotOK, resp.StatusCode, http.StatusText(resp.StatusCode), string(bodyBytes))
	}

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 如果响应体为空且不需要解析到结构体，则直接返回
	if len(bodyBytes) == 0 {
		if response == nil {
			return nil
		}
		return ErrEmptyResponseBody
	}

	// 解析JSON
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("%w: %s, body: %s", ErrJSONUnmarshal, err, string(bodyBytes))
	}

	return nil
}
