/*
Copyright 2026 The XGo Authors (xgo.dev)
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
	"github.com/goplus/gogen/internal/js/primitive"
	"github.com/goplus/gogen/internal/js/primitive/math"
)

// ----------------------------------------------------------------------------

// int is a signed integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, int32.
//
// The size of int is 32 bits for JS.
type Int primitive.Number

// Int_Cast: func int(v float64) int
func Int_Cast__0(v primitive.Number) Int {
	return Int(v.JS_Or(0))
}

// XGo_Add implements func (a int) + (b int) int
func (a Int) XGo_Add(b Int) Int {
	return Int((primitive.Number(a) + primitive.Number(b)).JS_Or(0))
}

// XGo_Sub implements func (a int) - (b int) int
func (a Int) XGo_Sub(b Int) Int {
	return Int((primitive.Number(a) - primitive.Number(b)).JS_Or(0))
}

// XGo_Mul implements func (a int) * (b int) int
func (a Int) XGo_Mul(b Int) Int {
	return Int(math.Imul(primitive.Number(a), primitive.Number(b)))
}

// XGo_Quo implements func (a int) / (b int) int
func (a Int) XGo_Quo(b Int) Int {
	return Int((primitive.Number(a) / primitive.Number(b)).JS_Or(0))
}

// XGo_Rem implements func (a int) % (b int) int
func (a Int) XGo_Rem(b Int) Int {
	return Int(primitive.Number(a).JS_Rem(primitive.Number(b)))
}

// XGo_Or implements func (a int) | (b int) int
func (a Int) XGo_Or(b Int) Int {
	return Int(primitive.Number(a).JS_Or(primitive.Number(b)))
}

// XGo_And implements func (a int) & (b int) int
func (a Int) XGo_And(b Int) Int {
	return Int(primitive.Number(a).JS_And(primitive.Number(b)))
}

// XGo_Xor implements func (a int) ^ (b int) int
func (a Int) XGo_Xor(b Int) Int {
	return Int(primitive.Number(a).JS_Xor(primitive.Number(b)))
}

// XGo_AndNot implements func (a int) &^ (b int) int
func (a Int) XGo_AndNot(b Int) Int {
	return Int(primitive.Number(a).JS_And(primitive.Number(b).JS_Not()))
}

// XGo_Lsh implements func (a int) << (b uint) int
func (a Int) XGo_Lsh(b Uint) Int {
	return Int(primitive.Number(a).JS_Lsh(primitive.Number(b)))
}

// XGo_Rsh implements func (a int) >> (b uint) int
func (a Int) XGo_Rsh(b Uint) Int {
	return Int(primitive.Number(a).JS_Rsh(primitive.Number(b)))
}

// XGo_Neg implements func -(a int) int
func (a Int) XGo_Neg() Int {
	return Int((-primitive.Number(a)).JS_Or(0))
}

// XGo_Not implements func ^(a int) int
func (a Int) XGo_Not() Int {
	return Int(primitive.Number(a).JS_Not())
}

// ----------------------------------------------------------------------------

// uint is an unsigned integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, uint.
type Uint primitive.Number

// Uint_Cast: func uint(v float64) uint
func Uint_Cast__0(v primitive.Number) Uint {
	return Uint(v.JS_RshU(0))
}

// XGo_Add implements func (a uint) + (b uint) uint
func (a Uint) XGo_Add(b Uint) Uint {
	return Uint((primitive.Number(a) + primitive.Number(b)).JS_RshU(0))
}

// XGo_Sub implements func (a uint) - (b uint) uint
func (a Uint) XGo_Sub(b Uint) Uint {
	return Uint((primitive.Number(a) - primitive.Number(b)).JS_RshU(0))
}

// XGo_Mul implements func (a uint) * (b uint) uint
func (a Uint) XGo_Mul(b Uint) Uint {
	return Uint(math.Imul(primitive.Number(a), primitive.Number(b)).JS_RshU(0))
}

// XGo_Quo implements func (a uint) / (b uint) uint
func (a Uint) XGo_Quo(b Uint) Uint {
	return Uint((primitive.Number(a) / primitive.Number(b)).JS_RshU(0))
}

// XGo_Rem implements func (a uint) % (b uint) uint
func (a Uint) XGo_Rem(b Uint) Uint {
	return Uint(primitive.Number(a).JS_Rem(primitive.Number(b)).JS_RshU(0))
}

// XGo_Or implements func (a uint) | (b uint) uint
func (a Uint) XGo_Or(b Uint) Uint {
	return Uint(primitive.Number(a).JS_Or(primitive.Number(b)).JS_RshU(0))
}

// XGo_And implements func (a uint) & (b uint) uint
func (a Uint) XGo_And(b Uint) Uint {
	return Uint(primitive.Number(a).JS_And(primitive.Number(b)).JS_RshU(0))
}

// XGo_Xor implements func (a uint) ^ (b uint) uint
func (a Uint) XGo_Xor(b Uint) Uint {
	return Uint(primitive.Number(a).JS_Xor(primitive.Number(b)).JS_RshU(0))
}

// XGo_AndNot implements func (a uint) &^ (b uint) uint
func (a Uint) XGo_AndNot(b Uint) Uint {
	return Uint(primitive.Number(a).JS_And(primitive.Number(b).JS_Not()).JS_RshU(0))
}

// XGo_Lsh implements func (a uint) << (b uint) uint
func (a Uint) XGo_Lsh(b Uint) Uint {
	return Uint(primitive.Number(a).JS_Lsh(primitive.Number(b)).JS_RshU(0))
}

// XGo_Rsh implements func (a uint) >> (b uint) uint
func (a Uint) XGo_Rsh(b Uint) Uint {
	return Uint(primitive.Number(a).JS_RshU(primitive.Number(b)))
}

// XGo_Neg implements func -(a uint) uint
func (a Uint) XGo_Neg() Uint {
	return Uint((-primitive.Number(a)).JS_RshU(0))
}

// XGo_Not implements func ^(a uint) uint
func (a Uint) XGo_Not() Uint {
	return Uint(primitive.Number(a).JS_Not().JS_RshU(0))
}

// ----------------------------------------------------------------------------
