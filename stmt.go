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
		panic("TODO: if statement condition is not a boolean expr")
	}
	p.cond = cond.Val
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: if statement has too many init statements")
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
		panic("TODO: switch statement has too many init statements")
	}
}

func (p *switchStmt) Case(cb *CodeBuilder, n int) {
	var list []ast.Expr
	if n > 0 {
		list = make([]ast.Expr, n)
		for i, arg := range cb.stk.GetArgs(n) {
			if p.tag.Val != nil { // switch tag {...}
				if !ComparableTo(cb, arg.Type, p.tag.Type) {
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
	stmt := &caseStmt{list: list}
	cb.startBlockStmt(stmt, "case statement", &stmt.old)
}

func (p *switchStmt) End(cb *CodeBuilder) {
	body := &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: p.tag.Val, Body: body})
}

type caseStmt struct {
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
//
// select
// commStmt1 commCase()
//    ...
//    end
// commStmt2 commCase()
//    ...
//    end
// end
type selectStmt struct {
	old codeBlockCtx
}

func (p *selectStmt) CommCase(cb *CodeBuilder, n int) {
	var comm ast.Stmt
	if n == 1 {
		comm = cb.popStmt()
	}
	stmt := &commCase{comm: comm}
	cb.startBlockStmt(stmt, "comm case statement", &stmt.old)
}

func (p *selectStmt) End(cb *CodeBuilder) {
	body := &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(&ast.SelectStmt{Body: body})
}

type commCase struct {
	comm ast.Stmt
	old  codeBlockCtx
}

func (p *commCase) End(cb *CodeBuilder) {
	body := cb.endBlockStmt(p.old)
	cb.emitStmt(&ast.CommClause{Comm: p.comm, Body: body})
}

// ----------------------------------------------------------------------------
//
// typeSwitch(name) init; expr typeAssertThen
// type1, type2, ... typeN typeCase(N)
//    ...
//    end
// type1, type2, ... typeM typeCase(M)
//    ...
//    end
// end
//
type typeSwitchStmt struct {
	init  ast.Stmt
	name  string
	x     ast.Expr
	xType *types.Interface
	old   codeBlockCtx
}

func (p *typeSwitchStmt) TypeAssertThen(cb *CodeBuilder) {
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: type switch statement has too many init statements")
	}
	x := cb.stk.Pop()
	xType, ok := x.Type.(*types.Interface)
	if !ok {
		panic("TODO: can't type assert on non interface expr")
	}
	p.x, p.xType = x.Val, xType
}

func (p *typeSwitchStmt) TypeCase(cb *CodeBuilder, n int) {
	var list []ast.Expr
	var typ types.Type
	if n > 0 {
		list = make([]ast.Expr, n)
		args := cb.stk.GetArgs(n)
		for i, arg := range args {
			tt := arg.Type.(*TypeType)
			typ = tt.typ
			if !types.AssertableTo(p.xType, typ) {
				log.Panicf("TODO: can't assert type %v to %v\n", p.xType, typ)
			}
			list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}

	stmt := &typeCaseStmt{list: list}
	cb.startBlockStmt(stmt, "type case statement", &stmt.old)

	if p.name != "" {
		if n != 1 { // default, or case with multi expr
			typ = p.xType
		}
		name := types.NewParam(token.NoPos, cb.pkg.Types, p.name, typ)
		cb.current.scope.Insert(name)
	}
}

func (p *typeSwitchStmt) End(cb *CodeBuilder) {
	body := &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
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

type typeCaseStmt struct {
	list []ast.Expr
	old  codeBlockCtx
}

func (p *typeCaseStmt) End(cb *CodeBuilder) {
	body := cb.endBlockStmt(p.old)
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

// ----------------------------------------------------------------------------
//
// for init; cond then
//   body
//   post
// end
//
type forStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
}

func (p *forStmt) Then(cb *CodeBuilder) {
	cond := cb.stk.Pop()
	if cond.Val != nil {
		if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
			panic("TODO: for statement condition is not a boolean expr")
		}
		p.cond = cond.Val
	}
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("TODO: for condition has too many init statements")
	}
}

func (p *forStmt) Post(cb *CodeBuilder) {
	p.body = &ast.BlockStmt{List: cb.clearBlockStmt()}
}

func (p *forStmt) End(cb *CodeBuilder) {
	var stmts = cb.endBlockStmt(p.old)
	var post ast.Stmt
	if p.body != nil { // has post stmt
		if len(stmts) != 1 {
			panic("TODO: too many post statements")
		}
		post = stmts[0]
	} else { // no post
		p.body = &ast.BlockStmt{List: stmts}
	}
	cb.emitStmt(&ast.ForStmt{Init: p.init, Cond: p.cond, Post: post, Body: p.body})
}

// ----------------------------------------------------------------------------
//
// forRange names... exprX rangeAssignThen
//   ...
// end
//
// forRange exprKey exprVal exprX rangeAssignThen
//   ...
// end
//
type forRangeStmt struct {
	names []string
	stmt  *ast.RangeStmt
	old   codeBlockCtx
}

func (p *forRangeStmt) RangeAssignThen(cb *CodeBuilder) {
	if names := p.names; names != nil { // for k, v := range XXX {
		var val ast.Expr
		switch len(names) {
		case 1:
		case 2:
			val = ident(names[1])
		default:
			panic("TODO: invalid syntax of for range :=")
		}
		x := cb.stk.Pop()
		pkg, scope := cb.pkg, cb.current.scope
		typs := getKeyValTypes(x.Type)
		if typs[1] == nil { // chan
			if names[0] == "_" && len(names) > 1 {
				names[0], val = names[1], nil
				names = names[:1]
			}
		}
		for i, name := range names {
			if name == "_" {
				continue
			}
			if scope.Insert(types.NewVar(token.NoPos, pkg.Types, name, typs[i])) != nil {
				log.Panicln("TODO: variable already defined -", name)
			}
		}
		p.stmt = &ast.RangeStmt{
			Key:   ident(names[0]),
			Value: val,
			Tok:   token.DEFINE,
			X:     x.Val,
		}
	} else { // for k, v = range XXX {
		var key, val, x internal.Elem
		n := cb.stk.Len() - cb.current.base
		args := cb.stk.GetArgs(n)
		switch n {
		case 1:
			x = args[0]
		case 2:
			key, x = args[0], args[1]
		case 3:
			key, val, x = args[0], args[1], args[2]
		default:
			panic("TODO: invalid syntax of for range =")
		}
		cb.stk.PopN(n)
		p.stmt = &ast.RangeStmt{
			Key:   key.Val,
			Value: val.Val,
			X:     x.Val,
		}
		if n > 1 {
			p.stmt.Tok = token.ASSIGN
			typs := getKeyValTypes(x.Type)
			checkAssign(cb.pkg, key, typs[0], "range")
			if val.Val != nil {
				checkAssign(cb.pkg, val, typs[1], "range")
			}
		}
	}
}

func getKeyValTypes(typ types.Type) []types.Type {
	switch t := typ.(type) {
	case *types.Slice:
		return []types.Type{types.Typ[types.Int], t.Elem()}
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}
	case *types.Array:
		return []types.Type{types.Typ[types.Int], t.Elem()}
	case *types.Pointer:
		if e, ok := t.Elem().(*types.Array); ok {
			return []types.Type{types.Typ[types.Int], e.Elem()}
		}
	case *types.Chan:
		return []types.Type{t.Elem(), nil}
	}
	log.Panicln("TODO: can't for range to type", typ)
	return nil
}

func (p *forRangeStmt) End(cb *CodeBuilder) {
	p.stmt.Body = &ast.BlockStmt{List: cb.endBlockStmt(p.old)}
	cb.emitStmt(p.stmt)
}

// ----------------------------------------------------------------------------
