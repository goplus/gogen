/*
 Copyright 2021 The XGo Authors (xgo.dev)
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package builtin

import (
	"math/big"
)

const (
	XGoPackage = true // to indicate this is a XGo package
)

//
// XGo_: XGo object prefix
// XGo_xxx_Cast: type XGo_xxx typecast
// xxxx__N: the Nth overload function
//

type XGo_ninteger = uint

func XGo_istmp(a interface{}) bool {
	return false
}

// -----------------------------------------------------------------------------

type XGo_untyped_bigint *big.Int
type XGo_untyped_bigrat *big.Rat
type XGo_untyped_bigfloat *big.Float

type XGo_untyped_bigint_Default = XGo_bigint
type XGo_untyped_bigrat_Default = XGo_bigrat
type XGo_untyped_bigfloat_Default = XGo_bigfloat

func XGo_untyped_bigint_Init__0(x int) XGo_untyped_bigint {
	panic("make compiler happy")
}

func XGo_untyped_bigrat_Init__0(x int) XGo_untyped_bigrat {
	panic("make compiler happy")
}

func XGo_untyped_bigrat_Init__1(x XGo_untyped_bigint) XGo_untyped_bigrat {
	panic("make compiler happy")
}

// -----------------------------------------------------------------------------
// type bigint

// A XGo_bigint represents a signed multi-precision integer.
// The zero value for a XGo_bigint represents nil.
type XGo_bigint struct {
	*big.Int
}

func tmpint(a, b XGo_bigint) XGo_bigint {
	if XGo_istmp(a) {
		return a
	} else if XGo_istmp(b) {
		return b
	}
	return XGo_bigint{new(big.Int)}
}

func tmpint1(a XGo_bigint) XGo_bigint {
	if XGo_istmp(a) {
		return a
	}
	return XGo_bigint{new(big.Int)}
}

// IsNil returns a bigint object is nil or not
func (a XGo_bigint) IsNil() bool {
	return a.Int == nil
}

// XGo_Assign: func (a bigint) = (b bigint)
func (a XGo_bigint) XGo_Assign(b XGo_bigint) {
	if XGo_istmp(b) {
		*a.Int = *b.Int
	} else {
		a.Int.Set(b.Int)
	}
}

// XGo_Add: func (a bigint) + (b bigint) bigint
func (a XGo_bigint) XGo_Add(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Add(a.Int, b.Int)}
}

// XGo_Sub: func (a bigint) - (b bigint) bigint
func (a XGo_bigint) XGo_Sub(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Sub(a.Int, b.Int)}
}

// XGo_Mul: func (a bigint) * (b bigint) bigint
func (a XGo_bigint) XGo_Mul(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Mul(a.Int, b.Int)}
}

// XGo_Quo: func (a bigint) / (b bigint) bigint {
func (a XGo_bigint) XGo_Quo(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Quo(a.Int, b.Int)}
}

// XGo_Rem: func (a bigint) % (b bigint) bigint
func (a XGo_bigint) XGo_Rem(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Rem(a.Int, b.Int)}
}

// XGo_Or: func (a bigint) | (b bigint) bigint
func (a XGo_bigint) XGo_Or(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Or(a.Int, b.Int)}
}

// XGo_Xor: func (a bigint) ^ (b bigint) bigint
func (a XGo_bigint) XGo_Xor(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).Xor(a.Int, b.Int)}
}

// XGo_And: func (a bigint) & (b bigint) bigint
func (a XGo_bigint) XGo_And(b XGo_bigint) XGo_bigint {
	return XGo_bigint{tmpint(a, b).And(a.Int, b.Int)}
}

// XGo_AndNot: func (a bigint) &^ (b bigint) bigint
func (a *XGo_bigint) XGo_AndNot__0(b *XGo_bigint) *XGo_bigint {
	return a
}

// XGo_Lsh: func (a bigint) << (n untyped_uint) bigint
func (a XGo_bigint) XGo_Lsh(n XGo_ninteger) XGo_bigint {
	return XGo_bigint{tmpint1(a).Lsh(a.Int, uint(n))}
}

// XGo_Rsh: func (a bigint) >> (n untyped_uint) bigint
func (a XGo_bigint) XGo_Rsh(n XGo_ninteger) XGo_bigint {
	return XGo_bigint{tmpint1(a).Rsh(a.Int, uint(n))}
}

// XGo_LT: func (a bigint) < (b bigint) bool
func (a XGo_bigint) XGo_LT(b XGo_bigint) bool {
	return a.Cmp(b.Int) < 0
}

// XGo_LE: func (a bigint) <= (b bigint) bool
func (a XGo_bigint) XGo_LE(b XGo_bigint) bool {
	return a.Cmp(b.Int) <= 0
}

// XGo_GT: func (a bigint) > (b bigint) bool
func (a XGo_bigint) XGo_GT(b XGo_bigint) bool {
	return a.Cmp(b.Int) > 0
}

// XGo_GE: func (a bigint) >= (b bigint) bool
func (a XGo_bigint) XGo_GE(b XGo_bigint) bool {
	return a.Cmp(b.Int) >= 0
}

// XGo_EQ: func (a bigint) == (b bigint) bool
func (a XGo_bigint) XGo_EQ(b XGo_bigint) bool {
	return a.Cmp(b.Int) == 0
}

// XGo_NE: func (a bigint) != (b bigint) bool
func (a XGo_bigint) XGo_NE(b XGo_bigint) bool {
	return a.Cmp(b.Int) != 0
}

// XGo_Neg: func -(a bigint) bigint
func (a XGo_bigint) XGo_Neg() XGo_bigint {
	return XGo_bigint{tmpint1(a).Neg(a.Int)}
}

// XGo_Dup: func +(a bigint) bigint
func (a XGo_bigint) XGo_Dup() XGo_bigint {
	return a
}

// XGo_Not: func ^(a bigint) bigint
func (a XGo_bigint) XGo_Not() XGo_bigint {
	return XGo_bigint{tmpint1(a).Not(a.Int)}
}

// XGo_Add: func (a bigint) += (b bigint)
func (a XGo_bigint) XGo_AddAssign(b XGo_bigint) {
	a.Int.Add(a.Int, b.Int)
}

// XGo_Sub: func (a bigint) -= (b bigint)
func (a XGo_bigint) XGo_SubAssign(b XGo_bigint) {
	a.Int.Sub(a.Int, b.Int)
}

// XGo_Mul: func (a bigint) *= (b bigint)
func (a XGo_bigint) XGo_MulAssign(b XGo_bigint) {
	a.Int.Mul(a.Int, b.Int)
}

// XGo_Quo: func (a bigint) /= (b bigint) {
func (a XGo_bigint) XGo_QuoAssign(b XGo_bigint) {
	a.Int.Quo(a.Int, b.Int)
}

// XGo_Rem: func (a bigint) %= (b bigint)
func (a XGo_bigint) XGo_RemAssign(b XGo_bigint) {
	a.Int.Rem(a.Int, b.Int)
}

// XGo_Or: func (a bigint) |= (b bigint)
func (a XGo_bigint) XGo_OrAssign(b XGo_bigint) {
	a.Int.Or(a.Int, b.Int)
}

// XGo_Xor: func (a bigint) ^= (b bigint)
func (a XGo_bigint) XGo_XorAssign(b XGo_bigint) {
	a.Int.Xor(a.Int, b.Int)
}

// XGo_And: func (a bigint) &= (b bigint)
func (a XGo_bigint) XGo_AndAssign(b XGo_bigint) {
	a.Int.And(a.Int, b.Int)
}

// XGo_AndNot: func (a bigint) &^= (b bigint)
func (a XGo_bigint) XGo_AndNotAssign(b XGo_bigint) {
	a.Int.AndNot(a.Int, b.Int)
}

// XGo_Lsh: func (a bigint) <<= (n untyped_uint)
func (a XGo_bigint) XGo_LshAssign(n XGo_ninteger) {
	a.Int.Lsh(a.Int, uint(n))
}

// XGo_Rsh: func (a bigint) >>= (n untyped_uint)
func (a XGo_bigint) XGo_RshAssign(n XGo_ninteger) {
	a.Int.Rsh(a.Int, uint(n))
}

func (a XGo_bigint) XGo_Rcast() float64 {
	return 0
}

// XGo_bigint_Cast: func bigint(x int) bigint
func XGo_bigint_Cast__0(x int) XGo_bigint {
	return XGo_bigint{big.NewInt(int64(x))}
}

// XGo_bigint_Cast: func bigint(x untyped_bigint) bigint
func XGo_bigint_Cast__1(x XGo_untyped_bigint) XGo_bigint {
	return XGo_bigint{x}
}

// XGo_bigint_Cast: func bigint(x int64) bigint
func XGo_bigint_Cast__2(x int64) XGo_bigint {
	return XGo_bigint{big.NewInt(x)}
}

// XGo_bigint_Cast: func bigint(x uint64) bigint
func XGo_bigint_Cast__3(x uint64) XGo_bigint {
	return XGo_bigint{new(big.Int).SetUint64(x)}
}

// XGo_bigint_Cast: func bigint(x uint) bigint
func XGo_bigint_Cast__4(x uint) XGo_bigint {
	return XGo_bigint{new(big.Int).SetUint64(uint64(x))}
}

// XGo_bigint_Cast: func bigint(x *big.Int) bigint
func XGo_bigint_Cast__5(x *big.Int) XGo_bigint {
	return XGo_bigint{x}
}

// XGo_bigint_Cast: func bigint(x bigrat) bigint
func XGo_bigint_Cast__6(x XGo_bigrat) XGo_bigint {
	if x.IsInt() {
		return XGo_bigint{x.Num()}
	}
	ret, _ := new(big.Float).SetRat(x.Rat).Int(nil)
	return XGo_bigint{ret}
}

// XGo_bigint_Cast: func bigint(x bigrat) (ret bigint, exact bool)
func XGo_bigint_Cast__7(x XGo_bigrat) (XGo_bigint, bool) {
	return XGo_bigint_Cast__6(x), x.IsInt()
}

// XGo_bigint_Init: func bigint.init(x int) bigint
func XGo_bigint_Init__0(x int) XGo_bigint {
	return XGo_bigint{big.NewInt(int64(x))}
}

// XGo_bigint_Init: func bigint.init(x *big.Int) bigint
func XGo_bigint_Init__1(x *big.Int) XGo_bigint {
	return XGo_bigint{x}
}

// XGo_bigint_Init: func bigint.init(x *big.Rat) bigint
func XGo_bigint_Init__2(x *big.Rat) XGo_bigint {
	if x.IsInt() {
		return XGo_bigint{x.Num()}
	}
	panic("TODO: can't init bigint from bigrat")
}

// -----------------------------------------------------------------------------
// type bigrat

// A XGo_bigrat represents a quotient a/b of arbitrary precision.
// The zero value for a XGo_bigrat represents nil.
type XGo_bigrat struct {
	*big.Rat
}

func tmprat(a, b XGo_bigrat) XGo_bigrat {
	if XGo_istmp(a) {
		return a
	} else if XGo_istmp(b) {
		return b
	}
	return XGo_bigrat{new(big.Rat)}
}

func tmprat1(a XGo_bigrat) XGo_bigrat {
	if XGo_istmp(a) {
		return a
	}
	return XGo_bigrat{new(big.Rat)}
}

// IsNil returns a bigrat object is nil or not
func (a XGo_bigrat) IsNil() bool {
	return a.Rat == nil
}

// XGo_Assign: func (a bigrat) = (b bigrat)
func (a XGo_bigrat) XGo_Assign(b XGo_bigrat) {
	if XGo_istmp(b) {
		*a.Rat = *b.Rat
	} else {
		a.Rat.Set(b.Rat)
	}
}

// XGo_Add: func (a bigrat) + (b bigrat) bigrat
func (a XGo_bigrat) XGo_Add(b XGo_bigrat) XGo_bigrat {
	return XGo_bigrat{tmprat(a, b).Add(a.Rat, b.Rat)}
}

// XGo_Sub: func (a bigrat) - (b bigrat) bigrat
func (a XGo_bigrat) XGo_Sub__0(b XGo_bigrat) XGo_bigrat {
	return XGo_bigrat{tmprat(a, b).Sub(a.Rat, b.Rat)}
}

// XGo_Mul: func (a bigrat) * (b bigrat) bigrat
func (a XGo_bigrat) XGo_Mul(b XGo_bigrat) XGo_bigrat {
	return XGo_bigrat{tmprat(a, b).Mul(a.Rat, b.Rat)}
}

// XGo_Quo: func (a bigrat) / (b bigrat) bigrat
func (a XGo_bigrat) XGo_Quo(b XGo_bigrat) XGo_bigrat {
	return XGo_bigrat{tmprat(a, b).Quo(a.Rat, b.Rat)}
}

// XGo_LT: func (a bigrat) < (b bigrat) bool
func (a XGo_bigrat) XGo_LT(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) < 0
}

// XGo_LE: func (a bigrat) <= (b bigrat) bool
func (a XGo_bigrat) XGo_LE(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) <= 0
}

// XGo_GT: func (a bigrat) > (b bigrat) bool
func (a XGo_bigrat) XGo_GT(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) > 0
}

// XGo_GE: func (a bigrat) >= (b bigrat) bool
func (a XGo_bigrat) XGo_GE(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) >= 0
}

// XGo_EQ: func (a bigrat) == (b bigrat) bool
func (a XGo_bigrat) XGo_EQ(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) == 0
}

// XGo_NE: func (a bigrat) != (b bigrat) bool
func (a XGo_bigrat) XGo_NE(b XGo_bigrat) bool {
	return a.Cmp(b.Rat) != 0
}

// XGo_Neg: func -(a bigrat) bigrat
func (a XGo_bigrat) XGo_Neg() XGo_bigrat {
	return XGo_bigrat{tmprat1(a).Neg(a.Rat)}
}

// XGo_Dup: func +(a bigrat) bigrat
func (a XGo_bigrat) XGo_Dup() XGo_bigrat {
	return a
}

// XGo_Inc: func ++(a *bigrat)
func (a *XGo_bigrat) XGo_Inc() {
}

// XGo_Dec: func --(a *bigrat) int
func (a *XGo_bigrat) XGo_Dec() int { // error!
	return 0
}

// XGo_Add: func (a bigrat) += (b bigrat)
func (a XGo_bigrat) XGo_AddAssign(b XGo_bigrat) {
	a.Rat.Add(a.Rat, b.Rat)
}

// XGo_Sub: func (a bigrat) -= (b bigrat) int
func (a XGo_bigrat) XGo_SubAssign(b XGo_bigrat) int { // error!
	return 0
}

// XGo_Mul: func (a bigrat) *= (b bigrat)
func (a XGo_bigrat) XGo_MulAssign(b XGo_bigrat) {
	a.Rat.Mul(a.Rat, b.Rat)
}

// XGo_Quo: func (a bigrat) /= (b bigrat)
func (a XGo_bigrat) XGo_QuoAssign(b XGo_bigrat) {
	a.Rat.Quo(a.Rat, b.Rat)
}

func (a XGo_bigrat) XGo_Rcast__0() (int, bool) {
	return 0, false
}

func (a *XGo_bigrat) XGo_Rcast__1() int {
	return 0
}

func (a *XGo_bigrat) XGo_Rcast__2() float64 {
	return 0
}

func (a *XGo_bigrat) XGo_Rcast__3(int) int64 {
	return 0
}

// XGo_bigrat_Cast: func bigrat(a untyped_bigint) bigrat
func XGo_bigrat_Cast__0(a XGo_untyped_bigint) XGo_bigrat {
	return XGo_bigrat{new(big.Rat).SetInt(a)}
}

// XGo_bigrat_Cast: func bigrat(a bigint) bigrat
func XGo_bigrat_Cast__1(a XGo_bigint) XGo_bigrat {
	return XGo_bigrat{new(big.Rat).SetInt(a.Int)}
}

// XGo_bigrat_Cast: func bigrat(a *big.Int) bigrat
func XGo_bigrat_Cast__2(a *big.Int) XGo_bigrat {
	return XGo_bigrat{new(big.Rat).SetInt(a)}
}

// XGo_bigrat_Cast: func bigrat(a, b int64) bigrat
func XGo_bigrat_Cast__3(a, b int64) XGo_bigrat {
	return XGo_bigrat{big.NewRat(a, b)}
}

// XGo_bigrat_Cast: func bigrat(a *big.Rat) bigrat
func XGo_bigrat_Cast__4(a *big.Rat) XGo_bigrat {
	return XGo_bigrat{a}
}

// XGo_bigrat_Cast: func bigrat() bigrat
func XGo_bigrat_Cast__5() XGo_bigrat {
	return XGo_bigrat{new(big.Rat)}
}

// XGo_bigrat_Init: func bigrat.init(x untyped_int) bigrat
func XGo_bigrat_Init__0(x int) XGo_bigrat {
	return XGo_bigrat{big.NewRat(int64(x), 1)}
}

// XGo_bigrat_Init: func bigrat.init(x untyped_bigint) bigrat
func XGo_bigrat_Init__1(x XGo_untyped_bigint) XGo_bigrat {
	return XGo_bigrat{new(big.Rat).SetInt(x)}
}

// XGo_bigrat_Init: func bigrat.init(x *big.Rat) bigrat
func XGo_bigrat_Init__2(x *big.Rat) XGo_bigrat {
	return XGo_bigrat{x}
}

// -----------------------------------------------------------------------------
// type bigfloat

// A XGo_bigfloat represents a multi-precision floating point number.
// The zero value for a XGo_bigfloat represents nil.
type XGo_bigfloat struct {
	*big.Float
}

// -----------------------------------------------------------------------------
