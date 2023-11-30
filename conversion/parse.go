package conversion

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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

func NumberToString[T int | int32 | int64 | uint32 | uint64 | float32 | float64](a T) string {
	return fmt.Sprint(a)
}

func ArrayNumberToString[T int | int32 | int64 | uint32 | uint64 | float32 | float64](a []T, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func StringToArrayInt(str, delim string) []int {
	var result []int
	for _, v := range strings.Split(str, delim) {
		if v != "" {
			i, _ := strconv.Atoi(v)
			result = append(result, i)
		}
	}
	return result
}

func StringToArrayInt64(str string, delim string) []int64 {
	var result []int64
	for _, v := range strings.Split(str, delim) {
		if v != "" {
			i, _ := strconv.ParseInt(v, 10, 64)
			result = append(result, i)
		}
	}
	return result
}

func ArrayStringToString(a []string, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func StringToArrayString(str, delim string, trim bool) []string {
	var result []string
	for _, v := range strings.Split(str, delim) {
		if trim {
			v = strings.TrimSpace(v)
		}
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
