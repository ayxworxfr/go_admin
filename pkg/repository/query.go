package repository

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Condition 单个查询条件（叶子节点）
type Condition struct {
	Field string
	Op    Op
	Value any
}

// Op 查询操作符
type Op string

func (op Op) String() string { return string(op) }

const (
	OpEq    Op = "eq"
	OpNe    Op = "ne"
	OpGt    Op = "gt"
	OpLt    Op = "lt"
	OpGe    Op = "ge"
	OpLe    Op = "le"
	OpLike  Op = "like"
	OpIn    Op = "in"
	OpNotIn Op = "notin"
)

// joinKind 描述子表达式与前一个兄弟的连接方式。
type joinKind uint8

const (
	joinAnd joinKind = iota
	joinOr
)

// exprNode 条件树节点：叶子是单条件，枝是分组（对应 SQL 括号）。
type exprNode struct {
	join     joinKind
	cond     *Condition
	children []*exprNode
}

func (n *exprNode) isGroup() bool {
	return n != nil && len(n.children) > 0
}

func toBuilderCond(nodes []*exprNode) builder.Cond {
	var result builder.Cond
	for _, n := range nodes {
		part := nodeToCond(n)
		if part == nil {
			continue
		}
		if result == nil {
			result = part
			continue
		}
		if n.join == joinOr {
			result = result.Or(part)
		} else {
			result = result.And(part)
		}
	}
	return result
}

func nodeToCond(n *exprNode) builder.Cond {
	if n == nil {
		return nil
	}
	if n.isGroup() {
		return toBuilderCond(n.children)
	}
	if n.cond == nil {
		return nil
	}
	return conditionToCond(*n.cond)
}

func conditionToCond(c Condition) builder.Cond {
	// 条件列名统一加反引号，避免 key/order/value 等 MySQL 保留字踩坑
	field := quoteIdent(c.Field)
	switch c.Op {
	case OpEq:
		return builder.Eq{field: c.Value}
	case OpNe:
		return builder.Neq{field: c.Value}
	case OpGt:
		return builder.Gt{field: c.Value}
	case OpLt:
		return builder.Lt{field: c.Value}
	case OpGe:
		return builder.Gte{field: c.Value}
	case OpLe:
		return builder.Lte{field: c.Value}
	case OpLike:
		return builder.Like{field, fmt.Sprintf("%%%v%%", c.Value)}
	case OpIn:
		return builder.In(field, toAnySlice(c.Value)...)
	case OpNotIn:
		return builder.NotIn(field, toAnySlice(c.Value)...)
	default:
		return nil
	}
}

// quoteIdent 为 MySQL 标识符加反引号（保留字、特殊名安全）。
// 已加引号的原样返回；支持 table.column 分段转义。
func quoteIdent(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "*" {
		return name
	}
	if strings.HasPrefix(name, "`") && strings.HasSuffix(name, "`") && strings.Count(name, "`") == 2 {
		return name
	}
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		for i, p := range parts {
			parts[i] = quoteIdent(p)
		}
		return strings.Join(parts, ".")
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// quoteOrderBy 转义 ORDER BY 表达式中的列名，例如 "key ASC, id DESC" → "`key` ASC, `id` DESC"。
func quoteOrderBy(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return expr
	}
	chunks := strings.Split(expr, ",")
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		fields := strings.Fields(chunk)
		if len(fields) == 0 {
			continue
		}
		fields[0] = quoteIdent(fields[0])
		out = append(out, strings.Join(fields, " "))
	}
	return strings.Join(out, ", ")
}

func toAnySlice(value any) []any {
	if value == nil {
		return nil
	}
	if slice, ok := value.([]any); ok {
		return slice
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return []any{value}
	}
	out := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		out[i] = v.Index(i).Interface()
	}
	return out
}

// buildFiltersFromModel 从结构体非零字段生成查询条件。
// 列名与操作符来自 xorm 标签：`xorm:"'username' op=like"`。
func buildFiltersFromModel(model any) []Condition {
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	var filters []Condition
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)
		if !value.IsValid() || !value.CanInterface() {
			continue
		}
		if value.Kind() == reflect.Ptr && value.IsNil() {
			continue
		}
		if value.IsZero() {
			continue
		}
		// 嵌套结构体（含 time.Time）不自动进过滤条件
		if value.Kind() == reflect.Struct {
			continue
		}

		tag := field.Tag.Get("xorm")
		dbField, op := parseFilterTag(tag)
		if dbField == "" {
			continue
		}
		filters = append(filters, Condition{
			Field: dbField,
			Op:    op,
			Value: value.Interface(),
		})
	}
	return filters
}

func parseFilterTag(tag string) (string, Op) {
	if tag == "" {
		return "", OpEq
	}
	fieldName := quotedName(tag)
	if fieldName == "" {
		for _, part := range strings.Fields(tag) {
			if strings.HasPrefix(part, "op=") || isXormTypeToken(part) {
				continue
			}
			switch strings.ToLower(part) {
			case "pk", "autoincr", "notnull", "unique", "index", "created", "updated", "deleted", "unsigned":
				continue
			}
			fieldName = part
			break
		}
	}

	op := OpEq
	if idx := strings.Index(tag, "op="); idx >= 0 {
		start := idx + 3
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

// QueryBuilder 链式查询构建器。
//
// 默认条件之间是 AND。调用 Or() 后，下一条条件以 OR 连接；
// And() 显式切回 AND。需要括号时用 AndGroup / OrGroup：
//
//	// WHERE status = 1 AND (role = 'admin' OR role = 'owner')
//	qb.Eq("status", 1).AndGroup(func(g *QueryBuilder[T]) {
//	    g.Eq("role", "admin").Or().Eq("role", "owner")
//	})
type QueryBuilder[T any] struct {
	db *DB

	nodes     []*exprNode
	nextJoin  joinKind
	orderBy   string
	limit     int
	offset    int
	forUpdate bool
}

// NewQueryBuilder 创建链式查询构建器
func NewQueryBuilder[T any](db *DB) *QueryBuilder[T] {
	return &QueryBuilder[T]{db: mustDB(db), nextJoin: joinAnd}
}

// And 下一条条件以 AND 连接（默认行为，可省略）。
func (qb *QueryBuilder[T]) And() *QueryBuilder[T] {
	qb.nextJoin = joinAnd
	return qb
}

// Or 下一条条件以 OR 连接。
func (qb *QueryBuilder[T]) Or() *QueryBuilder[T] {
	qb.nextJoin = joinOr
	return qb
}

// AndGroup 添加一个以 AND 连接到前序条件的括号分组。
func (qb *QueryBuilder[T]) AndGroup(fn func(*QueryBuilder[T])) *QueryBuilder[T] {
	return qb.group(joinAnd, fn)
}

// OrGroup 添加一个以 OR 连接到前序条件的括号分组。
func (qb *QueryBuilder[T]) OrGroup(fn func(*QueryBuilder[T])) *QueryBuilder[T] {
	return qb.group(joinOr, fn)
}

func (qb *QueryBuilder[T]) group(join joinKind, fn func(*QueryBuilder[T])) *QueryBuilder[T] {
	child := &QueryBuilder[T]{db: qb.db, nextJoin: joinAnd}
	fn(child)
	if len(child.nodes) == 0 {
		return qb
	}
	qb.nodes = append(qb.nodes, &exprNode{join: join, children: child.nodes})
	qb.nextJoin = joinAnd
	return qb
}

func (qb *QueryBuilder[T]) where(field string, op Op, value any) *QueryBuilder[T] {
	qb.nodes = append(qb.nodes, &exprNode{
		join: qb.nextJoin,
		cond: &Condition{Field: field, Op: op, Value: value},
	})
	qb.nextJoin = joinAnd
	return qb
}

func (qb *QueryBuilder[T]) applyFilters(filters []Condition) *QueryBuilder[T] {
	for _, f := range filters {
		qb.where(f.Field, f.Op, f.Value)
	}
	return qb
}

// Eq 等于
func (qb *QueryBuilder[T]) Eq(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpEq, value)
}

// Ne 不等于
func (qb *QueryBuilder[T]) Ne(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpNe, value)
}

// Gt 大于
func (qb *QueryBuilder[T]) Gt(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpGt, value)
}

// Lt 小于
func (qb *QueryBuilder[T]) Lt(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLt, value)
}

// Gte 大于等于
func (qb *QueryBuilder[T]) Gte(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpGe, value)
}

// Lte 小于等于
func (qb *QueryBuilder[T]) Lte(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLe, value)
}

// Like 模糊匹配（前后 %）
func (qb *QueryBuilder[T]) Like(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpLike, value)
}

// In IN
func (qb *QueryBuilder[T]) In(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpIn, value)
}

// NotIn NOT IN
func (qb *QueryBuilder[T]) NotIn(field string, value any) *QueryBuilder[T] {
	return qb.where(field, OpNotIn, value)
}

// OrderBy 排序，例如 "id DESC"；列名自动加反引号以兼容保留字
func (qb *QueryBuilder[T]) OrderBy(fields string) *QueryBuilder[T] {
	qb.orderBy = quoteOrderBy(fields)
	return qb
}

// Limit 限制条数
func (qb *QueryBuilder[T]) Limit(limit int) *QueryBuilder[T] {
	qb.limit = limit
	return qb
}

// Offset 偏移
func (qb *QueryBuilder[T]) Offset(offset int) *QueryBuilder[T] {
	qb.offset = offset
	return qb
}

// ForUpdate 行锁 SELECT ... FOR UPDATE（须在事务内使用才有意义）
func (qb *QueryBuilder[T]) ForUpdate() *QueryBuilder[T] {
	qb.forUpdate = true
	return qb
}

// Find 执行查询并返回列表（只跑 SELECT，不附带 COUNT）
func (qb *QueryBuilder[T]) Find(ctx context.Context) ([]T, error) {
	var rows []T
	err := qb.db.withSession(ctx, func(session *xorm.Session) error {
		qb.applySelect(session)
		return session.Find(&rows)
	})
	return rows, wrapDBErr("Find", err)
}

// First 返回第一条记录；无结果返回 ErrNotFound
func (qb *QueryBuilder[T]) First(ctx context.Context) (*T, error) {
	qb.limit = 1
	rows, err := qb.Find(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return &rows[0], nil
}

// Count 返回匹配条件的记录数（只跑 COUNT）
func (qb *QueryBuilder[T]) Count(ctx context.Context) (int64, error) {
	var total int64
	err := qb.db.withSession(ctx, func(session *xorm.Session) error {
		qb.applyWhere(session)
		n, err := session.Count(new(T))
		total = n
		return err
	})
	return total, wrapDBErr("Count", err)
}

// Delete 按当前条件删除
func (qb *QueryBuilder[T]) Delete(ctx context.Context) error {
	err := qb.db.withSession(ctx, func(session *xorm.Session) error {
		qb.applyWhere(session)
		_, err := session.Delete(new(T))
		return err
	})
	return wrapDBErr("Delete", err)
}

func (qb *QueryBuilder[T]) applyWhere(session *xorm.Session) {
	if len(qb.nodes) == 0 {
		return
	}
	if cond := toBuilderCond(qb.nodes); cond != nil {
		session.Where(cond)
	}
}

func (qb *QueryBuilder[T]) applySelect(session *xorm.Session) {
	qb.applyWhere(session)
	if qb.orderBy != "" {
		session.OrderBy(qb.orderBy)
	}
	if qb.limit > 0 {
		session.Limit(qb.limit, qb.offset)
	} else if qb.offset > 0 {
		session.Limit(-1, qb.offset)
	}
	if qb.forUpdate {
		session.ForUpdate()
	}
}
