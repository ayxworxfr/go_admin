package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/httpclient"
)

func TestNewClient(t *testing.T) {
	client := httpclient.NewClient("https://api.example.com")
	if client.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL mismatch, got: %s, want: %s", client.BaseURL, "https://api.example.com")
	}
	if client.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("Timeout mismatch, got: %v, want: %v", client.HTTPClient.Timeout, 30*time.Second)
	}
	if client.Retries != 3 {
		t.Errorf("Retries mismatch, got: %d, want: %d", client.Retries, 3)
	}
	if client.Backoff != 500*time.Millisecond {
		t.Errorf("Backoff mismatch, got: %v, want: %v", client.Backoff, 500*time.Millisecond)
	}
}

func TestClient_WithOptions(t *testing.T) {
	client := httpclient.NewClient(
		"https://api.example.com",
		httpclient.WithTimeout(15*time.Second),
		httpclient.WithRetries(5),
		httpclient.WithBackoff(200*time.Millisecond),
		httpclient.WithHeader("X-App-ID", "test-app"),
	)

	if client.HTTPClient.Timeout != 15*time.Second {
		t.Errorf("Timeout mismatch, got: %v, want: %v", client.HTTPClient.Timeout, 15*time.Second)
	}
	if client.Retries != 5 {
		t.Errorf("Retries mismatch, got: %d, want: %d", client.Retries, 5)
	}
	if client.Backoff != 200*time.Millisecond {
		t.Errorf("Backoff mismatch, got: %v, want: %v", client.Backoff, 200*time.Millisecond)
	}
	if client.Headers["X-App-ID"] != "test-app" {
		t.Errorf("Header mismatch, got: %s, want: %s", client.Headers["X-App-ID"], "test-app")
	}
}

func TestClient_SetHeader(t *testing.T) {
	client := httpclient.NewClient("https://api.example.com")
	client.SetHeader("Authorization", "Bearer token123")

	if client.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Header not set correctly, got: %s, want: %s", client.Headers["Authorization"], "Bearer token123")
	}
}

func TestClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method mismatch, got: %s, want: %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/test" {
			t.Errorf("Path mismatch, got: %s, want: %s", r.URL.Path, "/test")
		}
		if r.URL.Query().Get("param") != "value" {
			t.Errorf("Query param mismatch, got: %s, want: %s", r.URL.Query().Get("param"), "value")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type mismatch, got: %s, want: %s", r.Header.Get("Content-Type"), "application/json")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	params := url.Values{}
	params.Add("param", "value")

	resp, err := client.Get(context.Background(), "/test", params)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code mismatch, got: %d, want: %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"message": "success"}` {
		t.Errorf("Response body mismatch, got: %s, want: %s", string(body), `{"message": "success"}`)
	}
}

func TestClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method mismatch, got: %s, want: %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/test" {
			t.Errorf("Path mismatch, got: %s, want: %s", r.URL.Path, "/test")
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"key":"value"}` {
			t.Errorf("Request body mismatch, got: %s, want: %s", string(body), `{"key":"value"}`)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "created"}`))
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	body := map[string]string{"key": "value"}

	resp, err := client.Post(context.Background(), "/test", body)
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Status code mismatch, got: %d, want: %d", resp.StatusCode, http.StatusCreated)
	}
}

func TestClient_GetJSON(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  error
		expectedResult any
	}{
		{
			name:         "success",
			statusCode:   http.StatusOK,
			responseBody: `{"message": "success", "data": {"id": 1}}`,
			expectedResult: map[string]any{
				"message": "success",
				"data": map[string]any{
					"id": 1,
				},
			},
		},
		{
			name:          "error_status",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"error": "not found"}`,
			expectedError: httpclient.ErrStatusNotOK,
		},
		{
			name:          "invalid_json",
			statusCode:    http.StatusOK,
			responseBody:  `invalid json`,
			expectedError: httpclient.ErrJSONUnmarshal,
		},
		{
			name:          "empty_body",
			statusCode:    http.StatusOK,
			responseBody:  "",
			expectedError: httpclient.ErrEmptyResponseBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer ts.Close()

			client := httpclient.NewClient(ts.URL)
			var result any

			err := client.GetJSON(context.Background(), "/test", nil, &result)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error: %v, got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Fatalf("expected error: %v, got: %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetJSON() error: %v", err)
			}

			// 统一使用map进行比较
			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("result is not a map, got: %T", result)
			}

			expectedMap, ok := tt.expectedResult.(map[string]any)
			if !ok && tt.expectedResult != nil {
				t.Fatalf("expectedResult is not a map, got: %T", tt.expectedResult)
			}

			if resultMap["message"] != expectedMap["message"] {
				t.Errorf("result mismatch, got: %v, want: %v", resultMap, expectedMap)
			}
		})
	}
}

func TestClient_Retry(t *testing.T) {
	var callCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		t.Logf("Request count: %d", callCount) // 增加日志输出
		if callCount <= 2 {
			// 前两次请求返回500错误
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`)) // 增加错误响应体
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()

	client := httpclient.NewClient(
		ts.URL,
		httpclient.WithRetries(3), // 重试3次
		httpclient.WithBackoff(100*time.Millisecond), // 增加退避时间
	)

	var result struct {
		Message string `json:"message"`
	}

	err := client.GetJSON(context.Background(), "/test", nil, &result)
	if err != nil {
		// 输出详细错误信息
		respBody, _ := json.Marshal(result)
		t.Fatalf("GetJSON() error: %v, response: %s", err, respBody)
	}

	if callCount != 3 {
		t.Errorf("unexpected call count, got: %d, want: %d", callCount, 3)
	}

	if result.Message != "success" {
		t.Errorf("unexpected result, got: %s, want: %s", result.Message, "success")
	}
}

func TestClient_Request_InvalidURL(t *testing.T) {
	client := httpclient.NewClient("invalid-url")
	resp, err := client.Get(context.Background(), "/test", nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// 检查是否为URL解析错误
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("expected url.Error, got: %T %v", err, err)
	}

	// 检查错误信息是否包含"invalid URL"
	if !strings.Contains(err.Error(), "invalid-url") {
		t.Fatalf("expected error containing 'invalid URL', got: %v", err)
	}

	if resp != nil {
		t.Fatalf("expected nil response, got: %v", resp)
	}
}

func TestClient_Request_ContextCanceled(t *testing.T) {
	client := httpclient.NewClient("https://api.example.com")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消上下文

	resp, err := client.Get(ctx, "/test", nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got: %v", err)
	}

	if resp != nil {
		t.Fatalf("expected nil response, got: %v", resp)
	}
}

func TestClient_Request_WithReaderBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != "test-body" {
			t.Errorf("Request body mismatch, got: %s, want: %s", string(body), "test-body")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	body := strings.NewReader("test-body")

	resp, err := client.Post(context.Background(), "/test", body)
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code mismatch, got: %d, want: %d", resp.StatusCode, http.StatusOK)
	}
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		err       error
		wantRetry bool
	}{
		{
			err:       nil,
			wantRetry: false,
		},
		{
			err:       fmt.Errorf("connection refused"),
			wantRetry: true,
		},
		{
			err:       fmt.Errorf("timeout"),
			wantRetry: true,
		},
		{
			err:       fmt.Errorf("TLS handshake timeout"),
			wantRetry: true,
		},
		{
			err:       fmt.Errorf("invalid request"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(strconv.FormatBool(tt.wantRetry), func(t *testing.T) {
			if got := httpclient.IsRetriableError(tt.err); got != tt.wantRetry {
				t.Errorf("IsRetriableError() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}
