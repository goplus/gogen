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

func Abs(primitive.Number) primitive.Number
func Acos(primitive.Number) primitive.Number
func Acosh(primitive.Number) primitive.Number
func Asin(primitive.Number) primitive.Number
func Asinh(primitive.Number) primitive.Number
func Atan(primitive.Number) primitive.Number
func Atan2(y, x primitive.Number) primitive.Number
func Atanh(primitive.Number) primitive.Number
func Cbrt(primitive.Number) primitive.Number
func Ceil(primitive.Number) primitive.Number
func Clz32(primitive.Number) primitive.Number
func Cos(primitive.Number) primitive.Number
func Cosh(primitive.Number) primitive.Number
func Exp(primitive.Number) primitive.Number
func Expm1(primitive.Number) primitive.Number
func Floor(primitive.Number) primitive.Number
func F16round(primitive.Number) primitive.Number
func Fround(primitive.Number) primitive.Number
func Hypot(p, q primitive.Number) primitive.Number
func Imul(primitive.Number, primitive.Number) primitive.Number
func Log(primitive.Number) primitive.Number
func Log10(primitive.Number) primitive.Number
func Log1p(primitive.Number) primitive.Number
func Log2(primitive.Number) primitive.Number
func Max(...primitive.Number) primitive.Number
func Min(...primitive.Number) primitive.Number
func Pow(x, y primitive.Number) primitive.Number
func Random() primitive.Number
func Round(primitive.Number) primitive.Number
func Sign(x primitive.Number) primitive.Number
func Sin(primitive.Number) primitive.Number
func Sinh(primitive.Number) primitive.Number
func Sqrt(primitive.Number) primitive.Number
func SumPrecise([]primitive.Number) primitive.Number
func Tan(primitive.Number) primitive.Number
func Tanh(primitive.Number) primitive.Number
func Trunc(primitive.Number) primitive.Number

// ----------------------------------------------------------------------------
