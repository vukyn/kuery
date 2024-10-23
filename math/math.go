package math

import "github.com/vukyn/kuery/t"

// Abs returns the absolute value of a number.
//
// Example:
//
//	result := Abs(-5) // result will be 5
//
// Parameters:
//   - x: The number for which the absolute value is calculated.
//
// Returns:
//   - The absolute value of x.
func Abs[T t.Number](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

// Min finds the minimum value among the input values.
//
// Example:
//
//	result := Min(3, 7, 1, 9) // result will be 1 (minimum value among 3, 7, 1, 9)
//
// Parameters:
//   - vals: Variadic input values of type T to find the minimum from.
//
// Returns:
//   - The minimum value among the input values.
func Min[T t.Number](vals ...T) T {
	min := vals[0]
	for _, val := range vals {
		if val < min {
			min = val
		}
	}
	return min
}

// Max finds the maximum value among the input values.
//
// Example:
//
//	result := Max(3, 7, 1, 9) // result will be 1 (maximum value among 3, 7, 1, 9)
//
// Parameters:
//   - vals: Variadic input values of type T to find the maximum from.
//
// Returns:
//   - The maximum value among the input values.
func Max[T t.Number](vals ...T) T {
	max := vals[0]
	for _, val := range vals {
		if max < val {
			max = val
		}
	}
	return max
}

// Calculate the sum of multiple values.
//
// Example:
//
//	result := Sum(2, 5, 3) // result will be 10 (2 + 5 + 3)
//
// Parameters:
//   - vals: The values to be summed up.
//
// Returns:
//   - The sum of all input values.
func Sum[T t.Number](vals ...T) T {
	sum := vals[0]
	for _, val := range vals[1:] {
		sum += val
	}
	return sum
}

// Product calculates the product of multiple numbers.
//
// Example:
//
//	result := Product(2, 3, 4) // result will be 24 (2 * 3 * 4)
//
// Parameters:
//   - vals: The numbers to be multiplied.
//
// Returns:
//   - The product of all the input numbers.
func Product[T t.Number](vals ...T) T {
	prod := vals[0]
	for _, val := range vals[1:] {
		prod *= val
	}
	return prod
}
