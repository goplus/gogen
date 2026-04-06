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

package gogen

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/go/printer"
)

func getFunExpr(fn *internal.Elem) (caller string, pos, end token.Pos) {
	if fn == nil {
		return "the closure call", token.NoPos, token.NoPos
	}
	caller = types.ExprString(fn.Val)
	pos = getSrcPos(fn.Src)
	end = getSrcEnd(fn.Src)
	return
}

func getCaller(expr *internal.Elem) string {
	if ce, ok := expr.Val.(*ast.CallExpr); ok {
		return types.ExprString(ce.Fun)
	}
	return "the function call"
}

func toDecls(decls []ast.Decl) []ast.Decl {
	return decls
}

func appendDecls(to []ast.Decl, decls []ast.Decl) []ast.Decl {
	return append(to, decls...)
}

func newFuncLit(pkg *Package, t *types.Signature, body *ast.BlockStmt) *ast.FuncLit {
	return &ast.FuncLit{Type: toFuncType(pkg, t), Body: body}
}

func newCommentedNodes(p *Package, f *ast.File) *printer.CommentedNodes {
	return &printer.CommentedNodes{
		Node:           f,
		CommentedStmts: p.commentedStmts,
	}
}

func emitGoStmt(cb *CodeBuilder, call ast.Expr) {
	cb.emitStmt(&ast.GoStmt{Call: call})
}

func emitDeferStmt(cb *CodeBuilder, call ast.Expr) {
	cb.emitStmt(&ast.DeferStmt{Call: call})
}

func emitSendStmt(cb *CodeBuilder, ch, val ast.Expr) {
	cb.emitStmt(&ast.SendStmt{Chan: ch, Value: val})
}

func emitGotoStmt(cb *CodeBuilder, name string) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: ident(name)})
}

func emitIfStmt(cb *CodeBuilder, p *ifStmt, el ast.Stmt) {
	cb.emitStmt(&ast.IfStmt{Init: p.init, Cond: p.cond, Body: p.body, Else: el})
}

func emitSWitchStmt(cb *CodeBuilder, p *switchStmt, stmts []ast.Stmt) {
	body := &ast.BlockStmt{List: stmts}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: checkParenExpr(p.tag.Val), Body: body})
}

func emitFullthrough(cb *CodeBuilder) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.FALLTHROUGH})
}

func emitCaseClause(cb *CodeBuilder, p *caseStmt, body []ast.Stmt) {
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

func emitSelectStmt(cb *CodeBuilder, stmts []ast.Stmt) {
	cb.emitStmt(&ast.SelectStmt{Body: &ast.BlockStmt{List: stmts}})
}

func emitCommClause(cb *CodeBuilder, p *commCase, body []ast.Stmt) {
	cb.emitStmt(&ast.CommClause{Comm: p.comm, Body: body})
}

func emitTypeSwitchStmt(cb *CodeBuilder, p *typeSwitchStmt, stmts []ast.Stmt) {
	body := &ast.BlockStmt{List: stmts}
	var assign ast.Stmt
	x := &ast.TypeAssertExpr{X: p.x}
	if p.name != "" {
		assign = &ast.AssignStmt{
			Tok: token.DEFINE,
			Lhs: []ast.Expr{ident(p.name)},
			Rhs: []ast.Expr{x},
		}
	} else {
		assign = &ast.ExprStmt{X: x}
	}
	cb.emitStmt(&ast.TypeSwitchStmt{Init: p.init, Assign: assign, Body: body})
}

func emitTypeCaseClause(cb *CodeBuilder, p *typeCaseStmt, body []ast.Stmt) {
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

func emitForRangeStmt(cb *CodeBuilder, p *forRangeStmt, stmts []ast.Stmt) {
	if n := p.udt; n == 0 {
		if p.enumName != "" {
			p.stmt.X = &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: p.stmt.X, Sel: ident(p.enumName)},
			}
		}
		p.stmt.Body = p.handleFor(&ast.BlockStmt{List: stmts}, 1)
		cb.emitStmt(p.stmt)
	} else if n > 0 {
		cb.stk.Push(p.x)
		cb.MemberVal(p.enumName, 0).Call(0)
		callEnum := cb.stk.Pop().Val
		/*
			for _xgo_it := X.XGo_Enum();; {
				var _xgo_ok bool
				k, v, _xgo_ok = _xgo_it.Next()
				if !_xgo_ok {
					break
				}
				...
			}
		*/
		lhs := make([]ast.Expr, n)
		lhs[0] = p.stmt.Key
		lhs[1] = p.stmt.Value
		lhs[n-1] = identXgoOk
		if lhs[0] == nil { // bugfix: for range udt { ... }
			lhs[0] = underscore
			if p.stmt.Tok == token.ILLEGAL {
				p.stmt.Tok = token.ASSIGN
			}
		} else {
			// for _ = range udt { ... }
			if ki, ok := lhs[0].(*ast.Ident); ok && ki.Name == "_" {
				if n == 2 {
					p.stmt.Tok = token.ASSIGN
				} else {
					// for _ , _ = range udt { ... }
					if vi, ok := lhs[1].(*ast.Ident); ok && vi.Name == "_" {
						p.stmt.Tok = token.ASSIGN
					}
				}
			}
		}
		body := make([]ast.Stmt, len(stmts)+3)
		body[0] = stmtXGoOkDecl
		body[1] = &ast.AssignStmt{Lhs: lhs, Tok: p.stmt.Tok, Rhs: exprIterNext}
		body[2] = stmtBreakIfNotXGoOk
		copy(body[3:], stmts)
		stmt := &ast.ForStmt{
			Init: &ast.AssignStmt{
				Lhs: []ast.Expr{identXgoIt},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callEnum},
			},
			Body: p.handleFor(&ast.BlockStmt{List: body}, 2),
		}
		cb.emitStmt(stmt)
	} else {
		/*
			X.XGo_Enum(func(k K, v V) {
				...
			})
		*/
		if flows != 0 {
			cb.panicCodeError(p.stmt.For, p.stmt.For, cantUseFlows)
		}
		n = -n
		def := p.stmt.Tok == token.DEFINE
		args := make([]*ast.Field, n)
		if def {
			args[0] = &ast.Field{
				Names: []*ast.Ident{p.stmt.Key.(*ast.Ident)},
				Type:  toType(cb.pkg, p.kvt[0]),
			}
			if n > 1 {
				args[1] = &ast.Field{
					Names: []*ast.Ident{p.stmt.Value.(*ast.Ident)},
					Type:  toType(cb.pkg, p.kvt[1]),
				}
			}
		} else {
			panic("TODO: for range udt assign")
		}
		stmt := &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: p.stmt.X, Sel: ident(p.enumName)},
				Args: []ast.Expr{
					&ast.FuncLit{
						Type: &ast.FuncType{Params: &ast.FieldList{List: args}},
						Body: p.handleFor(&ast.BlockStmt{List: stmts}, -1),
					},
				},
			},
		}
		cb.emitStmt(stmt)
	}
}

var (
	identXgoOk = ident("_xgo_ok")
	identXgoIt = ident("_xgo_it")
)

var (
	stmtXGoOkDecl = &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{Names: []*ast.Ident{identXgoOk}, Type: ident("bool")},
			},
		},
	}
	stmtBreakIfNotXGoOk = &ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: identXgoOk},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	}
	exprIterNext = []ast.Expr{
		&ast.CallExpr{
			Fun: &ast.SelectorExpr{X: identXgoIt, Sel: ident("Next")},
		},
	}
)
