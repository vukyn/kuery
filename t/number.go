package t

type uint interface {
	uint8 | uint16 | uint32 | uint64
} // unsigned integer (8, 16, 32, 64)
type sint interface {
	int8 | int16 | int32 | int64
}                                       // signed integer (8, 16, 32, 64)
type int interface{ uint | sint }       // all integer
type dec interface{ float32 | float64 } // float (32, 64)
type number interface{ int | dec }      // all number

type UINT uint     // unsigned integer (8, 16, 32, 64)
type SINT sint     // signed integer (8, 16, 32, 64)
type INT int       // all integer (8, 16, 32, 64)
type DEC dec       // float (32, 64)
type Number number // all number (8, 16, 32, 64, 32, 64)

const (
	Numbers      = "0123456789"
	LowerLetters = "abcdefghijklmnopqrstuvwxyz"
	UpperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	SpecialChars = "!@#$%^&*()-_=+[]{}|;:,.<>?/"
)
