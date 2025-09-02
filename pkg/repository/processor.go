package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"xorm.io/xorm"
)

// XormProcessor xorm处理器实现
type XormProcessor struct {
	engine *xorm.Engine
}

// NewXormProcessor 创建xorm处理器
func NewXormProcessor(engine *xorm.Engine) *XormProcessor {
	return &XormProcessor{engine: engine}
}

// 从上下文中获取会话（优先使用事务会话）
func (p *XormProcessor) getSession(ctx context.Context) *xorm.Session {
	if session, ok := ctx.Value(TransactionKeyInstance).(*xorm.Session); ok && session != nil {
		return session
	}
	// 没有事务时创建新会话
	return p.engine.NewSession()
}

// Create 实现ORM创建
func (p *XormProcessor) Create(ctx context.Context, model any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		return session.Insert(model)
	})
	return err
}

// Update 实现ORM更新
func (p *XormProcessor) Update(ctx context.Context, model any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		_, idFieldValue, err := p.getIDFieldName(model)
		if err != nil {
			return nil, err
		}
		if !idFieldValue.IsValid() || (idFieldValue.Kind() == reflect.Ptr && idFieldValue.IsNil()) {
			return nil, errors.New("model must have a valid primary key field")
		}
		return session.ID(idFieldValue.Interface()).Update(model)
	})
	return err
}

func (p *XormProcessor) getIDFieldName(model any) (string, reflect.Value, error) {
	t := reflect.TypeOf(model).Elem()
	v := reflect.ValueOf(model).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag := field.Tag.Get("xorm"); strings.Contains(tag, "pk") {
			// 解析 xorm 标签以获取实际的数据库列名
			tagParts := strings.Split(tag, " ")
			for _, part := range tagParts {
				if strings.HasPrefix(part, "'") && strings.HasSuffix(part, "'") {
					// 找到了数据库列名
					columnName := strings.Trim(part, "'")
					return columnName, v.Field(i), nil
				}
			}
			// 如果没有找到明确的列名，就使用字段名
			return field.Name, v.Field(i), nil
		}
	}

	return "", reflect.Value{}, fmt.Errorf("no primary key found in model")
}

// Delete 实现ORM删除
func (p *XormProcessor) Delete(ctx context.Context, model any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		return session.Delete(model)
	})
	return err
}

// UpdateByOption 根据查询选项更新记录
func (p *XormProcessor) UpdateByOption(ctx context.Context, model any, opts *QueryOption) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		// 应用过滤条件
		for _, filter := range opts.Filters {
			session = applyCondition(session, filter)
		}

		// 应用排序（可选，更新场景可能不需要排序）
		if opts.OrderBy != "" {
			session = session.OrderBy(opts.OrderBy)
		}

		// 执行更新（Xorm的Update方法会根据模型类型生成表名）
		_, err := session.Update(model)
		return nil, err
	})
	return err
}

// DeleteByOption 根据查询选项删除记录
func (p *XormProcessor) DeleteByOption(ctx context.Context, model any, opts *QueryOption) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		// 应用过滤条件
		for _, filter := range opts.Filters {
			session = applyCondition(session, filter) // 复用之前的条件应用函数
		}

		// 应用排序（可选，删除场景可能不需要排序）
		if opts.OrderBy != "" {
			session = session.OrderBy(opts.OrderBy)
		}

		// 执行删除（Xorm的Delete方法会根据模型类型生成表名）
		_, err := session.Delete(model)
		return nil, err
	})
	return err
}

// Query 实现ORM查询
func (p *XormProcessor) Query(ctx context.Context, model any, opts *QueryOption) (*QueryResult, error) {
	session := p.getSession(ctx)

	// 应用过滤条件
	for _, filter := range opts.Filters {
		session = applyCondition(session, filter)
	}

	// 应用排序
	if opts.OrderBy != "" {
		session = session.OrderBy(opts.OrderBy)
	}

	// 应用分页
	if opts.Limit > 0 {
		session = session.Limit(opts.Limit, opts.Offset)
	}

	// 应用锁
	if opts.Lock != "" {
		session = session.ForUpdate()
	}

	// 查询数据
	sliceType := reflect.SliceOf(reflect.TypeOf(model).Elem())
	slicePtr := reflect.New(sliceType)
	_, err := p.executeBySession(ctx, session, func(session *xorm.Session) (any, error) {
		return nil, session.Find(slicePtr.Interface())
	})
	if err != nil {
		return nil, err
	}

	for _, filter := range opts.Filters {
		session = applyCondition(session, filter)
	}
	// 查询总数
	total, err := p.executeBySession(ctx, session, func(session *xorm.Session) (any, error) {
		return session.Count(model)
	})
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Data:  slicePtr.Elem().Interface(),
		Total: total.(int64),
	}, nil
}

// BatchCreate 批量插入
func (p *XormProcessor) BatchCreate(ctx context.Context, models []any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		for _, model := range models {
			if _, err := session.Insert(model); err != nil {
				return nil, err
			}
		}
		return len(models), nil
	})
	return err
}

// BatchUpdate 批量更新
func (p *XormProcessor) BatchUpdate(ctx context.Context, models []any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		for _, model := range models {
			idField := reflect.ValueOf(model).Elem().FieldByName("ID")
			if !idField.IsValid() {
				return nil, errors.New("model must have ID field")
			}
			if _, err := session.ID(idField.Interface()).Update(model); err != nil {
				return nil, err
			}
		}
		return len(models), nil
	})
	return err
}

// BatchDelete 批量删除
func (p *XormProcessor) BatchDelete(ctx context.Context, models []any) error {
	_, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		for _, model := range models {
			if _, err := session.Delete(model); err != nil {
				return nil, err
			}
		}
		return len(models), nil
	})
	return err
}

// Exec 执行SQL语句
func (p *XormProcessor) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	count, err := p.ExecuteInTransaction(ctx, func(session *xorm.Session) (any, error) {
		sqlOrArgs := append([]any{sql}, args...)
		result, err := session.Exec(sqlOrArgs...)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	})
	return count.(int64), err
}

// Transaction 执行事务(支持事务嵌套)
func (p *XormProcessor) ExecuteInTransaction(ctx context.Context, fn func(*xorm.Session) (any, error)) (any, error) {
	// 检查是否已在事务中
	if session, ok := ctx.Value(TransactionKeyInstance).(*xorm.Session); ok {
		return p.executeFunWithSpanEvent(ctx, session, fn)
	}

	// 创建新事务
	return p.Transaction(ctx, func(txCtx context.Context) (any, error) {
		return p.executeFunWithSpanEvent(txCtx, p.getSession(txCtx), fn)
	})
}

func (p *XormProcessor) executeFunWithSpanEvent(ctx context.Context, session *xorm.Session, fn func(*xorm.Session) (any, error)) (any, error) {
	start := time.Now()
	result, err := fn(session)
	sql, args := session.LastSQL()
	info := map[string]any{
		"sql":      sql,
		"args":     args,
		"duration": time.Since(start),
	}
	if len(args) == 0 {
		delete(info, "args")
	}
	RecordDbEvent(ctx, info)
	return result, err
}

func (p *XormProcessor) executeBySession(ctx context.Context, session *xorm.Session, fn func(*xorm.Session) (any, error)) (any, error) {
	start := time.Now()
	result, err := fn(session)
	sql, args := session.LastSQL()
	info := map[string]any{
		"sql":      sql,
		"args":     args,
		"duration": time.Since(start),
	}
	if len(args) == 0 {
		delete(info, "args")
	}
	RecordDbEvent(ctx, info)
	return result, err
}

// QueryRow 查询单行结果
func (p *XormProcessor) QueryRow(ctx context.Context, sql string, args ...any) (map[string]any, error) {
	sqlOrArgs := append([]any{sql}, args...)
	data, err := p.executeBySession(ctx, p.getSession(ctx), func(session *xorm.Session) (any, error) {
		return session.Query(sqlOrArgs...)
	})
	if err != nil {
		return nil, err
	}
	rows, _ := data.([]map[string][]byte)
	if len(rows) == 0 {
		return nil, errors.New("record not found")
	}

	// 类型转换
	row := rows[0]
	result := make(map[string]any)
	for k, v := range row {
		result[k] = string(v)
	}

	return result, nil
}

// QueryRows 查询多行结果
func (p *XormProcessor) QueryRows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	sqlOrArgs := append([]any{sql}, args...)
	data, err := p.executeBySession(ctx, p.getSession(ctx), func(session *xorm.Session) (any, error) {
		return session.Query(sqlOrArgs...)
	})
	if err != nil {
		return nil, err
	}
	rows, _ := data.([]map[string][]byte)

	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		result[i] = make(map[string]any)
		for k, v := range row {
			result[i][k] = v
		}
	}

	return result, nil
}

// 辅助函数：应用查询条件
func applyCondition(session *xorm.Session, cond Condition) *xorm.Session {
	// 处理 IN 和 NOT IN 条件的切片参数
	if cond.Op == OpIn || cond.Op == OpNotIn {
		// 转换为 []any
		var interfaceSlice []any

		// 检查是否已经是 []any
		if sliceVal, ok := cond.Value.([]any); ok {
			interfaceSlice = sliceVal
		} else {
			// 使用反射转换其他切片类型
			v := reflect.ValueOf(cond.Value)
			if v.Kind() != reflect.Slice {
				log.Printf("Invalid type for IN condition: %T", cond.Value)
				return session
			}

			interfaceSlice = make([]any, v.Len())
			for i := 0; i < v.Len(); i++ {
				interfaceSlice[i] = v.Index(i).Interface()
			}
		}

		// 注意：这里使用 session.In 而非 session.Where
		return session.In(cond.Field, interfaceSlice...)
	}

	// 处理其他操作符
	switch cond.Op {
	case OpEq:
		return session.Where(cond.Field+" = ?", cond.Value)
	case OpNe:
		return session.Where(cond.Field+" != ?", cond.Value)
	case OpGt:
		return session.Where(cond.Field+" > ?", cond.Value)
	case OpLt:
		return session.Where(cond.Field+" < ?", cond.Value)
	case OpGe:
		return session.Where(cond.Field+" >= ?", cond.Value)
	case OpLe:
		return session.Where(cond.Field+" <= ?", cond.Value)
	case OpLike:
		return session.Where(cond.Field+" LIKE ?", fmt.Sprintf("%%%s%%", cond.Value))
	case OpStartsWith:
		return session.Where(cond.Field+" LIKE ?", fmt.Sprintf("%s%%", cond.Value))
	case OpEndsWith:
		return session.Where(cond.Field+" LIKE ?", fmt.Sprintf("%%%s", cond.Value))
	default:
		return session
	}
}

// BuildFiltersFromModel 从模型中提取非零字段作为查询条件
func (p *XormProcessor) BuildFiltersFromModel(model any) []Condition {
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var filters []Condition
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		value := val.Field(i)

		if value.IsZero() || value.Kind() == reflect.Ptr && value.IsNil() {
			continue // 跳过零值字段
		}
		if value.Kind() == reflect.Struct {
			continue // 跳过结构体
		}

		// 解析xorm标签获取数据库字段名和操作符
		dbField, op := p.parseXormTag(field.Tag.Get("xorm"))
		if dbField == "" {
			continue // 跳过没有xorm标签的字段
		}

		filters = append(filters, Condition{
			Field: dbField,
			Op:    op,
			Value: value.Interface(),
		})
	}

	return filters
}

// parseXormTag 解析xorm标签获取字段名和操作符
func (p *XormProcessor) parseXormTag(tag string) (string, Op) {
	// 处理空标签
	if tag == "" {
		return "", OpEq
	}

	// 首先提取可能存在的单引号或反引号内的字段名
	var fieldName string
	inQuote := false
	quoteChar := rune(0)
	quoteStart := -1

	for i, r := range tag {
		if r == '\'' || r == '`' {
			if !inQuote {
				// 开始引号
				inQuote = true
				quoteChar = r
				quoteStart = i
			} else if r == quoteChar {
				// 结束引号
				fieldName = tag[quoteStart+1 : i]
				inQuote = false
				break
			}
		}
	}

	// 如果没有找到引号内的字段名，则使用第一个单词作为字段名
	if fieldName == "" {
		parts := strings.FieldsFunc(tag, func(r rune) bool {
			return r == ' ' || r == '`' || r == '\''
		})
		if len(parts) > 0 {
			fieldName = parts[0]
		}
	}

	// 查找操作符
	op := OpEq
	opIndex := strings.Index(tag, "op=")
	if opIndex != -1 {
		// 找到操作符声明，提取操作符值
		start := opIndex + 3 // 跳过"op="
		end := start
		for end < len(tag) && tag[end] != ' ' && tag[end] != '`' && tag[end] != '\'' {
			end++
		}
		if start < end {
			op = Op(tag[start:end])
		}
	}

	return fieldName, op
}

// Begin 开始一个数据库事务
func (p *XormProcessor) Begin(ctx context.Context) (context.Context, error) {
	session := p.engine.NewSession()
	if err := session.Begin(); err != nil {
		session.Close()
		return ctx, err
	}
	return context.WithValue(ctx, TransactionKeyInstance, session), nil
}

// Commit 提交事务
func (p *XormProcessor) Commit(ctx context.Context) error {
	session, ok := ctx.Value(TransactionKeyInstance).(*xorm.Session)
	if !ok || session == nil {
		return errors.New("transaction session not found in context")
	}
	defer session.Close()
	return session.Commit()
}

// Rollback 回滚事务
func (p *XormProcessor) Rollback(ctx context.Context) error {
	session, ok := ctx.Value(TransactionKeyInstance).(*xorm.Session)
	if !ok || session == nil {
		return errors.New("transaction session not found in context")
	}
	defer session.Close()
	return session.Rollback()
}

// XormProcessor 实现 TransactionExecutor 接口
func (p *XormProcessor) Transaction(ctx context.Context, fn TransactionFunc) (any, error) {
	txCtx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			// 发生 panic 时回滚事务
			_ = p.Rollback(txCtx)
			panic(r)
		}
	}()

	// 执行事务函数并获取结果
	result, err := fn(txCtx)
	if err != nil {
		// 执行失败时回滚事务
		rollbackErr := p.Rollback(txCtx)
		if rollbackErr != nil {
			log.Printf("Rollback failed: %v", rollbackErr)
		}
		return nil, err
	}

	// 执行成功时提交事务
	commitErr := p.Commit(txCtx)
	if commitErr != nil {
		return nil, commitErr
	}

	return result, nil
}
