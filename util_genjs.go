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

package gogen

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/go/printer"
	"github.com/goplus/gogen/internal/target"
	"github.com/goplus/gogen/target/js"
)

func getFunExpr(fn *internal.Elem) (caller string, pos, end token.Pos) {
	panic("todo")
}

func getCaller(expr *internal.Elem) string {
	panic("todo")
}

func toDecls(decls []target.Decl) []ast.Decl {
	panic("todo")
}

func appendDecls(to []ast.Decl, decls []target.Decl) []ast.Decl {
	panic("todo")
}

func newFuncLit(pkg *Package, t *types.Signature, body *js.BlockStmt) *js.FuncLit {
	panic("todo")
}

func newCommentedNodes(p *Package, f *ast.File) *printer.CommentedNodes {
	return &printer.CommentedNodes{
		Node: f,
	}
}

func emitGoStmt(cb *CodeBuilder, call js.Expr) {
	panic("todo")
}

func emitDeferStmt(cb *CodeBuilder, call js.Expr) {
	panic("todo")
}

func emitSendStmt(cb *CodeBuilder, ch, val js.Expr) {
	panic("todo")
}

func emitGotoStmt(cb *CodeBuilder, name string) {
	panic("todo")
}

func emitIfStmt(cb *CodeBuilder, p *ifStmt, el js.Stmt) {
	cb.emitStmt(p.init)
	cb.emitStmt(&js.IfStmt{Cond: p.cond, Body: p.body, Else: el})
}

func emitSWitchStmt(cb *CodeBuilder, p *switchStmt, stmts []js.Stmt) {
	panic("todo")
}

func emitFullthrough(cb *CodeBuilder) {
	panic("todo")
}

func emitCaseClause(cb *CodeBuilder, p *caseStmt, body []js.Stmt) {
	panic("todo")
}

func emitSelectStmt(cb *CodeBuilder, stmts []js.Stmt) {
	panic("todo")
}

func emitCommClause(cb *CodeBuilder, p *commCase, body []js.Stmt) {
	panic("todo")
}

func emitTypeSwitchStmt(cb *CodeBuilder, p *typeSwitchStmt, stmts []js.Stmt) {
	panic("todo")
}

func emitTypeCaseClause(cb *CodeBuilder, p *typeCaseStmt, body []js.Stmt) {
	panic("todo")
}

func emitForRangeStmt(cb *CodeBuilder, p *forRangeStmt, stmts []js.Stmt) {
	panic("todo")
}
