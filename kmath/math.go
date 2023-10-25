package kmath

func Abs[T int | int32 | int64 | float32 | float64](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

func Min[T int | int32 | int64 | uint32 | uint64 | float32 | float64](vals ...T) T {
	min := vals[0]
	for _, val := range vals {
		if val < min {
			min = val
		}
	}
	return min
}

func Max[T int | int32 | int64 | uint32 | uint64 | float32 | float64](vals ...T) T {
	max := vals[0]
	for _, val := range vals {
		if max < val {
			max = val
		}
	}
	return max
}
