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
			// Unreachable. rand.Int only returns an error when its reader
			// fails, and since Go 1.24 crypto/rand.Reader cannot fail — a
			// failure of the OS entropy source terminates the program instead.
			// Its other failure mode, max <= 0, is impossible here because the
			// charset always contains at least the two letter sets.
			//
			// It panics rather than returning "" because callers use this to
			// mint share tokens, API keys and slugs and none of them check a
			// return value they were told is a string. A silent "" would issue
			// an empty token — a credential that is wrong in the direction of
			// letting someone in — where a panic stops the request. Same
			// reasoning as ulid.MustNew and post-1.24 crypto/rand itself.
			//
			// The signature stays (string) deliberately: adding an error would
			// break every consumer for a branch that cannot be taken.
			panic("kuery/cryp/rand: crypto/rand failed: " + err.Error())
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
