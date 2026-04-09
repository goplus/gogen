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
	"github.com/goplus/gogen/internal/target/js/universe"
)

// ----------------------------------------------------------------------------

// bool is the set of boolean values, true and false.
type Bool = universe.Boolean

// uint8 is the set of all unsigned 8-bit integers.
// Range: 0 through 255.
type Uint8 universe.Number

// uint16 is the set of all unsigned 16-bit integers.
// Range: 0 through 65535.
type Uint16 universe.Number

// uint32 is the set of all unsigned 32-bit integers.
// Range: 0 through 4294967295.
type Uint32 universe.Number

// uint64 is the set of all unsigned 64-bit integers.
// Range: 0 through 18446744073709551615.
type Uint64 universe.Number

// int8 is the set of all signed 8-bit integers.
// Range: -128 through 127.
type Int8 universe.Number

// int16 is the set of all signed 16-bit integers.
// Range: -32768 through 32767.
type Int16 universe.Number

// int32 is the set of all signed 32-bit integers.
// Range: -2147483648 through 2147483647.
type Int32 universe.Number

// int64 is the set of all signed 64-bit integers.
// Range: -9223372036854775808 through 9223372036854775807.
type Int64 universe.Number

// float32 is the set of all IEEE-754 32-bit floating-point numbers.
type Float32 universe.Number

// float64 is the set of all IEEE-754 64-bit floating-point numbers.
type Float64 universe.Number

/* TODO(xsw):
// complex64 is the set of all complex numbers with float32 real and
// imaginary parts.
type complex64 complex64

// complex128 is the set of all complex numbers with float64 real and
// imaginary parts.
type complex128 complex128
*/

// string is the set of all strings of 8-bit bytes, conventionally but not
// necessarily representing UTF-8-encoded text. A string may be empty, but
// not nil. Values of string type are immutable.
type String = universe.String

// int is a signed integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, int32.
type Int universe.Number

// uint is an unsigned integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, uint32.
type Uint universe.Number

// uintptr is an integer type that is large enough to hold the bit pattern of
// any pointer.
type Uintptr universe.Number

// byte is an alias for uint8 and is equivalent to uint8 in all ways. It is
// used, by convention, to distinguish byte values from 8-bit unsigned
// integer values.
type Byte = Uint8

// rune is an alias for int32 and is equivalent to int32 in all ways. It is
// used, by convention, to distinguish character values from integer values.
type Rune = Int32

// ----------------------------------------------------------------------------
