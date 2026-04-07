//go:build genjs

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

package util

import (
	"go/ast"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/target/js"
)

// -----------------------------------------------------------------------------

func IntLit(v int) *js.BasicLit {
	panic("todo")
}

func StringLit(v string) *js.BasicLit {
	panic("todo")
}

func RuneLit(v rune) *js.BasicLit {
	panic("todo")
}

func FloatLit(v float64) *js.BasicLit {
	panic("todo")
}

// -----------------------------------------------------------------------------

func ElemFromBasicLit(v *ast.BasicLit, src ast.Node) *internal.Elem {
	panic("todo")
}

// -----------------------------------------------------------------------------

func CheckParenExpr(x js.Expr) js.Expr {
	panic("todo")
}

// -----------------------------------------------------------------------------

func Ref(x *js.PkgRef, name string) js.Expr {
	panic("todo")
}

func RefType(x *js.PkgRef, name string) ast.Expr {
	panic("todo")
}

func AddrOf(v js.Expr) js.Expr {
	panic("todo")
}

func TypeExpr(typ ast.Expr) js.Expr {
	panic("todo")
}

// -----------------------------------------------------------------------------
