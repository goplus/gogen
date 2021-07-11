/*
 Copyright 2021 The GoPlus Authors (goplus.org)
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

package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gox/internal"
)

type controlFlow interface {
	Then(cb *CodeBuilder)
}

// ----------------------------------------------------------------------------
//
// if init; cond then
//   ...
// else
//   ...
// end
//
type ifStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
}

func (p *ifStmt) Then(cb *CodeBuilder) {
	cond := cb.stk.Pop()
	if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
		panic("TODO: if cond is not a boolean expr")
	}
	p.cond = cond.Val
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: if stmt has too many init statements")
	}
}

func (p *ifStmt) Else(cb *CodeBuilder) {
	if p.body != nil {
		panic("TODO: else statement already exists")
	}
	p.body = &ast.BlockStmt{List: cb.clearBlockStmt()}
}

func (p *ifStmt) End(cb *CodeBuilder) {
	var blockStmt = &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	var el ast.Stmt
	if p.body != nil { // if..else
		el = blockStmt
	} else { // if without else
		p.body = blockStmt
	}
	cb.emitStmt(&ast.IfStmt{Init: p.init, Cond: p.cond, Body: p.body, Else: el})
}

// ----------------------------------------------------------------------------
//
// switch init; tag then
//   expr1, expr2, ..., exprN case(N)
//     ...
//     end
//   expr1, expr2, ..., exprM case(M)
//     ...
//     end
// end
//
type switchStmt struct {
	init ast.Stmt
	tag  internal.Elem
	old  codeBlockCtx
}

func (p *switchStmt) Then(cb *CodeBuilder) {
	p.tag = cb.stk.Pop()
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: switch stmt has too many init statements")
	}
}

func (p *switchStmt) Case(cb *CodeBuilder, n int) {
	var list []ast.Expr
	if n > 0 {
		list = make([]ast.Expr, n)
		for i, arg := range cb.stk.GetArgs(n) {
			if p.tag.Val != nil { // switch tag {...}
				if !ComparableTo(arg.Type, p.tag.Type) {
					log.Panicf("TODO: case expr can't compare %v to %v\n", arg.Type, p.tag.Type)
				}
			} else { // switch {...}
				if !types.AssignableTo(arg.Type, types.Typ[types.Bool]) {
					log.Panicln("TODO: case expr is not a boolean expr")
				}
			}
			list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}
	stmt := &caseStmt{at: p, list: list}
	cb.startBlockStmt(stmt, "case statement", &stmt.old)
}

func (p *switchStmt) End(cb *CodeBuilder) {
	body := &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: p.tag.Val, Body: body})
}

type caseStmt struct {
	at   *switchStmt
	list []ast.Expr
	old  codeBlockCtx
}

func (p *caseStmt) Fallthrough(cb *CodeBuilder) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.FALLTHROUGH})
}

func (p *caseStmt) End(cb *CodeBuilder) {
	body := cb.endBlockStmt(p.old)
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

// ----------------------------------------------------------------------------
