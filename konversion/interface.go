package konversion

import (
	"reflect"
)

// Get value from interface. Return default value if key not exists
func ReadInterface(src map[string]interface{}, key string, defaultValue interface{}) interface{} {
	value, ok := src[key]
	if !ok {
		return defaultValue
	}
	return value
}

// DoubleSlice helps deal with variadic functions in Go.
func DoubleSlice(s interface{}) []interface{} {
	v := reflect.ValueOf(s)
	items := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		items[i] = v.Index(i).Interface()
	}
	return items
}
