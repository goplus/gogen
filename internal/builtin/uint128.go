package builtin

const (
	Uint128_Max       = 1<<128 - 1
	Uint128_Min       = 0
	Uint128_IsUntyped = true
)

type Uint128 struct {
	hi uint64
	lo uint64
}

// Uint128_Init: func uint128.init(v int) uint128
func Uint128_Init__0(v int) (out Uint128) {
	return
}

// Uint128_Init: func bigint.init(v untyped_bigint) bigint
func Uint128_Init__1(v Gop_untyped_bigint) (out Uint128) {
	return
}

// Uint128_Cast: func uint128(v untyped_int) uint128
func Uint128_Cast__0(v int) (out Uint128) {
	return
}

// Uint128_Cast: func uint128(v untyped_bigint) uint128
func Uint128_Cast__1(v Gop_untyped_bigint) (out Uint128) {
	return
}
