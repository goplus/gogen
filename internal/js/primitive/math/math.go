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

package math

import (
	_ "unsafe"

	"github.com/goplus/gogen/internal/js/primitive"
)

const (
	XGoJSPackage = "Math"
)

// ----------------------------------------------------------------------------

// Mathematical constants.
const (
	E       = 2.71828182845904523536028747135266249775724709369995957496696763 // https://oeis.org/A001113
	LN10    = 2.30258509299404568401799145468436420760110148862877297603332790
	LN2     = 0.693147180559945309417232121458176568075500134360255254120680009 // https://oeis.org/A002162
	LOG10E  = 1 / LN10
	LOG2E   = 1 / LN2
	PI      = 3.14159265358979323846264338327950288419716939937510582097494459 // https://oeis.org/A000796
	SQRT1_2 = SQRT_2 / 2                                                       // https://oeis.org/A002162
	SQRT_2  = 1.41421356237309504880168872420969807856967187537694807317667974 // https://oeis.org/A002193
)

// ----------------------------------------------------------------------------

//go:linkname Abs _
func Abs(primitive.Number) primitive.Number

//go:linkname Acos _
func Acos(primitive.Number) primitive.Number

//go:linkname Acosh _
func Acosh(primitive.Number) primitive.Number

//go:linkname Asin _
func Asin(primitive.Number) primitive.Number

//go:linkname Asinh _
func Asinh(primitive.Number) primitive.Number

//go:linkname Atan _
func Atan(primitive.Number) primitive.Number

//go:linkname Atan2 _
func Atan2(y, x primitive.Number) primitive.Number

//go:linkname Atanh _
func Atanh(primitive.Number) primitive.Number

//go:linkname Cbrt _
func Cbrt(primitive.Number) primitive.Number

//go:linkname Ceil _
func Ceil(primitive.Number) primitive.Number

//go:linkname Clz32 _
func Clz32(primitive.Number) primitive.Number

//go:linkname Cos _
func Cos(primitive.Number) primitive.Number

//go:linkname Cosh _
func Cosh(primitive.Number) primitive.Number

//go:linkname Exp _
func Exp(primitive.Number) primitive.Number

//go:linkname Expm1 _
func Expm1(primitive.Number) primitive.Number

//go:linkname Floor _
func Floor(primitive.Number) primitive.Number

//go:linkname F16round _
func F16round(primitive.Number) primitive.Number

//go:linkname Fround _
func Fround(primitive.Number) primitive.Number

//go:linkname Hypot _
func Hypot(p, q primitive.Number) primitive.Number

//go:linkname Imul _
func Imul(primitive.Number, primitive.Number) primitive.Number

//go:linkname Log _
func Log(primitive.Number) primitive.Number

//go:linkname Log10 _
func Log10(primitive.Number) primitive.Number

//go:linkname Log1p _
func Log1p(primitive.Number) primitive.Number

//go:linkname Log2 _
func Log2(primitive.Number) primitive.Number

//go:linkname Max _
func Max(...primitive.Number) primitive.Number

//go:linkname Min _
func Min(...primitive.Number) primitive.Number

//go:linkname Pow _
func Pow(x, y primitive.Number) primitive.Number

//go:linkname Random _
func Random() primitive.Number

//go:linkname Round _
func Round(primitive.Number) primitive.Number

//go:linkname Sign _
func Sign(x primitive.Number) primitive.Number

//go:linkname Sin _
func Sin(primitive.Number) primitive.Number

//go:linkname Sinh _
func Sinh(primitive.Number) primitive.Number

//go:linkname Sqrt _
func Sqrt(primitive.Number) primitive.Number

//go:linkname SumPrecise _
func SumPrecise([]primitive.Number) primitive.Number

//go:linkname Tan _
func Tan(primitive.Number) primitive.Number

//go:linkname Tanh _
func Tanh(primitive.Number) primitive.Number

//go:linkname Trunc _
func Trunc(primitive.Number) primitive.Number

// ----------------------------------------------------------------------------
