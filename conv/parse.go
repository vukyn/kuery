package conv

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/vukyn/kuery/t"
)

func NumberToString[T t.Number](a T) string {
	return fmt.Sprint(a)
}

func ArrayNumberToString[T t.Number](a []T, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

func StringToArrayNumber[T t.Number](str, delim string) []T {
	var t T
	var result []T
	for _, v := range strings.Split(str, delim) {
		if v != "" {
			switch reflect.TypeOf(t).Kind() {
			case reflect.Int:
				i, _ := strconv.Atoi(v)
				result = append(result, T(i))
			case reflect.Int8:
				i, _ := strconv.ParseInt(v, 10, 8)
				result = append(result, T(i))
			case reflect.Int16:
				i, _ := strconv.ParseInt(v, 10, 16)
				result = append(result, T(i))
			case reflect.Int32:
				i, _ := strconv.ParseInt(v, 10, 32)
				result = append(result, T(i))
			case reflect.Int64:
				i, _ := strconv.ParseInt(v, 10, 64)
				result = append(result, T(i))
			case reflect.Uint8:
				i, _ := strconv.ParseUint(v, 10, 8)
				result = append(result, T(i))
			case reflect.Uint16:
				i, _ := strconv.ParseUint(v, 10, 16)
				result = append(result, T(i))
			case reflect.Uint32:
				i, _ := strconv.ParseUint(v, 10, 32)
				result = append(result, T(i))
			case reflect.Uint64:
				i, _ := strconv.ParseUint(v, 10, 64)
				result = append(result, T(i))
			case reflect.Float32:
				i, _ := strconv.ParseFloat(v, 32)
				result = append(result, T(i))
			case reflect.Float64:
				i, _ := strconv.ParseFloat(v, 64)
				result = append(result, T(i))
			}
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

func ToPointer[T any](i T) *T {
	return &i
}
