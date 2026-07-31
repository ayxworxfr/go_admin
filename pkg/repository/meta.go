package repository

import (
	"reflect"
	"strings"
	"sync"
)

// modelMeta 缓存单个模型类型的主键元信息，避免每次反射扫描。
type modelMeta struct {
	pkFieldIndex int
	pkColumn     string
}

var metaCache sync.Map // reflect.Type -> *modelMeta

func metaOf[T any]() (*modelMeta, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() != reflect.Struct {
		return nil, ErrInvalidModel
	}
	if cached, ok := metaCache.Load(t); ok {
		return cached.(*modelMeta), nil
	}
	meta, err := parseModelMeta(t)
	if err != nil {
		return nil, err
	}
	actual, _ := metaCache.LoadOrStore(t, meta)
	return actual.(*modelMeta), nil
}

func parseModelMeta(t reflect.Type) (*modelMeta, error) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xorm")
		if tag == "" || !containsToken(tag, "pk") {
			continue
		}
		col := columnFromXormTag(tag)
		if col == "" {
			col = strings.ToLower(field.Name)
		}
		return &modelMeta{pkFieldIndex: i, pkColumn: col}, nil
	}
	return nil, ErrNoPrimaryKey
}

// columnFromXormTag 从 xorm 标签中提取列名：优先 'col' / `col`，否则取第一个非关键字 token。
func columnFromXormTag(tag string) string {
	if name := quotedName(tag); name != "" {
		return name
	}
	for _, part := range strings.Fields(tag) {
		switch strings.ToLower(part) {
		case "pk", "autoincr", "notnull", "unique", "index", "created", "updated", "deleted":
			continue
		}
		if strings.HasPrefix(part, "op=") {
			continue
		}
		if strings.Contains(part, "(") || isXormTypeToken(part) {
			continue
		}
		return part
	}
	return ""
}

func quotedName(tag string) string {
	inQuote := false
	quoteChar := rune(0)
	start := -1
	for i, r := range tag {
		if r == '\'' || r == '`' {
			if !inQuote {
				inQuote = true
				quoteChar = r
				start = i
				continue
			}
			if r == quoteChar {
				return tag[start+1 : i]
			}
		}
	}
	return ""
}

func containsToken(tag, token string) bool {
	for _, part := range strings.Fields(tag) {
		if strings.EqualFold(part, token) {
			return true
		}
	}
	return false
}

func isXormTypeToken(part string) bool {
	lower := strings.ToLower(part)
	for _, prefix := range []string{
		"varchar", "char", "text", "tinyint", "smallint", "int", "bigint",
		"float", "double", "decimal", "datetime", "date", "time", "timestamp",
		"blob", "json", "bool", "boolean", "unsigned",
	} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"(") {
			return true
		}
	}
	return false
}

func setPrimaryKey(model any, id any) error {
	v := reflect.ValueOf(model)
	if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return ErrInvalidModel
	}
	meta, err := parseModelMeta(v.Elem().Type())
	if err != nil {
		return err
	}
	field := v.Elem().Field(meta.pkFieldIndex)
	if !field.CanSet() {
		return ErrInvalidModel
	}
	idVal := reflect.ValueOf(id)
	if !idVal.Type().AssignableTo(field.Type()) {
		if idVal.Type().ConvertibleTo(field.Type()) {
			idVal = idVal.Convert(field.Type())
		} else {
			return ErrInvalidModel
		}
	}
	field.Set(idVal)
	return nil
}

func primaryKeyValue(model any) (any, error) {
	v := reflect.ValueOf(model)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil, ErrInvalidModel
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil, ErrInvalidModel
	}
	meta, err := parseModelMeta(v.Type())
	if err != nil {
		return nil, err
	}
	field := v.Field(meta.pkFieldIndex)
	if !field.IsValid() || (field.Kind() == reflect.Ptr && field.IsNil()) {
		return nil, ErrNoPrimaryKey
	}
	return field.Interface(), nil
}
