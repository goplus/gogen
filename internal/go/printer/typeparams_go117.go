//go:build !go1.18
// +build !go1.18

package printer

import (
	"go/ast"
	"go/token"
)

func unsupported() {
	panic("type parameters are unsupported at this go version")
}

// IndexListExpr is a placeholder type, as type parameters are not supported at
// this Go version. Its methods panic on use.
type IndexListExpr struct {
	ast.Expr
	X       ast.Expr   // expression
	Lbrack  token.Pos  // position of "["
	Indices []ast.Expr // index expressions
	Rbrack  token.Pos  // position of "]"
}

func (*IndexListExpr) Pos() token.Pos { unsupported(); return token.NoPos }
func (*IndexListExpr) End() token.Pos { unsupported(); return token.NoPos }
