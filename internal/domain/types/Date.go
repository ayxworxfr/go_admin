package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Date 代表一个只含日期（年/月/日）的类型，是 time.Time 的别名
type Date time.Time

// NewDate 创建一个新的 Date 类型（手动指定年月日）
func NewDate(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// FromTime 将 time.Time 转为 Date（只保留年月日）
func FromTime(t time.Time) Date {
	return Date(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
}

// ToTime 返回 time.Time 值
func (d Date) ToTime() time.Time {
	return time.Time(d)
}

// String 返回日期的字符串表示，格式为 "2006-01-02"
func (d Date) String() string {
	return time.Time(d).Format(time.DateOnly)
}

// MarshalJSON 实现 json.Marshaler 接口，格式为 time.DateOnly
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, time.Time(d).Format(time.DateOnly))), nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// Value 实现 driver.Valuer 接口（用于数据库写入）
func (d Date) Value() (driver.Value, error) {
	return time.Time(d).Format(time.DateOnly), nil
}

// Scan 实现 sql.Scanner 接口（用于数据库读取）
func (d *Date) Scan(value interface{}) error {
	switch v := value.(type) {
	case time.Time:
		*d = Date(v)
		return nil
	case string:
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return err
		}
		*d = Date(t)
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into Date", value)
	}
}

// Year 返回年份
func (d Date) Year() int {
	return time.Time(d).Year()
}

// Month 返回月份
func (d Date) Month() time.Month {
	return time.Time(d).Month()
}

// Day 返回日
func (d Date) Day() int {
	return time.Time(d).Day()
}

// Format 格式化日期
func (d Date) Format(layout string) string {
	return time.Time(d).Format(layout)
}
