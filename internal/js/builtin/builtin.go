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
)

// ----------------------------------------------------------------------------

// bool is the set of boolean values, true and false.
type Bool = primitive.Boolean

// uint8 is the set of all unsigned 8-bit integers.
// Range: 0 through 255.
type Uint8 primitive.Number

// uint16 is the set of all unsigned 16-bit integers.
// Range: 0 through 65535.
type Uint16 primitive.Number

// uint64 is the set of all unsigned 64-bit integers.
// Range: 0 through 18446744073709551615.
type Uint64 primitive.Number

// int8 is the set of all signed 8-bit integers.
// Range: -128 through 127.
type Int8 primitive.Number

// int16 is the set of all signed 16-bit integers.
// Range: -32768 through 32767.
type Int16 primitive.Number

// int64 is the set of all signed 64-bit integers.
// Range: -9223372036854775808 through 9223372036854775807.
type Int64 primitive.Number

// float32 is the set of all IEEE-754 32-bit floating-point numbers.
type Float32 primitive.Number

// float64 is the set of all IEEE-754 64-bit floating-point numbers.
type Float64 = primitive.Number

// string is the set of all strings of 8-bit bytes, conventionally but not
// necessarily representing UTF-8-encoded text. A string may be empty, but
// not nil. Values of string type are immutable.
type String = primitive.String

// uintptr is an integer type that is large enough to hold the bit pattern of
// any pointer.
type Uintptr primitive.Number

// byte is an alias for uint8 and is equivalent to uint8 in all ways. It is
// used, by convention, to distinguish byte values from 8-bit unsigned
// integer values.
type Byte = Uint8

// ----------------------------------------------------------------------------
