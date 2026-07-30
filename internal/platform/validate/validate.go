package validator

import (
	"io"
	"reflect"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/server/binding"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

type DecimalBinder struct{}

func NewDecimalBinder() *DecimalBinder {
	return &DecimalBinder{}
}

func (d *DecimalBinder) Name() string {
	return "decimal"
}

func (d *DecimalBinder) Bind(req *protocol.Request, v any, params param.Params) error {
	return d.BindAndValidate(req, v, params)
}

func (d *DecimalBinder) BindAndValidate(req *protocol.Request, v interface{}, params param.Params) error {
	if err := binding.DefaultBinder().BindAndValidate(req, v, params); err != nil {
		return err
	}
	contentType := string(req.Header.ContentType())
	switch {
	case strings.Contains(contentType, consts.MIMEApplicationJSON):
		return d.BindJSON(req, v)
	case strings.Contains(contentType, consts.MIMEApplicationHTMLForm),
		strings.Contains(contentType, consts.MIMEMultipartPOSTForm):
		return d.BindForm(req, v)
	default:
		// 使用默认校验器
		return nil
	}
}

func (d *DecimalBinder) BindQuery(req *protocol.Request, v any) error {
	if err := binding.DefaultBinder().BindQuery(req, v); err != nil {
		return err
	}
	return d.handleDecimalFields(v)
}

func (d *DecimalBinder) BindHeader(req *protocol.Request, v any) error {
	if err := binding.DefaultBinder().BindHeader(req, v); err != nil {
		return err
	}
	return d.handleDecimalFields(v)
}

func (d *DecimalBinder) BindPath(req *protocol.Request, v any, params param.Params) error {
	if err := binding.DefaultBinder().BindPath(req, v, params); err != nil {
		return err
	}
	return d.handleDecimalFields(v)
}

func (d *DecimalBinder) BindForm(req *protocol.Request, v any) error {
	if err := binding.DefaultBinder().BindForm(req, v); err != nil {
		return err
	}
	return d.handleDecimalFields(v)
}

func (d *DecimalBinder) BindJSON(req *protocol.Request, v any) error {
	if err := binding.DefaultBinder().BindJSON(req, v); err != nil {
		// 仅当错误是"空请求体"时忽略（允许空JSON），其他错误正常返回
		if err != io.EOF {
			return err
		}
	}
	return d.handleDecimalFields(v)
}

func (d *DecimalBinder) BindProtobuf(req *protocol.Request, v any) error {
	return binding.DefaultBinder().BindProtobuf(req, v)
}

func (d *DecimalBinder) handleDecimalFields(v any) error {
	value := reflect.ValueOf(v)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := field.Type()
		structField := value.Type().Field(i)

		// 检查字段是否可导出
		if !field.CanInterface() {
			continue // 跳过未导出的字段
		}

		switch {
		case fieldType == reflect.TypeOf(decimal.Decimal{}):
			// 直接的 decimal.DecimalBinder 类型
			if field.CanSet() && !field.IsZero() {
				if err := d.validateDecimalField(field.Interface().(decimal.Decimal), structField); err != nil {
					return err
				}
			}
		case fieldType == reflect.TypeOf((*decimal.Decimal)(nil)):
			// 指针类型的 decimal.DecimalBinder
			if field.CanSet() && !field.IsNil() {
				decimalPtr := field.Interface().(*decimal.Decimal)
				if err := d.validateDecimalField(*decimalPtr, structField); err != nil {
					return err
				}
				field.Set(reflect.ValueOf(decimalPtr))
			}
		case field.Kind() == reflect.Struct:
			// 递归处理嵌套结构
			if field.CanAddr() {
				if err := d.handleDecimalFields(field.Addr().Interface()); err != nil {
					return err
				}
			}
		case field.Kind() == reflect.Slice:
			// 处理切片中的每个元素
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if elem.Kind() == reflect.Struct && elem.CanAddr() {
					if err := d.handleDecimalFields(elem.Addr().Interface()); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (d *DecimalBinder) validateDecimalField(field decimal.Decimal, structField reflect.StructField) error {
	tag := structField.Tag.Get("vd")
	if tag == "" {
		return nil // 如果没有验证标签，则跳过验证
	}

	parts := strings.Split(tag, "&&")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		op, valStr := extractOperatorAndValue(part)
		if op == "" {
			return errors.Errorf("unsupported validation rule for field %s: %s", structField.Name, part)
		}

		val, err := decimal.NewFromString(valStr)
		if err != nil {
			return errors.Wrapf(err, "invalid validation tag for field %s", structField.Name)
		}

		if !compareDecimal(field, val, op) {
			return errors.Errorf("field %s must be %s %s", structField.Name, getOperatorDescription(op), valStr)
		}
	}

	return nil
}

func extractOperatorAndValue(part string) (string, string) {
	operators := []string{"$>=", "$<=", "$>", "$<", "$="}
	for _, op := range operators {
		if strings.HasPrefix(part, op) {
			return op, strings.TrimPrefix(part, op)
		}
	}
	return "", ""
}

func compareDecimal(field, val decimal.Decimal, op string) bool {
	switch op {
	case "$>=":
		return field.GreaterThanOrEqual(val)
	case "$<=":
		return field.LessThanOrEqual(val)
	case "$>":
		return field.GreaterThan(val)
	case "$<":
		return field.LessThan(val)
	case "$=":
		return field.Equal(val)
	}
	return false
}

func getOperatorDescription(op string) string {
	switch op {
	case "$>=":
		return "greater than or equal to"
	case "$<=":
		return "less than or equal to"
	case "$>":
		return "greater than"
	case "$<":
		return "less than"
	case "$=":
		return "equal to"
	}
	return ""
}
