package conversion

import (
	"reflect"
)

// Get value from interface. Return default value if key not exists or empty value.
func ReadInterface(src map[string]interface{}, key string, defaultValue interface{}) interface{} {
	value, ok := src[key]
	if !ok {
		return defaultValue
	}
	return value
}

// Deal with variadic functions in Go.
func DoubleSlice(s interface{}) []interface{} {
	v := reflect.ValueOf(s)
	items := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		items[i] = v.Index(i).Interface()
	}
	return items
}

// Detect whether a value is the zero value for its type.
func IsZero(p interface{}) bool {
	v := reflect.ValueOf(p)
	return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}