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

	"github.com/goplus/gogen/internal/js/primitive/math"
)

const (
	E   = math.E
	Pi  = math.PI
	Phi = 1.61803398874989484820458683436563811772030917980576286213544862

	Sqrt2   = math.SQRT_2
	SqrtE   = 1.64872127070012814684865078781416357165377610071014801157507931 // https://oeis.org/A019774
	SqrtPi  = 1.77245385090551602729816748334114518279754945612238712821380779 // https://oeis.org/A002161
	SqrtPhi = 1.27201964951406896425242246173749149171560804184009624861664038 // https://oeis.org/A139339

	Ln2    = math.LN2
	Log2E  = math.LOG2E
	Ln10   = math.LN10
	Log10E = math.LOG10E
)

// ----------------------------------------------------------------------------

const (
	XGoLink = `
Abs		github.com/goplus/gogen/internal/js/primitive/math.Abs
Acos	github.com/goplus/gogen/internal/js/primitive/math.Acos
Acosh	github.com/goplus/gogen/internal/js/primitive/math.Acosh
Asin	github.com/goplus/gogen/internal/js/primitive/math.Asin
Asinh	github.com/goplus/gogen/internal/js/primitive/math.Asinh
Atan	github.com/goplus/gogen/internal/js/primitive/math.Atan
Atan2	github.com/goplus/gogen/internal/js/primitive/math.Atan2
Atanh	github.com/goplus/gogen/internal/js/primitive/math.Atanh
Cbrt	github.com/goplus/gogen/internal/js/primitive/math.Cbrt
Ceil	github.com/goplus/gogen/internal/js/primitive/math.Ceil
Cos 	github.com/goplus/gogen/internal/js/primitive/math.Cos
Cosh	github.com/goplus/gogen/internal/js/primitive/math.Cosh
Exp		github.com/goplus/gogen/internal/js/primitive/math.Exp
Expm1	github.com/goplus/gogen/internal/js/primitive/math.Expm1
Floor 	github.com/goplus/gogen/internal/js/primitive/math.Floor
Max		github.com/goplus/gogen/internal/js/primitive/math.Max
Min		github.com/goplus/gogen/internal/js/primitive/math.Min
Hypot	github.com/goplus/gogen/internal/js/primitive/math.Hypot
Log 	github.com/goplus/gogen/internal/js/primitive/math.Log
Log10 	github.com/goplus/gogen/internal/js/primitive/math.Log10
Log1p 	github.com/goplus/gogen/internal/js/primitive/math.Log1p
Log2 	github.com/goplus/gogen/internal/js/primitive/math.Log2
Pow 	github.com/goplus/gogen/internal/js/primitive/math.Pow
Round 	github.com/goplus/gogen/internal/js/primitive/math.Round
Sin 	github.com/goplus/gogen/internal/js/primitive/math.Sin
Sinh 	github.com/goplus/gogen/internal/js/primitive/math.Sinh
Sqrt 	github.com/goplus/gogen/internal/js/primitive/math.Sqrt
Tan 	github.com/goplus/gogen/internal/js/primitive/math.Tan
Tanh 	github.com/goplus/gogen/internal/js/primitive/math.Tanh
Trunc 	github.com/goplus/gogen/internal/js/primitive/math.Trunc
`
)

//go:linkname Abs _
func Abs(x float64) float64

//go:linkname Acos _
func Acos(x float64) float64

//go:linkname Acosh _
func Acosh(x float64) float64

//go:linkname Asin _
func Asin(x float64) float64

//go:linkname Asinh _
func Asinh(x float64) float64

//go:linkname Atan _
func Atan(x float64) float64

//go:linkname Atan2 _
func Atan2(y, x float64) float64

//go:linkname Atanh _
func Atanh(x float64) float64

//go:linkname Cbrt _
func Cbrt(x float64) float64

//go:linkname Ceil _
func Ceil(x float64) float64

//go:linkname Cos _
func Cos(x float64) float64

//go:linkname Cosh _
func Cosh(x float64) float64

//func Copysign(x, y float64) float64
//func Dim(x, y float64) float64
//func Erf(x float64) float64
//func Erfc(x float64) float64

//go:linkname Exp _
func Exp(x float64) float64

//func Exp2(x float64) float64

//go:linkname Expm1 _
func Expm1(x float64) float64

//go:linkname Floor _
func Floor(x float64) float64

//func FMA(x, y, z float64) float64

//go:linkname Max _
func Max(x, y float64) float64

//go:linkname Min _
func Min(x, y float64) float64

//func Mod(x, y float64) float64
//func Frexp(f float64) (float64, int)
//func Gamma(x float64) float64

//go:linkname Hypot _
func Hypot(x, y float64) float64

//func Ilogb(x float64) int
//func J0(x float64) float64
//func J1(x float64) float64
//func Jn(n int, x float64) float64
//func Ldexp(x float64, exp int) float64
//func Lgamma(x float64) (lgamma float64, sign int)

//go:linkname Log _
func Log(x float64) float64

//go:linkname Log10 _
func Log10(x float64) float64

//go:linkname Log1p _
func Log1p(x float64) float64

//go:linkname Log2 _
func Log2(x float64) float64

//func Logb(x float64) float64
//func Modf(f float64) (float64, float64)
//func Nextafter(x, y float64) float64

//go:linkname Pow _
func Pow(x, y float64) float64

//func Remainder(x, y float64) float64

//go:linkname Round _
func Round(x float64) float64

//go:linkname Sin _
func Sin(x float64) float64

//go:linkname Sinh _
func Sinh(x float64) float64

//go:linkname Sqrt _
func Sqrt(x float64) float64

//go:linkname Tan _
func Tan(x float64) float64

//go:linkname Tanh _
func Tanh(x float64) float64

//go:linkname Trunc _
func Trunc(x float64) float64

//func Tgamma(x float64) float64

// ----------------------------------------------------------------------------
