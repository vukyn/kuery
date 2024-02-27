package query

// Index returns index of first element that satisfies v in l.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		index := query.Index(list, 1)
func Index[L ~[]T, T comparable](l L, v T) int {
	for i := range l {
		if l[i] == v {
			return i
		}
	}
	return -1
}

// IndexFunc returns index of first element that satisfies the provided f.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		index := query.IndexFunc(list, func(n name) bool {return n == "ABC"})
func IndexFunc[L ~[]T, T any](l L, f func(T) bool) int {
	for i := range l {
		if f(l[i]) {
			return i
		}
	}
	return -1
}

// Any checks if any element in the slice satisfies v in l.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		any := query.Any(list, "ABC")
func Any[L ~[]T, T comparable](l L, v T) bool {
	return Index(l, v) >= 0
}

// AnyFunc checks if any element in the slice satisfies the provided f.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		any := query.AnyFunc(list, func(n name) bool {return n == "ABC"})
func AnyFunc[L ~[]T, T any](l L, f func(T) bool) bool {
	return IndexFunc(l, f) >= 0
}

// Every checks if every element in the slice satisfies v in l.
//
// Complexity is O(n) for time and space.
//
// Example:
//		every := query.Every(list, "ABC")
func Every[L ~[]T, T comparable](l L, v T) bool {
	return len(Where(l, v) ) == len(l)
}

// EveryFunc checks if every element in the slice satisfies the provided f.
//
// Complexity is O(n) for time and space.
//
// Example:
//		every := query.EveryFunc(list, func(n name) bool {return n == "ABC"})
func EveryFunc[L ~[]T, T any](l L, f func(T) bool) bool {
	return len(WhereFunc(l, f) ) == len(l)
}

// Find returns first element that satisfies v in l.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		item := Find(list, 1)
func Find[L ~[]T, T comparable](l L, v T) (T, bool) {
	index := Index(l, v)
	if index >= 0 {
		return l[index], true
	}
	var empty T
	return empty, false
}

// FindFunc returns first element that satisfies the provided f.
//
// Complexity is O(n) for time and O(1) for space.
//
// Example:
//		item := FindFunc(list, func(n name) bool {return n == "ABC"})
func FindFunc[L ~[]T, T comparable](l L, f func(T) bool) (T, bool) {
	index := IndexFunc(l, f)
	if index >= 0 {
		return l[index], true
	}
	var empty T
	return empty, false
}

// Where returns the new slice that satisfies v in l.
//
// Complexity is O(n) for time and space.
//
// Example:
//		items := query.Where(list, 1)
func Where[L ~[]T, T comparable](l L, v T) L {
	var res L
	for i := range l {
		if l[i] == v {
			res = append(res, l[i])
		}
	}
	return res
}

// WhereFunc returns the new slice that satisfies the provided f.
//
// Complexity is O(n) for time and space.
//
// Example:
//		items := query.WhereFunc(list, func(i int) bool {return i == 1})
func WhereFunc[L ~[]T, T any](l L, f func(T) bool) L {
	var res L
	for i := range l {
		if f(l[i]) {
			res = append(res, l[i])
		}
	}
	return res
}

// Distinct returns the new slice that removed duplicate elements.
//
// Complexity is O(n) for time and space.
//
//	 Example:
//		list1 := query.Distinct(list)
func Distinct[L ~[]T, M map[T]bool, T comparable](l L) L {
	var res L
	keys := make(M)
	for i := range l {
		if _, value := keys[l[i]]; !value {
			keys[l[i]] = true
			res = append(res, l[i])
		}
	}
	return res
}

// Pop returns the modified slice that removed last element.
//
//	 Example:
//		x, a := query.Pop(items)
func Pop[L ~[]T, T any](l L) (T, L) {
	if len(l) == 0 {
		panic("empty slice")
	}
	return l[len(l)-1], l[:len(l)-1]
}

// Shift returns the modified slice that removed first element.
//
//	 Example:
//		x, a := query.Shift(items)
func Shift[L ~[]T, T any](l L) (T, L) {
	if len(l) == 0 {
		panic("empty slice")
	}
	return l[0], l[1:]
}

// Unshift returns the modified slice that added the new element to the beginning.
//
//	 Example:
//		items := query.Unshift(items, a)
func Unshift[L ~[]T, T any](l L, x T) L {
	if len(l) == 0 {
		return append(l, x)
	}
	return append([]T{x}, l...)
}

// RemoveAt return the modified slice that removed the element based on index.
//
//	 Example:
//		x, a := query.RemoveAt(items, 1)
func RemoveAt[L ~[]T, T any](l L, i int) (T, L) {
	n := len(l)
	if i < 0 || i >= n {
		panic("index out of range")
	}
	// remove last element
	if i == n-1 {
		return Pop(l)
	}
	// remove first element
	if i == 0 {
		return Shift(l)
	}
	return l[i], append(l[:i], l[i+1:]...)
}

// Map creates a new array populated with the results of
// calling a provided function on every element in the array.
//
// Complexity is O(n) for time and space.
//
//	 Example:
//		items := query.Map([]int{1,2,3}, func(n int) int {return n*2})
func Map[L ~[]T, R []U, T any, U any](l L, f func(T) U) R {
	var res R
	for i := range l {
		res = append(res, f(l[i]))
	}
	return res
}

// Slice create new slice from map(value) with condition (optional).
//
// Complexity is O(n) for time and space.
//
//	 Example:
//		items := query.Slice(map[int32]string{1:"1",2:"2"}, func(n string) bool {return true})
func Slice[M ~map[T]U, R []U, T comparable, U any](m M, f ...func(U) bool) R {
	var res R
	for _, v := range m {
		if f != nil {
			if f[0](v) {
				res = append(res, v)
			}
		} else {
			res = append(res, v)
		}
	}
	return res
}

// Keys create new slice from map(key) with condition (optional).
//
// Complexity is O(n) for time and space.
//
//	 Example:
//		items := query.Keys(map[int32]string{1:"1",2:"2"}, func(n string) bool {return true})
func Keys[M ~map[T]U, R []T, T comparable, U any](m M, f ...func(U) bool) R {
	var res R
	for k, v := range m {
		if f != nil {
			if f[0](v) {
				res = append(res, k)
			}
		} else {
			res = append(res, k)
		}
	}
	return res
}

// Mapper create new map from slice with key.
//
// Complexity is O(n) for time and space.
//
//	 Example:
//		items := query.Mapper([]int{1,2,3}, func(n int) int {return n})
func Mapper[L ~[]T, M map[U]T, T any, U comparable](l L, key func(T) U) M {
	res := make(M)
	for i := range l {
		res[key(l[i])] = l[i]
	}
	return res
}
