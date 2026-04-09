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

package primitive

const (
	XGoJSPackage = ""
)

// ----------------------------------------------------------------------------

type String = string

// ----------------------------------------------------------------------------

type Boolean = bool

// ----------------------------------------------------------------------------

type Number float64

// Go native: a+b a-b a*b a/b -a

func (a Number) JS_Or(b Number) Number { // a | b
	panic("unreachable")
}

func (a Number) JS_And(b Number) Number { // a & b
	panic("unreachable")
}

func (a Number) JS_Xor(b Number) Number { // a ^ b
	panic("unreachable")
}

func (a Number) JS_Lsh(b Number) Number { // a << b
	panic("unreachable")
}

func (a Number) JS_Rsh(b Number) Number { // a >> b
	panic("unreachable")
}

func (a Number) JS_RshU(b Number) Number { // a >>> b
	panic("unreachable")
}

func (a Number) JS_Rem(b Number) Number { // a % b
	panic("unreachable")
}

func (a Number) JS_Not() Number { // ~a
	panic("unreachable")
}

// ----------------------------------------------------------------------------
