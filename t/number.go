package t

type uint interface {
	uint8 | uint16 | uint32 | uint64
}
type sint interface {
	int | int8 | int16 | int32 | int64
}
type aint interface{ uint | sint }
type dec interface{ float32 | float64 }
type number interface{ aint | dec }

type UINT uint
type SINT sint
type AINT aint
type DEC dec
type Number number
