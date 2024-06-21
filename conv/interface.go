package conv

import (
	"reflect"
)

// Get value from interface. Return default value if key not exists or empty value.
func ReadInterface[T any](src map[string]any, key string, defaultValue T, isZero ...bool) T {
	value, exists := src[key]
	if !exists || (IsZero(value) && len(isZero) > 0 && isZero[0]) {
		return defaultValue
	}
	return value.(T)
}

// Deal with variadic functions in Go.
func DoubleSlice(s any) []any {
	v := reflect.ValueOf(s)
	items := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		items[i] = v.Index(i).Interface()
	}
	return items
}

// Detect whether a value is the zero value for its type.
func IsZero(p any) bool {
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	default:
		return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}
