package conversion

import (
	"reflect"
)

// Get value from interface. Return default value if key not exists or empty value.
func ReadInterface(src map[string]interface{}, key string, defaultValue interface{}) interface{} {
	value, ok := src[key]
	if !ok || IsZero(value) {
		return defaultValue
	}
	return value
}

// Get value from interface. Return default value if key not exists or empty value (optional).
func GetFromInterfaceV2[T any](src map[string]interface{}, key string, defaultValue T, isZero ...bool) T {
	value, exists := src[key]
	if !exists || (IsZero(value) && len(isZero) > 0 && isZero[0]) {
		return defaultValue
	}
	return value.(T)
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
	if v.Kind() == reflect.Slice {
		return v.Len() == 0
	}
	return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}