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
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/target/js"
)

// -----------------------------------------------------------------------------

func IntLit(v int) *js.BasicLit {
	panic("todo IntLit")
}

func StringLit(v string) *js.BasicLit {
	return &js.BasicLit{Kind: token.STRING, Value: strconv.Quote(v)}
}

func RuneLit(v rune) *js.BasicLit {
	panic("todo RuneLit")
}

func FloatLit(v float64) *js.BasicLit {
	val := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(val, ".e") {
		val += ".0"
	}
	return &js.BasicLit{Kind: token.FLOAT, Value: val}
}

// -----------------------------------------------------------------------------

func toBasicKind(tok token.Token) types.BasicKind {
	return tok2BasicKinds[tok]
}

var (
	tok2BasicKinds = [...]types.BasicKind{
		token.INT:    types.UntypedInt,
		token.STRING: types.UntypedString,
		token.CHAR:   types.UntypedRune,
		token.FLOAT:  types.UntypedFloat,
		token.IMAG:   types.UntypedComplex,
	}
)

func ElemFromBasicLit(v *ast.BasicLit, src ast.Node) *internal.Elem {
	var lit *js.BasicLit
	switch v.Kind {
	case token.STRING:
		lit = &js.BasicLit{Kind: token.STRING, Value: v.Value}
	case token.INT, token.FLOAT:
		lit = &js.BasicLit{Kind: token.FLOAT, Value: v.Value}
	default:
		panic("todo: ElemFromBasicLit: " + v.Value)
	}
	return &internal.Elem{
		Val:  lit,
		Type: types.Typ[toBasicKind(v.Kind)],
		CVal: constant.MakeFromLiteral(v.Value, v.Kind, 0),
		Src:  src,
	}
}

// -----------------------------------------------------------------------------

func CheckParenExpr(x js.Expr) js.Expr {
	/* TODO(xsw):
	switch v := x.(type) {
	case *js.CompositeLit:
		return &js.ParenExpr{X: x}
	case *js.SelectorExpr:
		v.X = CheckParenExpr(v.X)
	}
	*/
	return x
}

// -----------------------------------------------------------------------------

func AddrOf(v js.Expr) js.Expr {
	panic("todo AddrOf")
}

// -----------------------------------------------------------------------------

type FakeExpr struct {
	js.Expr
	Real ast.Expr
}

func (e *FakeExpr) Pos() token.Pos { return e.Real.Pos() }
func (e *FakeExpr) End() token.Pos { return e.Real.End() }

func FakeExprOf(real ast.Expr) js.Expr {
	return &FakeExpr{Real: real}
}

// -----------------------------------------------------------------------------
