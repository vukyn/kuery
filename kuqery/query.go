package kuery

// Where function filter elements based on conditions.
// Where return a new slice.
//
//	 Example:
//		evens := Filter(list, func(i int) bool {return i % 2 == 0})
func Where[T any](list []T, f func(T) bool) []T {
	var newList []T
	for _, v := range list {
		if f(v) {
			newList = append(newList, v)
		}
	}
	return newList
}

// Find function find first element based on conditions.
// Find return a new value T.
//
//	 Example:
//		item := Find(list, func(n name) bool {return n == "ABC"})
func Find[T any](list []T, f func(T) bool) T {
	var newValue T
	for _, v := range list {
		if f(v) {
			newValue = v
			break
		}
	}
	return newValue
}

// Distinct function remove duplicates from slice.
// Distinct return a new slice.
//
//	 Example:
//		newList := Distinct(oldList)
func Distinct[Y string | int32 | int64 | float32 | float64](list []Y) []Y {
	allKeys := make(map[Y]bool)
	newList := []Y{}
	for _, item := range list {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			newList = append(newList, item)
		}
	}
	return newList
}

// Pop removes the last element.
// Pop returns new slice and that element.
//
//	 Example:
//		x, a := Pop(items)
func Pop[T any](list []T) (T, []T) {
	return list[len(list)-1], list[:len(list)-1]
}

// Shift removes the first element.
// Shift returns new slice and that element.
//
//	 Example:
//		x, a := Shift(items)
func Shift[T any](list []T) (T, []T) {
	return list[0], list[1:]
}

// Unshift adds the specified elements to the beginning.
// Unshift returns new slice.
//
//	 Example:
//		items := Unshift(items, a)
func Unshift[T any](list []T, x T) []T {
	list = append([]T{x}, list...)
	return list
}

// RemoveAt removes the element based on index.
// RemoveAt returns new slice.
//
//	 Example:
//		a := RemoveAt(items, 1)
func RemoveAt[T any](list []T, i int) []T {
	list[i] = list[len(list)-1]
	return list[:len(list)-1]
}

// IndexOf returns the index of the
// first element that satisfies the condition.
//
//	 Example:
//		index := IndexOf(list, func(n name) bool {return n == "ABC"})
func IndexOf[T any](list []T, f func(T) bool) int {
	index := -1
	for i, v := range list {
		if f(v) {
			index = i
			break
		}
	}
	return index
}