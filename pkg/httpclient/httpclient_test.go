package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/httpclient"
)

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

func TestClient_Put(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method mismatch, got: %s, want: %s", r.Method, http.MethodPut)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"key":"updated"}` {
			t.Errorf("Request body mismatch, got: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	resp, err := client.Put(context.Background(), "/test", map[string]string{"key": "updated"})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code mismatch, got: %d, want: %d", resp.StatusCode, http.StatusOK)
	}
}

func TestClient_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method mismatch, got: %s, want: %s", r.Method, http.MethodDelete)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	resp, err := client.Delete(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Status code mismatch, got: %d, want: %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestClient_SetHeader(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	client.SetHeader("Authorization", "Bearer token123")

	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer token123" {
		t.Errorf("Header not applied, got: %s, want: %s", gotAuth, "Bearer token123")
	}
}

func TestClient_WithHeaderOption(t *testing.T) {
	var gotAppID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAppID = r.Header.Get("X-App-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL, httpclient.WithHeader("X-App-ID", "test-app"))
	resp, err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer resp.Body.Close()

	if gotAppID != "test-app" {
		t.Errorf("Header not applied, got: %s, want: %s", gotAppID, "test-app")
	}
}

func TestClient_WithTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL, httpclient.WithTimeout(10*time.Millisecond), httpclient.WithRetries(0))

	_, err := client.Get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClient_GetJSON(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  error
		expectedResult map[string]any
	}{
		{
			name:         "success",
			statusCode:   http.StatusOK,
			responseBody: `{"message": "success", "data": {"id": 1}}`,
			expectedResult: map[string]any{
				"message": "success",
				"data": map[string]any{
					"id": float64(1),
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
			var result map[string]any

			err := client.GetJSON(context.Background(), "/test", nil, &result)

			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error: %v, got nil", tt.expectedError)
				}
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf("expected error: %v, got: %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetJSON() error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("result mismatch, got: %v, want: %v", result, tt.expectedResult)
			}
		})
	}
}

func TestClient_PostJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"echo": "ok"}`))
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	var result struct {
		Echo string `json:"echo"`
	}
	if err := client.PostJSON(context.Background(), "/test", map[string]string{"a": "b"}, &result); err != nil {
		t.Fatalf("PostJSON() error: %v", err)
	}
	if result.Echo != "ok" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClient_Retry(t *testing.T) {
	var callCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()

	client := httpclient.NewClient(
		ts.URL,
		httpclient.WithRetries(3),
		httpclient.WithBackoff(10*time.Millisecond),
	)

	var result struct {
		Message string `json:"message"`
	}

	err := client.GetJSON(context.Background(), "/test", nil, &result)
	if err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}

	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("unexpected call count, got: %d, want: %d", got, 3)
	}
	if result.Message != "success" {
		t.Errorf("unexpected result, got: %s, want: %s", result.Message, "success")
	}
}

// TestClient_Retry_ReplaysRequestBody 验证请求体在重试之间被正确重放：
// 老实现直接复用同一个 *http.Request，Body 在首次尝试后被 Transport 消费，
// 第二次尝试会发出空 body。这里第一次请求返回 500 触发重试，
// 断言服务端两次都收到了完整的请求体。
func TestClient_Retry_ReplaysRequestBody(t *testing.T) {
	var callCount int32
	var receivedBodies []string
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedBodies = append(receivedBodies, string(body))
		mu.Unlock()

		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL, httpclient.WithRetries(1), httpclient.WithBackoff(5*time.Millisecond))

	resp, err := client.Post(context.Background(), "/test", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	defer resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(receivedBodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(receivedBodies))
	}
	for i, b := range receivedBodies {
		if b != "payload" {
			t.Errorf("request #%d: expected body %q, got %q", i+1, "payload", b)
		}
	}
}

// TestClient_RetryExhausted 验证重试次数耗尽后返回 nil 响应与非空错误，
// 而不是像老实现那样返回一个 body 已被关闭的 *http.Response。
func TestClient_RetryExhausted(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL, httpclient.WithRetries(2), httpclient.WithBackoff(5*time.Millisecond))

	resp, err := client.Get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Fatalf("expected nil response, got: %v", resp)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("unexpected call count, got: %d, want: %d", got, 3)
	}
}

func TestClient_Request_InvalidURL(t *testing.T) {
	client := httpclient.NewClient("invalid-url")
	resp, err := client.Get(context.Background(), "/test", nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("expected url.Error, got: %T %v", err, err)
	}

	if !strings.Contains(err.Error(), "invalid-url") {
		t.Fatalf("expected error containing 'invalid-url', got: %v", err)
	}

	if resp != nil {
		t.Fatalf("expected nil response, got: %v", resp)
	}
}

func TestClient_Request_ContextCanceled(t *testing.T) {
	client := httpclient.NewClient("https://api.example.com")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

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

func TestClient_ConcurrentSetHeaderAndRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			client.SetHeader("X-Seq", "value")
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/test", nil)
			if err != nil {
				t.Errorf("Get() error: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name:      "nil",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "dial refused",
			err:       &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			wantRetry: true,
		},
		{
			name:      "read reset",
			err:       &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")},
			wantRetry: true,
		},
		{
			name:      "dns lookup failure is not retriable",
			err:       &net.OpError{Op: "lookup", Net: "tcp", Err: errors.New("no such host")},
			wantRetry: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			wantRetry: true,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			wantRetry: false,
		},
		{
			name:      "plain error",
			err:       errors.New("invalid request"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpclient.IsRetriableError(tt.err); got != tt.wantRetry {
				t.Errorf("IsRetriableError() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestClient_JSONEncoding(t *testing.T) {
	// 确认结构体作为响应值时也能正确处理（覆盖非 map 场景）。
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"count": 3})
	}))
	defer ts.Close()

	client := httpclient.NewClient(ts.URL)
	var result struct {
		Count int `json:"count"`
	}
	if err := client.GetJSON(context.Background(), "/test", nil, &result); err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}
	if result.Count != 3 {
		t.Errorf("unexpected count: %d", result.Count)
	}
}
