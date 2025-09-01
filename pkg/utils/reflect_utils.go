package utils

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// MethodInvoker 封装反射方法调用
type MethodInvoker struct {
	structValue reflect.Value
	methodValue reflect.Value
	methodType  reflect.Type
	methodName  string
}

type InvokeResult struct {
	Values []reflect.Value
	Error  error
}

// NewMethodInvoker 创建方法调用器
func NewMethodInvoker(structPtr any, methodName string) (*MethodInvoker, error) {
	structValue := reflect.ValueOf(structPtr)
	if structValue.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("structPtr must be a pointer to a struct")
	}

	methodValue := structValue.MethodByName(methodName)
	if !methodValue.IsValid() {
		return nil, fmt.Errorf("method %s does not exist or is not exported", methodName)
	}

	return &MethodInvoker{
		structValue: structValue,
		methodValue: methodValue,
		methodType:  methodValue.Type(),
		methodName:  methodName,
	}, nil
}

// Invoke 调用方法
func (invoker *MethodInvoker) Invoke(args ...any) ([]reflect.Value, error) {
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	out := invoker.methodValue.Call(in)

	// 检查是否有错误返回
	if len(out) > 0 {
		lastValue := out[len(out)-1]
		if lastValue.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !lastValue.IsNil() {
				return out[:len(out)-1], lastValue.Interface().(error)
			}
		}
	}

	return out, nil
}

// InvokeWithContext 调用带context参数的方法
func (invoker *MethodInvoker) InvokeWithContext(ctx context.Context, args ...any) ([]reflect.Value, error) {
	if invoker.methodType.NumIn() < 1 || !invoker.methodType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		return nil, fmt.Errorf("method does not have context.Context as its first parameter")
	}

	allArgs := make([]any, 0, len(args)+1)
	allArgs = append(allArgs, ctx)
	allArgs = append(allArgs, args...)

	return invoker.Invoke(allArgs...)
}

// InvokeWithTimeout 带超时的方法调用
func (invoker *MethodInvoker) InvokeWithTimeout(timeout time.Duration, args ...any) ([]reflect.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultChan := make(chan InvokeResult, 1)
	go func() {
		values, err := invoker.Invoke(args...)
		resultChan <- InvokeResult{Values: values, Error: err}
	}()

	select {
	case result := <-resultChan:
		return result.Values, result.Error
	case <-ctx.Done():
		return nil, fmt.Errorf("method invocation timed out after %v", timeout)
	}
}

// AsyncInvoke 异步调用方法
func (invoker *MethodInvoker) AsyncInvoke(args ...any) <-chan InvokeResult {
	resultChan := make(chan InvokeResult, 1)
	go func() {
		values, err := invoker.Invoke(args...)
		resultChan <- InvokeResult{Values: values, Error: err}
		close(resultChan)
	}()
	return resultChan
}

// InvokeWithRetry 带重试的方法调用
func (invoker *MethodInvoker) InvokeWithRetry(maxRetries int, retryInterval time.Duration, args ...any) ([]reflect.Value, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		result, err := invoker.Invoke(args...)
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(retryInterval)
	}
	return nil, fmt.Errorf("failed after %d retries: %v", maxRetries, lastErr)
}

// GetMethodName 获取方法名称
func (invoker *MethodInvoker) GetMethodName() string {
	return invoker.methodName
}

// GetMethodParams 获取方法参数类型
func (invoker *MethodInvoker) GetMethodParams() []reflect.Type {
	params := make([]reflect.Type, invoker.methodType.NumIn())
	for i := 0; i < invoker.methodType.NumIn(); i++ {
		params[i] = invoker.methodType.In(i)
	}
	return params
}

// GetMethodReturns 获取方法返回值类型
func (invoker *MethodInvoker) GetMethodReturns() []reflect.Type {
	returns := make([]reflect.Type, invoker.methodType.NumOut())
	for i := 0; i < invoker.methodType.NumOut(); i++ {
		returns[i] = invoker.methodType.Out(i)
	}
	return returns
}

// BatchInvoke 并发批量调用方法
func (invoker *MethodInvoker) BatchInvoke(argsList [][]any) []InvokeResult {
	results := make([]InvokeResult, len(argsList))
	var wg sync.WaitGroup
	for i, args := range argsList {
		wg.Add(1)
		go func(index int, methodArgs []any) {
			defer wg.Done()
			values, err := invoker.Invoke(methodArgs...)
			results[index] = InvokeResult{Values: values, Error: err}
		}(i, args)
	}
	wg.Wait()
	return results
}

// MethodCache 方法缓存
type MethodCache struct {
	cache sync.Map
}

// GetOrCreateMethodInvoker 从缓存中获取或创建方法调用器
func (cache *MethodCache) GetOrCreateMethodInvoker(structPtr any, methodName string) (*MethodInvoker, error) {
	structType := reflect.TypeOf(structPtr)
	if structType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("structPtr must be a pointer to a struct")
	}
	structType = structType.Elem()

	key := structType.String() + "." + methodName

	// 尝试从缓存中获取
	if cached, ok := cache.cache.Load(key); ok {
		return cached.(*MethodInvoker), nil
	}

	// 创建新的调用器
	invoker, err := NewMethodInvoker(structPtr, methodName)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	cache.cache.Store(key, invoker)
	return invoker, nil
}

// ClearCache 清空缓存
func (cache *MethodCache) ClearCache() {
	cache.cache = sync.Map{}
}

// MethodProxy 动态代理
type MethodProxy struct {
	target any
	cache  *MethodCache
}

func NewMethodProxy(target any) *MethodProxy {
	return &MethodProxy{
		target: target,
		cache:  &MethodCache{},
	}
}

func (mp *MethodProxy) InvokeMethod(methodName string, args ...any) ([]reflect.Value, error) {
	invoker, err := mp.cache.GetOrCreateMethodInvoker(mp.target, methodName)
	if err != nil {
		return nil, err
	}
	return invoker.Invoke(args...)
}

type HookFunc func(methodName string, args []any)

type HookedMethodInvoker struct {
	*MethodInvoker
	beforeHook HookFunc
	afterHook  HookFunc
}

func NewHookedMethodInvoker(invoker *MethodInvoker, before, after HookFunc) *HookedMethodInvoker {
	return &HookedMethodInvoker{
		MethodInvoker: invoker,
		beforeHook:    before,
		afterHook:     after,
	}
}

func (hmi *HookedMethodInvoker) InvokeWithHooks(args ...any) ([]reflect.Value, error) {
	if hmi.beforeHook != nil {
		hmi.beforeHook(hmi.methodName, args)
	}

	result, err := hmi.Invoke(args...)

	if hmi.afterHook != nil {
		hmi.afterHook(hmi.methodName, args)
	}

	return result, err
}
