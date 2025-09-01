package utils

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type TestStruct struct {
	Value int
}

func (t *TestStruct) Add(a, b int) int {
	return a + b
}

func (t *TestStruct) MultiplyWithContext(ctx context.Context, a, b int) int {
	return a * b
}

func (t *TestStruct) SlowMethod(duration time.Duration) int {
	time.Sleep(duration)
	return t.Value
}

func (t *TestStruct) ErrorMethod() error {
	return errors.New("test error")
}

func TestNewMethodInvoker(t *testing.T) {
	ts := &TestStruct{}

	invoker, err := NewMethodInvoker(ts, "Add")
	if err != nil {
		t.Fatalf("NewMethodInvoker failed: %v", err)
	}

	if invoker.GetMethodName() != "Add" {
		t.Errorf("Expected method name 'Add', got '%s'", invoker.GetMethodName())
	}

	_, err = NewMethodInvoker(ts, "NonExistentMethod")
	if err == nil {
		t.Error("Expected error for non-existent method, got nil")
	}
}

func TestInvoke(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "Add")

	result, err := invoker.Invoke(2, 3)
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 5 {
		t.Errorf("Expected result 5, got %v", result[0].Int())
	}
}

func TestInvokeWithContext(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "MultiplyWithContext")

	ctx := context.Background()
	result, err := invoker.InvokeWithContext(ctx, 4, 5)
	if err != nil {
		t.Fatalf("InvokeWithContext failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 20 {
		t.Errorf("Expected result 20, got %v", result[0].Int())
	}
}

func TestInvokeWithTimeout(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "SlowMethod")

	_, err := invoker.InvokeWithTimeout(50*time.Millisecond, 100*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	result, err := invoker.InvokeWithTimeout(150*time.Millisecond, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("InvokeWithTimeout failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 0 {
		t.Errorf("Unexpected result: %v", result)
	}
}

func TestAsyncInvoke(t *testing.T) {
	ts := &TestStruct{Value: 42}
	invoker, _ := NewMethodInvoker(ts, "SlowMethod")

	resultChan := invoker.AsyncInvoke(50 * time.Millisecond)

	result := <-resultChan
	if result.Error != nil {
		t.Fatalf("AsyncInvoke failed: %v", result.Error)
	}

	if len(result.Values) != 1 || result.Values[0].Int() != 42 {
		t.Errorf("Expected result 42, got %v", result.Values[0].Int())
	}
}

func TestInvokeWithRetry(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "ErrorMethod")

	_, err := invoker.InvokeWithRetry(3, 10*time.Millisecond)
	if err == nil {
		t.Error("Expected error after retries, got nil")
	}

	invoker, _ = NewMethodInvoker(ts, "Add")
	result, err := invoker.InvokeWithRetry(3, 10*time.Millisecond, 2, 3)
	if err != nil {
		t.Fatalf("InvokeWithRetry failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 5 {
		t.Errorf("Expected result 5, got %v", result[0].Int())
	}
}

func TestGetMethodParams(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "Add")

	params := invoker.GetMethodParams()
	if len(params) != 2 {
		t.Fatalf("Expected 2 parameters, got %d", len(params))
	}

	if params[0] != reflect.TypeOf(0) || params[1] != reflect.TypeOf(0) {
		t.Errorf("Unexpected parameter types: %v", params)
	}
}

func TestGetMethodReturns(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "Add")

	returns := invoker.GetMethodReturns()
	if len(returns) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(returns))
	}

	if returns[0] != reflect.TypeOf(0) {
		t.Errorf("Unexpected return type: %v", returns[0])
	}
}

func TestMethodCache(t *testing.T) {
	cache := &MethodCache{}
	ts := &TestStruct{}

	invoker1, err := cache.GetOrCreateMethodInvoker(ts, "Add")
	if err != nil {
		t.Fatalf("GetOrCreateMethodInvoker failed: %v", err)
	}

	invoker2, err := cache.GetOrCreateMethodInvoker(ts, "Add")
	if err != nil {
		t.Fatalf("GetOrCreateMethodInvoker failed: %v", err)
	}

	if invoker1 != invoker2 {
		t.Error("Expected same invoker instance from cache")
	}

	invoker3, err := cache.GetOrCreateMethodInvoker(ts, "MultiplyWithContext")
	if err != nil {
		t.Fatalf("GetOrCreateMethodInvoker failed: %v", err)
	}

	if invoker1 == invoker3 {
		t.Error("Expected different invoker instances for different methods")
	}

	// Test cache clear
	cache.ClearCache()

	invoker4, err := cache.GetOrCreateMethodInvoker(ts, "Add")
	if err != nil {
		t.Fatalf("GetOrCreateMethodInvoker failed after cache clear: %v", err)
	}

	if invoker1 == invoker4 {
		t.Error("Expected different invoker instance after cache clear")
	}
}

func TestMethodCacheConcurrency(t *testing.T) {
	cache := &MethodCache{}
	ts := &TestStruct{}

	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.GetOrCreateMethodInvoker(ts, "Add")
			if err != nil {
				t.Errorf("Concurrent GetOrCreateMethodInvoker failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestBatchInvoke(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "Add")

	argsList := [][]any{
		{1, 2},
		{3, 4},
		{5, 6},
	}

	results := invoker.BatchInvoke(argsList)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	expectedResults := []int{3, 7, 11}
	for i, result := range results {
		if result.Error != nil {
			t.Errorf("BatchInvoke failed for args %v: %v", argsList[i], result.Error)
		}
		if len(result.Values) != 1 || result.Values[0].Int() != int64(expectedResults[i]) {
			t.Errorf("Expected result %d, got %v for args %v", expectedResults[i], result.Values[0].Int(), argsList[i])
		}
	}

	// Test with an error-producing method
	errorInvoker, _ := NewMethodInvoker(ts, "ErrorMethod")
	errorArgsList := [][]any{{}, {}, {}}

	errorResults := errorInvoker.BatchInvoke(errorArgsList)

	if len(errorResults) != 3 {
		t.Fatalf("Expected 3 error results, got %d", len(errorResults))
	}

	for i, result := range errorResults {
		if result.Error == nil {
			t.Errorf("Expected error for BatchInvoke %d, got nil", i)
		}
	}
}

func TestMethodProxy(t *testing.T) {
	ts := &TestStruct{}
	proxy := NewMethodProxy(ts)

	result, err := proxy.InvokeMethod("Add", 2, 3)
	if err != nil {
		t.Fatalf("MethodProxy InvokeMethod failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 5 {
		t.Errorf("Expected result 5, got %v", result[0].Int())
	}

	// Test invoking a non-existent method
	_, err = proxy.InvokeMethod("NonExistentMethod")
	if err == nil {
		t.Error("Expected error for non-existent method, got nil")
	}
}

func TestHookedMethodInvoker(t *testing.T) {
	ts := &TestStruct{}
	invoker, _ := NewMethodInvoker(ts, "Add")

	beforeCalled := false
	afterCalled := false

	beforeHook := func(methodName string, args []any) {
		beforeCalled = true
		t.Logf("Before hook called for method: %s", methodName)
	}

	afterHook := func(methodName string, args []any) {
		afterCalled = true
		t.Logf("After hook called for method: %s", methodName)
	}

	hookedInvoker := NewHookedMethodInvoker(invoker, beforeHook, afterHook)

	// Test normal method
	result, err := hookedInvoker.InvokeWithHooks(2, 3)
	if err != nil {
		t.Fatalf("HookedMethodInvoker InvokeWithHooks failed: %v", err)
	}

	if len(result) != 1 || result[0].Int() != 5 {
		t.Errorf("Expected result 5, got %v", result[0].Int())
	}

	if !beforeCalled || !afterCalled {
		t.Error("Hooks were not called for normal method")
	}

	// Test error method
	errorInvoker, _ := NewMethodInvoker(ts, "ErrorMethod")
	errorHookedInvoker := NewHookedMethodInvoker(errorInvoker, beforeHook, afterHook)

	beforeCalled = false
	afterCalled = false

	_, err = errorHookedInvoker.InvokeWithHooks()
	if err == nil {
		t.Error("Expected error from ErrorMethod, got nil")
	} else {
		t.Logf("Received expected error: %v", err)
	}

	if !beforeCalled || !afterCalled {
		t.Error("Hooks were not called for error method")
	}
}
