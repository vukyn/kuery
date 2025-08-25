package cryp

import (
	"crypto/rand"
	"math/big"

	"github.com/vukyn/kuery/t"
)

// RandInt generates a random number in range [min, max]
//
// Example:
//
//	RandInt(1, 10) => 5
func RandInt[T t.INT](min, max T) (T, error) {
	if min == max {
		return min, nil
	}
	bigMax := big.NewInt(int64(max - min + 1))
	n, err := rand.Int(rand.Reader, bigMax)
	if err != nil {
		return 0, err
	}
	return T(n.Int64()) + min, nil
}

// RandBool returns a random true/false
//
// Example:
//
//	RandBool() => true
func RandBool() bool {
	b := make([]byte, 1)
	rand.Read(b)
	return b[0]%2 == 0
}

// RandString generates a random string of length n
//
// Example:
//
//	RandString(10) => "abcdefghij"
func RandString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return string(b)
}
