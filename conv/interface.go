package conv

import (
	"reflect"
)

// IsZero returns true if the value is nil or zero.
// The value can be a pointer, slice, map, or any other type.
// If the value is a pointer, it returns true if the pointer is nil.
// If the value is a slice or map, it returns true if the length is zero.
//
// Example:
//
//	var a *int
//	fmt.Println(conv.IsZero(a)) // true
//	a = new(int)
//	fmt.Println(conv.IsZero(a)) // false
//	b := 0
//	fmt.Println(conv.IsZero(b)) // true
//	c := []int{}
//	fmt.Println(conv.IsZero(c)) // true
//	d := map[string]int{}
//	fmt.Println(conv.IsZero(d)) // true
//	type S struct {
//		A int
//	}
//	e := S{}
//	fmt.Println(conv.IsZero(e)) // true
//	f := S{A: 1}
//	fmt.Println(conv.IsZero(f)) // false
//	g := &S{}
//	fmt.Println(conv.IsZero(g)) // true
//	h := &S{A: 1}
//	fmt.Println(conv.IsZero(h)) // false
//	i := make(chan int)
//	fmt.Println(conv.IsZero(i)) // true
//	j := make(chan int, 0)
//	fmt.Println(conv.IsZero(j)) // true
//	k := make(chan int, 1)
//	fmt.Println(conv.IsZero(k)) // true
//	k <- 1
//	fmt.Println(conv.IsZero(k)) // false
//	<-k
//	fmt.Println(conv.IsZero(k)) // true
func IsZero(p any) bool {
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Chan:
		return v.IsNil() || v.Len() == 0
	default:
		return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}
