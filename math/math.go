package math

import "github.com/vukyn/kuery/t"

func Abs[T t.Number](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

func Min[T t.Number](vals ...T) T {
	min := vals[0]
	for _, val := range vals {
		if val < min {
			min = val
		}
	}
	return min
}

func Max[T t.Number](vals ...T) T {
	max := vals[0]
	for _, val := range vals {
		if max < val {
			max = val
		}
	}
	return max
}

func Sum[T t.Number](vals ...T) T {
	sum := vals[0]
	for _, val := range vals[1:] {
		sum += val
	}
	return sum
}

func Product[T t.Number](vals ...T) T {
	prod := vals[0]
	for _, val := range vals[1:] {
		prod *= val
	}
	return prod
}
