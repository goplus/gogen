//go:build gengo
// +build gengo

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
)

// -----------------------------------------------------------------------------

func IntLit(v int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(v)}
}

func StringLit(v string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(v)}
}

func RuneLit(v rune) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.CHAR, Value: strconv.QuoteRune(v)}
}

func FloatLit(v float64) *ast.BasicLit {
	val := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(val, ".e") {
		val += ".0"
	}
	return &ast.BasicLit{Kind: token.FLOAT, Value: val}
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
	return &internal.Elem{
		Val:  v,
		Type: types.Typ[toBasicKind(v.Kind)],
		CVal: constant.MakeFromLiteral(v.Value, v.Kind, 0),
		Src:  src,
	}
}

// -----------------------------------------------------------------------------

func CheckParenExpr(x ast.Expr) ast.Expr {
	switch v := x.(type) {
	case *ast.CompositeLit:
		return &ast.ParenExpr{X: x}
	case *ast.SelectorExpr:
		v.X = CheckParenExpr(v.X)
	}
	return x
}

// -----------------------------------------------------------------------------

func Ref(x *ast.Ident, name string) ast.Expr {
	return &ast.SelectorExpr{
		X:   x,
		Sel: &ast.Ident{Name: name},
	}
}

func RefType(x *ast.Ident, name string) ast.Expr {
	return Ref(x, name)
}

func AddrOf(v ast.Expr) ast.Expr {
	return &ast.UnaryExpr{Op: token.AND, X: v}
}

func TypeExpr(typ ast.Expr) ast.Expr {
	return typ
}

// -----------------------------------------------------------------------------
