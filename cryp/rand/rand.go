package rand

import (
	"crypto/rand"
	"fmt"
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

// RandMixedString generates a random string of length n with mixed characters
//
// Example:
//
//	RandMixedString(10, true, true) => "A1b2C3d!@"
func RandMixedString(n int, hasNumber bool, hasSpecial bool) string {
	charset := t.LowerLetters + t.UpperLetters
	if hasNumber {
		charset += t.Numbers
	}
	if hasSpecial {
		charset += t.SpecialChars
	}
	b := make([]byte, n)
	for i := range n {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return ""
		}
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

// RandIpV4 generates a random IPv4 address
//
// The generated IP address is guaranteed to be valid.
//
// Example:
//
//	RandIpV4() => "192.168.1.1"
func RandIpV4() string {
	ip := make([]byte, 4)
	rand.Read(ip)
	for i := range ip {
		ip[i] = ip[i] % 255
	}
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}
