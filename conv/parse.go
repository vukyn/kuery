package conv

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/vukyn/kuery/t"
)

// NumberToString converts a number to a string.
//
// Example:
//
//	fmt.Println(NumberToString(123)) // Output: "123"
func NumberToString[T t.Number](a T) string {
	return fmt.Sprint(a)
}

// ArrayNumberToString converts an array of numbers to a string.
//
// Example:
//
//	fmt.Println(ArrayNumberToString[int]([]int{1, 2, 3}, ",")) // Output: "1,2,3"
func ArrayNumberToString[T t.Number](a []T, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

// StringToArrayNumber converts a string to an array of numbers.
//
// Example:
//
//	fmt.Println(StringToArrayNumber[int]("1,2,3", ",")) // Output: [1 2 3]
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

// ArrayStringToString converts an array of strings to a string.
//
// Example:
//
//	fmt.Println(ArrayStringToString([]string{"a", "b", "c"}, ",")) // Output: "a,b,c"
func ArrayStringToString(a []string, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}

// StringToArrayString converts a string to an array of strings.
//
// Example:
//
//	fmt.Println(StringToArrayString("a,b,c", ",", false)) // Output: [a b c]
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

// ToPointer converts a value to a pointer.
//
// Example:
//
//	fmt.Println(ToPointer(123)) // Output: 0xc0000b6018
func ToPointer[T any](i T) *T {
	return &i
}
