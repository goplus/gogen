package builtin

const (
	Int128_Max       = 1<<127 - 1
	Int128_Min       = -1 << 127
	Int128_IsUntyped = true
)

type Int128 struct {
	hi uint64
	lo uint64
}

// Int128_Init: func int128.init(v int) int128
func Int128_Init__0(v int) (out Int128) {
	return
}

// Int128_Init: func int128.init(v untyped_bigint) int128
func Int128_Init__1(v Gop_untyped_bigint) (out Int128) {
	return
}

// Int128_Cast: func int128(v int) int128
func Int128_Cast__0(v int) (out Int128) {
	return
}

// Int128_Cast: func int128(v untyped_bigint) int128
func Int128_Cast__1(v Gop_untyped_bigint) (out Int128) {
	return
}
