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

package target

import (
	"go/token"

	"github.com/goplus/gogen/target/js"
)

const (
	Kind = JS
)

type (
	Object = js.Object
	PkgRef = js.PkgRef

	Expr         = js.Expr
	BasicLit     = js.BasicLit
	Ident        = js.Ident
	UnaryExpr    = js.UnaryExpr
	BinaryExpr   = js.BinaryExpr
	SelectorExpr = js.SelectorExpr
	CallExpr     = js.CallExpr
	ParenExpr    = js.ParenExpr
	FuncLit      = js.FuncLit
	IndexExpr    = js.IndexExpr

	Stmt        = js.Stmt
	EmptyStmt   = js.EmptyStmt
	LabeledStmt = js.LabeledStmt
	ExprStmt    = js.ExprStmt
	BlockStmt   = js.BlockStmt
	IfStmt      = js.IfStmt
	ForStmt     = js.ForStmt
	BranchStmt  = js.BranchStmt
)

type RangeStmt struct {
	For        token.Pos   // position of "for" keyword
	Key, Value Expr        // Key, Value may be nil
	TokPos     token.Pos   // position of Tok; invalid if Key == nil
	Tok        token.Token // ILLEGAL if Key == nil, ASSIGN, DEFINE
	Range      token.Pos   // position of "range" keyword
	X          Expr        // value to range over
	Body       *BlockStmt
}

// An AssignStmt node represents an assignment or
// a short variable declaration.
type AssignStmt struct {
	js.Stmt
	Lhs    []Expr
	TokPos token.Pos   // position of Tok
	Tok    token.Token // assignment token, DEFINE
	Rhs    []Expr
}
