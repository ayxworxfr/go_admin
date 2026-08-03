// Package httpclient 提供带自动重试与指数退避的轻量 HTTP 客户端，
// 用于服务内部相互调用及请求第三方 HTTP 接口。
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
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

// statusError 表示服务端返回的状态码，仅供内部重试判定使用，不对外暴露。
type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("server returned status code %d", e.code)
}

// IsRetriableError 判断一次请求失败是否值得重试。
//
// 判定依据（均基于错误的结构化类型，而不是 err.Error() 的文本匹配，
// 避免因错误信息措辞变化或跨平台差异导致误判）：
//   - 5xx 状态码：服务端瞬时故障；
//   - *net.OpError 且 Op 为 dial/read/write：连接建立、读写阶段的传输层故障
//     （如连接被拒绝、连接被重置），通常是瞬时的；lookup（DNS 解析）类失败
//     通常是永久性配置问题，不视为可重试；
//   - 任意满足 net.Error 且 Timeout() 为 true 的错误：覆盖连接超时、
//     TLS 握手超时，以及 context 超时（context.DeadlineExceeded 本身实现了
//     net.Error，Timeout() 返回 true）。
func IsRetriableError(err error) bool {
	if err == nil {
		return false
	}

	var status *statusError
	if errors.As(err, &status) {
		return status.code >= http.StatusInternalServerError && status.code < 600
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		switch opErr.Op {
		case "dial", "read", "write":
			return true
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

// Client 是带重试能力的 HTTP 客户端。
//
// Client 在多个请求之间并发安全：headers 由内部读写锁保护；
// baseURL/httpClient/retries/backoff 只在构造期通过 Option 设置，
// 构造完成后只读，不会在请求过程中被修改。
type Client struct {
	baseURL    string
	httpClient *http.Client
	retries    int
	backoff    time.Duration

	headersMu sync.RWMutex
	headers   map[string]string
}

// Option 是配置 Client 的函数类型。
type Option func(*Client)

// WithTimeout 设置底层 http.Client 的超时时间（对每次尝试单独生效，不包含重试等待）。
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithRetries 设置失败重试次数（不含首次请求，例如设为 3 表示最多共尝试 4 次）。
func WithRetries(retries int) Option {
	return func(c *Client) {
		c.retries = retries
	}
}

// WithBackoff 设置重试的基础退避时间，第 n 次重试实际等待 backoff * 2^(n-1)。
func WithBackoff(backoff time.Duration) Option {
	return func(c *Client) {
		c.backoff = backoff
	}
}

// WithHeader 设置一个默认请求头，会附加到该 Client 发出的每个请求上。
func WithHeader(key, value string) Option {
	return func(c *Client) {
		c.headers[key] = value
	}
}

// WithHTTPClient 使用自定义的 http.Client（例如自定义 Transport、代理）。
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient 创建一个新的 HTTP 客户端。
//
// baseURL 允许为空字符串：此时每次请求需要把完整 URL 作为 path 参数传入
// （典型场景：调用方运行期才能确定 host，如面向本机的健康检查）。
func NewClient(baseURL string, opts ...Option) *Client {
	client := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retries: 3,
		backoff: 500 * time.Millisecond,
		headers: make(map[string]string),
	}

	for _, opt := range opts {
		opt(client)
	}

	if _, exists := client.headers["Content-Type"]; !exists {
		client.headers["Content-Type"] = "application/json"
	}

	return client
}

// SetHeader 设置一个 HTTP 头，并发安全，可在 Client 使用期间随时调用
// （例如刷新 Authorization token）。
func (c *Client) SetHeader(key, value string) {
	c.headersMu.Lock()
	defer c.headersMu.Unlock()
	c.headers[key] = value
}

// requestSpec 描述一次请求的不可变意图。
//
// 重试时会基于 spec 重新构建一个新的 *http.Request，而不是复用同一个实例：
// http.Request.Body 一旦被 Transport 读取（无论成功还是失败），都不能在重试时
// 重新发送，复用同一个请求对象会导致 POST/PUT 重试时发出空 body。
type requestSpec struct {
	method string
	url    string
	body   []byte // nil 表示无请求体
}

// buildRequestSpec 解析路径、拼接查询参数，并把 body 序列化为可重复读取的字节切片。
func (c *Client) buildRequestSpec(method, path string, params url.Values, body any) (requestSpec, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return requestSpec{}, fmt.Errorf("%w: %s", ErrInvalidURL, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	bodyBytes, err := serializeBody(body)
	if err != nil {
		return requestSpec{}, err
	}

	return requestSpec{method: method, url: u.String(), body: bodyBytes}, nil
}

// serializeBody 把请求体统一转换为可重复读取的字节切片：
// io.Reader 会被立即完整读取并缓存一次，以支持重试时重新发送；其余类型走 JSON 序列化。
func serializeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if reader, ok := body.(io.Reader); ok {
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrJSONMarshal, err)
	}
	return data, nil
}

// newHTTPRequest 基于 spec 构建一个全新的 *http.Request 并附加默认请求头。
func (c *Client) newHTTPRequest(ctx context.Context, spec requestSpec) (*http.Request, error) {
	var bodyReader io.Reader
	if spec.body != nil {
		bodyReader = bytes.NewReader(spec.body)
	}

	req, err := http.NewRequestWithContext(ctx, spec.method, spec.url, bodyReader)
	if err != nil {
		return nil, err
	}

	c.headersMu.RLock()
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	c.headersMu.RUnlock()

	return req, nil
}

// request 发起一次带重试的 HTTP 请求：5xx 状态码与可重试的网络错误都会触发重试，
// 每次重试都会重新构建请求并按指数退避等待。
func (c *Client) request(ctx context.Context, method, path string, params url.Values, body any) (*http.Response, error) {
	spec, err := c.buildRequestSpec(method, path, params, body)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			if err := c.waitBackoff(ctx, attempt); err != nil {
				return nil, err
			}
		}

		req, err := c.newHTTPRequest(ctx, spec)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if !IsRetriableError(err) {
				return nil, err
			}
			lastErr = err
			continue
		}

		if resp.StatusCode >= http.StatusInternalServerError {
			resp.Body.Close()
			lastErr = &statusError{code: resp.StatusCode}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retries+1, lastErr)
}

// waitBackoff 按指数退避等待下一次重试，等待期间会响应 ctx 取消/超时。
func (c *Client) waitBackoff(ctx context.Context, attempt int) error {
	backoff := c.backoff * time.Duration(1<<(attempt-1))
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Get 发送 GET 请求。
func (c *Client) Get(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	return c.request(ctx, http.MethodGet, path, params, nil)
}

// Post 发送 POST 请求。
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.request(ctx, http.MethodPost, path, nil, body)
}

// Put 发送 PUT 请求。
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.request(ctx, http.MethodPut, path, nil, body)
}

// Delete 发送 DELETE 请求。
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, http.MethodDelete, path, nil, nil)
}

// GetJSON 发送 GET 请求并将响应体解析到 response。
func (c *Client) GetJSON(ctx context.Context, path string, params url.Values, response any) error {
	resp, err := c.Get(ctx, path, params)
	return c.decodeJSON(resp, err, response)
}

// PostJSON 发送 POST 请求并将响应体解析到 response。
func (c *Client) PostJSON(ctx context.Context, path string, body, response any) error {
	resp, err := c.Post(ctx, path, body)
	return c.decodeJSON(resp, err, response)
}

// PutJSON 发送 PUT 请求并将响应体解析到 response。
func (c *Client) PutJSON(ctx context.Context, path string, body, response any) error {
	resp, err := c.Put(ctx, path, body)
	return c.decodeJSON(resp, err, response)
}

// DeleteJSON 发送 DELETE 请求并将响应体解析到 response。
func (c *Client) DeleteJSON(ctx context.Context, path string, response any) error {
	resp, err := c.Delete(ctx, path)
	return c.decodeJSON(resp, err, response)
}

// decodeJSON 是 GetJSON/PostJSON/PutJSON/DeleteJSON 共用的收尾逻辑。
func (c *Client) decodeJSON(resp *http.Response, err error, response any) error {
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.handleJSONResponse(resp, response)
}

// handleJSONResponse 检查状态码并解析JSON响应。
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
