package httptools

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
)

// Struct2URLValues 将结构体序列化为 x-www-form-urlencooded 格式 tag 为 form
func Struct2URLValues[T any](data T) (url.Values, error) {
	values := url.Values{}
	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)

	// 确保传入的数据是结构体类型
	if v.Kind() != reflect.Struct {
		return values, fmt.Errorf("%v is not a struct", data)
	}

	// 遍历结构体的字段
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 获取字段名称和值
		name := field.Tag.Get("form")
		value := fieldValue.Interface()

		// 如果字段没有设置url tag，则跳过
		if name == "" {
			continue
		}

		// 格式化值为字符串
		var strValue string
		switch fieldValue.Kind() {
		case reflect.String:
			strValue = value.(string)
		case reflect.Int:
			strValue = strconv.FormatInt(int64(value.(int)), 10)
		case reflect.Int8:
			strValue = strconv.FormatInt(int64(value.(int8)), 10)
		case reflect.Int16:
			strValue = strconv.FormatInt(int64(value.(int16)), 10)
		case reflect.Int32:
			strValue = strconv.FormatInt(int64(value.(int32)), 10)
		case reflect.Int64:
			strValue = strconv.FormatInt(value.(int64), 10)
		case reflect.Uint:
			strValue = strconv.FormatUint(uint64(value.(uint)), 10)
		case reflect.Uint8:
			strValue = strconv.FormatUint(uint64(value.(uint8)), 10)
		case reflect.Uint16:
			strValue = strconv.FormatUint(uint64(value.(uint16)), 10)
		case reflect.Uint32:
			strValue = strconv.FormatUint(uint64(value.(uint32)), 10)
		case reflect.Uint64:
			strValue = strconv.FormatUint(value.(uint64), 10)
		case reflect.Float32:
			strValue = strconv.FormatFloat(float64(value.(float32)), 'f', -1, 64)
		case reflect.Float64:
			strValue = strconv.FormatFloat(value.(float64), 'f', -1, 64)
		case reflect.Bool:
			strValue = strconv.FormatBool(value.(bool))
		default:
			strValue = fmt.Sprintf("%v", value)
		}
		// 添加到url.Values对象中
		values.Add(name, strValue)
	}
	return values, nil
}

var HttpHyperTransport = &http.Transport{
	DisableKeepAlives:   false,
	MaxConnsPerHost:     0,
	MaxIdleConns:        3000,
	MaxIdleConnsPerHost: 300,
}
