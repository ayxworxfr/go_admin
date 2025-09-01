package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unicode"

	"github.com/ettle/strcase"
	"github.com/mitchellh/mapstructure"
)

// Repository 通用仓储接口
type Repository[T any] interface {
	TransactionExecutor
	Create(ctx context.Context, model *T) error
	Update(ctx context.Context, model *T) error
	Delete(ctx context.Context, model *T) error
	DeleteByID(ctx context.Context, id any) error
	DeleteByOption(ctx context.Context, opts *QueryOption) error
	UpdateByOption(ctx context.Context, model any, opts *QueryOption) error
	Find(ctx context.Context, model *T) (*T, error)
	FindByID(ctx context.Context, id any) (*T, error)
	FindByKey(ctx context.Context, key string, value any) (*T, error)
	FindAll(ctx context.Context, model *T) ([]T, error)
	FindPage(ctx context.Context, query any, limit, offset int) ([]T, int64, error)
	FindPageV2(ctx context.Context, limit, offset int, orderBy string) ([]T, int64, error)
	BatchCreate(ctx context.Context, models []T) error
	BatchUpdate(ctx context.Context, models []T) error
	BatchDelete(ctx context.Context, models []T) error
	QueryBuilder() *QueryBuilder[T]
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) ([]T, error)
	QueryRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
}

// GenericRepository 通用仓储实现
type GenericRepository[T any] struct {
	processor ORMProcessor
}

// NewRepository 创建仓储实例
func NewRepository[T any](processor ORMProcessor) Repository[T] {
	return &GenericRepository[T]{processor: processor}
}

// Create 插入单条记录
func (r *GenericRepository[T]) Create(ctx context.Context, model *T) error {
	return r.processor.Create(ctx, model)
}

// Update 更新记录
func (r *GenericRepository[T]) Update(ctx context.Context, model *T) error {
	return r.processor.Update(ctx, model)
}

// UpdateByOption 根据条件更新记录
func (r *GenericRepository[T]) UpdateByOption(ctx context.Context, model any, opts *QueryOption) error {
	return r.processor.UpdateByOption(ctx, model, opts)
}

// DeleteByOption 根据条件删除记录
func (r *GenericRepository[T]) DeleteByOption(ctx context.Context, opts *QueryOption) error {
	model := new(T)
	return r.processor.DeleteByOption(ctx, model, opts)
}

// Delete 删除记录
func (r *GenericRepository[T]) Delete(ctx context.Context, model *T) error {
	return r.processor.Delete(ctx, model)
}

// DeleteByID 根据ID删除记录
func (r *GenericRepository[T]) DeleteByID(ctx context.Context, id any) error {
	// 创建空模型实例
	model := new(T)
	_, idField, err := r.getIDFieldName(model)
	if err != nil {
		return err
	}
	// 设置ID值
	idField.Set(reflect.ValueOf(id))
	// 执行删除
	return r.processor.Delete(ctx, model)
}

// FindByID 根据ID查询
func (r *GenericRepository[T]) FindByID(ctx context.Context, id any) (*T, error) {
	idField, _, err := r.getIDFieldName(new(T))
	if err != nil {
		return nil, err
	}
	return r.FindByKey(ctx, idField, id)
}

// getIDField 获取模型ID字段名称
func (r *GenericRepository[T]) getIDFieldName(model *T) (string, reflect.Value, error) {
	// 获取模型类型元数据
	modelType := reflect.TypeOf(model).Elem()
	modelName := modelType.Name()

	// 拆分驼峰命名为单词，取最后一段（核心逻辑）
	lastCamelPart := getLastCamelPart(modelName)

	// 定义字段检查函数
	checkField := func(fieldName string) (reflect.Value, bool) {
		field := reflect.ValueOf(model).Elem().FieldByName(fieldName)
		return field, field.IsValid()
	}

	// 可能的ID字段命名（按优先级排序）
	possibleFieldNames := []string{
		"ID", // 通用ID字段（最高优先级）
		"Id",
		fmt.Sprintf("%sID", modelName),     // 模型名+ID（如UserID）
		fmt.Sprintf("%sId", modelName),     // 模型名+Id（如UserId）
		fmt.Sprintf("%sID", lastCamelPart), // 驼峰最后一段+ID（如OpportunityID）
		fmt.Sprintf("%sId", lastCamelPart), // 驼峰最后一段+Id（如OpportunityId）
	}

	// 循环检查字段
	for _, fieldName := range possibleFieldNames {
		if field, ok := checkField(fieldName); ok {
			return strcase.ToSnake(fieldName), field, nil
		}
	}

	return "", reflect.ValueOf(nil), errors.New("model does not have an ID field")
}

// FindByKey 根据Key查询
func (r *GenericRepository[T]) FindByKey(ctx context.Context, key string, value any) (*T, error) {
	var result T
	queryOpts := &QueryOption{
		Filters: []Condition{
			{Field: key, Op: OpEq, Value: value},
		},
		Limit: 2,
	}

	resultPtr := reflect.New(reflect.TypeOf(result)).Interface()
	queryResult, err := r.processor.Query(ctx, resultPtr, queryOpts)
	if err != nil {
		return nil, err
	}

	resultSlice, ok := queryResult.Data.([]T)
	if !ok || len(resultSlice) == 0 {
		return nil, errors.New("record not found")
	}

	if len(resultSlice) > 1 {
		return nil, errors.New("multiple records found")
	}

	return &resultSlice[0], nil
}

// FindAll 根据模型中的非零值字段条件查询所有记录
func (r *GenericRepository[T]) FindAll(ctx context.Context, model *T) ([]T, error) {
	if model == nil {
		return nil, errors.New("model cannot be nil")
	}

	// 直接使用 BuildFiltersFromModel 构建查询条件
	filters := r.processor.BuildFiltersFromModel(model)

	// 执行查询，不限制数量（获取所有匹配记录）
	result, err := r.processor.Query(ctx, new(T), &QueryOption{
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}

	// 类型转换并返回结果
	return result.Data.([]T), nil
}

// Find 根据模型中的非零字段条件查询单条记录
func (r *GenericRepository[T]) Find(ctx context.Context, model *T) (*T, error) {
	if model == nil {
		return nil, errors.New("model cannot be nil")
	}

	// 使用QueryBuilder构建查询条件
	queryOpts := &QueryOption{
		Filters: r.processor.BuildFiltersFromModel(model),
		Limit:   2, // 设置为2，确保只返回单条记录
	}

	result, err := r.processor.Query(ctx, model, queryOpts)
	if err != nil {
		return nil, err
	}

	data, ok := result.Data.([]T)
	if !ok || len(data) == 0 {
		return nil, errors.New("record not found")
	}

	if len(data) > 1 {
		return nil, errors.New("multiple records found")
	}

	return &data[0], nil
}

// FindPage 根据模型条件分页查询
func (r *GenericRepository[T]) FindPage(ctx context.Context, query any, limit, offset int) ([]T, int64, error) {
	if query == nil {
		return nil, 0, errors.New("model cannot be nil")
	}

	// 构建查询条件
	filters := r.processor.BuildFiltersFromModel(query)

	// 创建查询选项
	queryOpts := &QueryOption{
		Filters: filters,
		Limit:   limit,
		Offset:  offset,
	}

	// 执行查询
	result, err := r.processor.Query(ctx, new(T), queryOpts)
	if err != nil {
		return nil, 0, err
	}

	// 解析结果
	data, ok := result.Data.([]T)
	if !ok {
		return nil, 0, errors.New("invalid result type")
	}

	return data, result.Total, nil
}

// FindPage 分页查询
func (r *GenericRepository[T]) FindPageV2(ctx context.Context, limit, offset int, orderBy string) ([]T, int64, error) {
	queryOpts := &QueryOption{
		OrderBy: orderBy,
		Limit:   limit,
		Offset:  offset,
	}

	result, err := r.processor.Query(ctx, new(T), queryOpts)
	if err != nil {
		return nil, 0, err
	}

	data, ok := result.Data.([]T)
	if !ok {
		return nil, 0, errors.New("invalid result type")
	}

	return data, result.Total, nil
}

// BatchCreate 批量插入
func (r *GenericRepository[T]) BatchCreate(ctx context.Context, models []T) error {
	interfaceModels := make([]any, len(models))
	for i, m := range models {
		interfaceModels[i] = m
	}
	return r.processor.BatchCreate(ctx, interfaceModels)
}

// BatchUpdate 批量更新
func (r *GenericRepository[T]) BatchUpdate(ctx context.Context, models []T) error {
	interfaceModels := make([]any, len(models))
	for i, m := range models {
		interfaceModels[i] = m
	}
	return r.processor.BatchUpdate(ctx, interfaceModels)
}

// BatchDelete 批量删除
func (r *GenericRepository[T]) BatchDelete(ctx context.Context, models []T) error {
	interfaceModels := make([]any, len(models))
	for i, m := range models {
		interfaceModels[i] = m
	}
	return r.processor.BatchDelete(ctx, interfaceModels)
}

// QueryBuilder 获取链式查询构建器
func (r *GenericRepository[T]) QueryBuilder() *QueryBuilder[T] {
	return NewQueryBuilder[T](r.processor)
}

// Exec 执行SQL语句
func (r *GenericRepository[T]) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	return r.processor.Exec(ctx, sql, args...)
}

// Query 查询单行结果
func (r *GenericRepository[T]) Query(ctx context.Context, sql string, args ...any) ([]T, error) {
	rows, err := r.processor.QueryRows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	var result []T
	for _, row := range rows {
		var item T
		if err := MapToStruct(row, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// QueryRows 查询多行结果
func (r *GenericRepository[T]) QueryRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	return r.processor.QueryRows(ctx, sql, args...)
}

// GenericRepository 实现 TransactionExecutor 接口
func (r *GenericRepository[T]) Transaction(ctx context.Context, fn TransactionFunc) (any, error) {
	return r.processor.Transaction(ctx, fn)
}

// Begin 开始事务（仓储层代理方法）
func (r *GenericRepository[T]) Begin(ctx context.Context) (context.Context, error) {
	return r.processor.Begin(ctx)
}

// Commit 提交事务（仓储层代理方法）
func (r *GenericRepository[T]) Commit(ctx context.Context) error {
	return r.processor.Commit(ctx)
}

// Rollback 回滚事务（仓储层代理方法）
func (r *GenericRepository[T]) Rollback(ctx context.Context) error {
	return r.processor.Rollback(ctx)
}

// MapToStruct 将map[string]any转换为结构体
func MapToStruct(src map[string]any, dst any) error {
	config := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           dst,
		TagName:          "json", // 使用json标签
		WeaklyTypedInput: true,   // 允许弱类型输入
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			// 处理[]byte到string的转换
			mapstructure.StringToSliceHookFunc(","),
			// 处理[]byte到数值类型的转换
			byteSliceToPrimitiveHookFunc(),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(src)
}

// byteSliceToPrimitiveHookFunc 处理[]byte到基本类型的转换
func byteSliceToPrimitiveHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data any,
	) (any, error) {
		// 检查源类型是否为[]byte
		if f.Kind() != reflect.Slice || f.Elem().Kind() != reflect.Uint8 {
			return data, nil
		}

		// 将[]byte转换为string
		str := string(data.([]byte))

		// 根据目标类型进行转换
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.ParseInt(str, 10, 64)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return strconv.ParseUint(str, 10, 64)
		case reflect.Float32, reflect.Float64:
			return strconv.ParseFloat(str, 64)
		case reflect.Bool:
			return strconv.ParseBool(str)
		case reflect.String:
			return str, nil
		case reflect.Struct:
			// 检查是否为time.Time类型
			if t.String() == "time.Time" {
				return parseTime(str)
			}
		}

		// 不支持的类型，返回原始数据
		return data, nil
	}
}

// parseTime 尝试多种格式解析时间字符串
func parseTime(str string) (time.Time, error) {
	// 定义常见的时间格式
	formats := []string{
		time.RFC3339,          // "2006-01-02T15:04:05Z07:00"
		"2006-01-02 15:04:05", // 常见的日期时间格式
		"2006-01-02",          // 只有日期
		"2006/01/02 15:04:05", // 斜杠分隔的日期时间
		"2006/01/02",          // 只有日期（斜杠分隔）
		"02/01/2006 15:04:05", // 欧洲格式：日/月/年
		"02/01/2006",          // 只有日期（欧洲格式）
		"01/02/2006 15:04:05", // 美国格式：月/日/年
		"01/02/2006",          // 只有日期（美国格式）
	}

	// 尝试每种格式
	for _, format := range formats {
		if t, err := time.Parse(format, str); err == nil {
			return t, nil
		}
	}

	// 尝试解析时间戳（秒）
	if unixTime, err := strconv.ParseInt(str, 10, 64); err == nil {
		return time.Unix(unixTime, 0), nil
	}

	// 尝试解析时间戳（毫秒）
	if unixTimeMs, err := strconv.ParseInt(str, 10, 64); err == nil {
		return time.Unix(0, unixTimeMs*int64(time.Millisecond)), nil
	}

	return time.Time{}, errors.New("Unrecognized time format: " + str)
}

// getLastCamelPart 拆分驼峰命名（大驼峰），返回最后一段单词
// 例：SalesOpportunity -> Opportunity；User -> User；CustomerContactInfo -> Info
func getLastCamelPart(camelName string) string {
	if camelName == "" {
		return ""
	}

	// 拆分驼峰为单词（大驼峰规则：大写字母前分割）
	var parts []string
	start := 0
	for i, r := range camelName {
		if unicode.IsUpper(r) && i > start {
			parts = append(parts, camelName[start:i])
			start = i
		}
	}
	// 添加最后一段
	parts = append(parts, camelName[start:])

	// 返回最后一段
	return parts[len(parts)-1]
}
