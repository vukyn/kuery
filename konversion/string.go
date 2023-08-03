package konversion

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func ToJsonString(src interface{}) (string, error) {
	values := reflect.TypeOf(src)
	if values.Kind() != reflect.Struct {
		return "", fmt.Errorf("src must be a struct")
	}
	out, err := json.Marshal(src)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func NumberToString[T int | int32 | int64 | float32 | float64](a T) string {
	return fmt.Sprint(a)
}

func ArrayNumberToString[T int | int32 | int64 | float32 | float64](a []T, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}