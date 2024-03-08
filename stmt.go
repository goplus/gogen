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

package gogen

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gogen/internal"
)

type controlFlow interface {
	Then(cb *CodeBuilder, src ...ast.Node)
}

// ----------------------------------------------------------------------------
//
// block
//
//	...
//
// end
type blockStmt struct {
	old codeBlockCtx
}

func (p *blockStmt) End(cb *CodeBuilder, src ast.Node) {
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= flows
	cb.emitStmt(&ast.BlockStmt{List: stmts})
}

// ----------------------------------------------------------------------------
//
// vblock
//
//	...
//
// end
type vblockStmt struct {
	old vblockCtx
}

func (p *vblockStmt) End(cb *CodeBuilder, src ast.Node) {
	cb.endVBlockStmt(&p.old)
}

// ----------------------------------------------------------------------------
//
// if init; cond then
//
//	...
//
// else
//
//	...
//
// end
type ifStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
	old2 codeBlockCtx
}

func (p *ifStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	cond := cb.stk.Pop()
	if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
		cb.panicCodeError(getPos(src), "non-boolean condition in if statement")
	}
	p.cond = cond.Val
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("if statement has too many init statements")
	}
	cb.startBlockStmt(p, src, "if body", &p.old2)
}

func (p *ifStmt) Else(cb *CodeBuilder, src ...ast.Node) {
	if p.body != nil {
		panic("else statement already exists")
	}

	stmts, flows := cb.endBlockStmt(&p.old2)
	cb.current.flows |= flows

	p.body = &ast.BlockStmt{List: stmts}
	cb.startBlockStmt(p, src, "else body", &p.old2)
}

func (p *ifStmt) End(cb *CodeBuilder, src ast.Node) {
	stmts, flows := cb.endBlockStmt(&p.old2)
	cb.current.flows |= flows

	var blockStmt = &ast.BlockStmt{List: stmts}
	var el ast.Stmt
	if p.body != nil { // if..else
		el = blockStmt
		if len(stmts) == 1 { // if..else if
			if stmt, ok := stmts[0].(*ast.IfStmt); ok {
				el = stmt
			}
		}
	} else { // if without else
		p.body = blockStmt
	}
	cb.endBlockStmt(&p.old)
	cb.emitStmt(&ast.IfStmt{Init: p.init, Cond: p.cond, Body: p.body, Else: el})
}

// ----------------------------------------------------------------------------
//
// switch init; tag then
//
//	case expr1, expr2, ..., exprN then
//	  ...
//	  end
//
//	case expr1, expr2, ..., exprM then
//	  ...
//	  end
//
//	defaultThen
//	  ...
//	  end
//
// end
type switchStmt struct {
	init ast.Stmt
	tag  *internal.Elem
	old  codeBlockCtx
}

func (p *switchStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	if cb.stk.Len() == cb.current.base {
		panic("use None() for empty switch tag")
	}
	p.tag = cb.stk.Pop()
	switch stmts := cb.clearBlockStmt(); len(stmts) {
	case 0:
		// nothing to do
	case 1:
		p.init = stmts[0]
	default:
		panic("switch statement has too many init statements")
	}
}

func (p *switchStmt) Case(cb *CodeBuilder, src ...ast.Node) {
	stmt := &caseStmt{tag: p.tag}
	cb.startBlockStmt(stmt, src, "case statement", &stmt.old)
}

func (p *switchStmt) End(cb *CodeBuilder, src ast.Node) {
	if p.tag == nil {
		return
	}
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ flowFlagBreak)

	body := &ast.BlockStmt{List: stmts}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: checkParenExpr(p.tag.Val), Body: body})
}

type caseStmt struct {
	tag  *internal.Elem
	list []ast.Expr
	old  codeBlockCtx
}

func (p *caseStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	n := cb.stk.Len() - cb.current.base
	if n > 0 {
		p.list = make([]ast.Expr, n)
		for i, arg := range cb.stk.GetArgs(n) {
			if p.tag.Val != nil { // switch tag {...}
				if !ComparableTo(cb.pkg, arg, p.tag) {
					src, pos := cb.loadExpr(arg.Src)
					cb.panicCodeErrorf(
						pos, "cannot use %s (type %v) as type %v", src, arg.Type, types.Default(p.tag.Type))
				}
			} else { // switch {...}
				if !types.AssignableTo(arg.Type, types.Typ[types.Bool]) && arg.Type != TyEmptyInterface {
					src, pos := cb.loadExpr(arg.Src)
					cb.panicCodeErrorf(pos, "cannot use %s (type %v) as type bool", src, arg.Type)
				}
			}
			p.list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}
}

func (p *caseStmt) Fallthrough(cb *CodeBuilder) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.FALLTHROUGH})
}

func (p *caseStmt) End(cb *CodeBuilder, src ast.Node) {
	body, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= flows
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

// ----------------------------------------------------------------------------
//
// select
// commCase commStmt1 then
//
//	...
//	end
//
// commCase commStmt2 then
//
//	...
//	end
//
// commDefaultThen
//
//	...
//	end
//
// end
type selectStmt struct {
	old codeBlockCtx
}

func (p *selectStmt) CommCase(cb *CodeBuilder, src ...ast.Node) {
	stmt := &commCase{}
	cb.startBlockStmt(stmt, src, "comm case statement", &stmt.old)
}

func (p *selectStmt) End(cb *CodeBuilder, src ast.Node) {
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ flowFlagBreak)
	cb.emitStmt(&ast.SelectStmt{Body: &ast.BlockStmt{List: stmts}})
}

type commCase struct {
	old  codeBlockCtx
	comm ast.Stmt
}

func (p *commCase) Then(cb *CodeBuilder, src ...ast.Node) {
	switch len(cb.current.stmts) {
	case 1:
		p.comm = cb.popStmt()
	case 0:
	default:
		panic("multi commStmt in comm clause?")
	}
}

func (p *commCase) End(cb *CodeBuilder, src ast.Node) {
	body, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= flows
	cb.emitStmt(&ast.CommClause{Comm: p.comm, Body: body})
}

// ----------------------------------------------------------------------------
//
// typeSwitch(name) init; expr typeAssertThen
// typeCase type1, type2, ... typeN then
//
//	...
//	end
//
// typeCase type1, type2, ... typeM then
//
//	...
//	end
//
// typeDefaultThen
//
//	...
//	end
//
// end
type typeSwitchStmt struct {
	init  ast.Stmt
	name  string
	x     ast.Expr
	xSrc  ast.Node
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
	xType, ok := cb.checkInterface(x.Type)
	if !ok {
		panic("TODO: can't type assert on non interface expr")
	}
	p.x, p.xSrc, p.xType = x.Val, x.Src, xType
}

func (p *typeSwitchStmt) TypeCase(cb *CodeBuilder, src ...ast.Node) {
	stmt := &typeCaseStmt{pss: p}
	cb.startBlockStmt(stmt, src, "type case statement", &stmt.old)
}

func (p *typeSwitchStmt) End(cb *CodeBuilder, src ast.Node) {
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ flowFlagBreak)

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

type typeCaseStmt struct {
	pss  *typeSwitchStmt
	list []ast.Expr
	old  codeBlockCtx
}

func (p *typeCaseStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	var typ types.Type
	pss := p.pss
	n := cb.stk.Len() - cb.current.base
	if n > 0 {
		p.list = make([]ast.Expr, n)
		args := cb.stk.GetArgs(n)
		for i, arg := range args {
			typ = arg.Type
			if tt, ok := typ.(*TypeType); ok {
				typ = tt.Type()
				if missing := cb.missingMethod(typ, pss.xType); missing != "" {
					xsrc, _ := cb.loadExpr(pss.xSrc)
					pos := getSrcPos(arg.Src)
					cb.panicCodeErrorf(
						pos, "impossible type switch case: %s (type %v) cannot have dynamic type %v (missing %s method)",
						xsrc, pss.xType, typ, missing)
				}
			} else if typ != types.Typ[types.UntypedNil] {
				src, pos := cb.loadExpr(arg.Src)
				cb.panicCodeErrorf(pos, "%s (type %v) is not a type", src, typ)
			}
			p.list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}
	if pss.name != "" {
		if n != 1 { // default, or case with multi expr
			typ = pss.xType
		}
		name := types.NewParam(token.NoPos, cb.pkg.Types, pss.name, typ)
		cb.current.scope.Insert(name)
	}
}

func (p *typeCaseStmt) End(cb *CodeBuilder, src ast.Node) {
	body, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= flows
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

// ----------------------------------------------------------------------------

type loopBodyHandler struct {
	handle func(body *ast.BlockStmt, kind int)
}

func (p *loopBodyHandler) SetBodyHandler(handle func(body *ast.BlockStmt, kind int)) {
	p.handle = handle
}

func (p *loopBodyHandler) handleFor(body *ast.BlockStmt, kind int) *ast.BlockStmt {
	if p.handle != nil {
		p.handle(body, kind)
	}
	return body
}

func InsertStmtFront(body *ast.BlockStmt, stmt ast.Stmt) {
	list := append(body.List, nil)
	copy(list[1:], list)
	list[0] = stmt
	body.List = list
}

// ----------------------------------------------------------------------------
//
// for init; cond then
//
//	body
//	post
//
// end
type forStmt struct {
	init ast.Stmt
	cond ast.Expr
	body *ast.BlockStmt
	old  codeBlockCtx
	old2 codeBlockCtx
	loopBodyHandler
}

func (p *forStmt) Then(cb *CodeBuilder, src ...ast.Node) {
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
	cb.startBlockStmt(p, src, "for body", &p.old2)
}

func (p *forStmt) Post(cb *CodeBuilder) {
	stmts, flows := cb.endBlockStmt(&p.old2)
	cb.current.flows |= (flows &^ (flowFlagBreak | flowFlagContinue))
	p.body = &ast.BlockStmt{List: stmts}
}

func (p *forStmt) End(cb *CodeBuilder, src ast.Node) {
	var post ast.Stmt
	if p.body != nil { // has post stmt
		stmts, _ := cb.endBlockStmt(&p.old)
		if len(stmts) != 1 {
			panic("TODO: too many post statements")
		}
		post = stmts[0]
	} else { // no post
		stmts, flows := cb.endBlockStmt(&p.old2)
		cb.current.flows |= (flows &^ (flowFlagBreak | flowFlagContinue))
		p.body = &ast.BlockStmt{List: stmts}
		cb.endBlockStmt(&p.old)
	}
	cb.emitStmt(&ast.ForStmt{
		Init: p.init, Cond: p.cond, Post: post, Body: p.handleFor(p.body, 0),
	})
}

// ----------------------------------------------------------------------------
//
// forRange names... exprX rangeAssignThen
//
//	...
//
// end
//
// forRange exprKey exprVal exprX rangeAssignThen
//
//	...
//
// end
type forRangeStmt struct {
	names []string
	stmt  *ast.RangeStmt
	x     *internal.Elem
	old   codeBlockCtx
	kvt   []types.Type
	udt   int // 0: non-udt, 2: (elem,ok), 3: (key,elem,ok)
	loopBodyHandler
}

func (p *forRangeStmt) RangeAssignThen(cb *CodeBuilder, pos token.Pos) {
	if names := p.names; names != nil { // for k, v := range XXX {
		var val ast.Expr
		switch len(names) {
		case 1:
		case 2:
			val = ident(names[1])
		default:
			cb.panicCodeError(pos, "too many variables in range")
		}
		x := cb.stk.Pop()
		pkg, scope := cb.pkg, cb.current.scope
		typs := p.getKeyValTypes(cb, x.Type)
		if typs == nil {
			src, _ := cb.loadExpr(x.Src)
			cb.panicCodeErrorf(pos, "cannot range over %v (type %v)", src, x.Type)
		}
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
		if p.udt != 0 {
			p.x = x
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
			x = *args[0]
		case 2:
			key, x = *args[0], *args[1]
		case 3:
			key, val, x = *args[0], *args[1], *args[2]
		default:
			cb.panicCodeError(pos, "too many variables in range")
		}
		cb.stk.PopN(n)
		typs := p.getKeyValTypes(cb, x.Type)
		if typs == nil {
			src, _ := cb.loadExpr(x.Src)
			cb.panicCodeErrorf(pos, "cannot range over %v (type %v)", src, x.Type)
		}
		if p.udt != 0 {
			p.x = &x
		}
		p.stmt = &ast.RangeStmt{
			Key:   key.Val,
			Value: val.Val,
			X:     x.Val,
		}
		if n > 1 {
			p.stmt.Tok = token.ASSIGN
			checkAssign(cb.pkg, &key, typs[0], "range")
			if val.Val != nil {
				checkAssign(cb.pkg, &val, typs[1], "range")
			}
		}
	}
	p.stmt.For = pos
}

func (p *forRangeStmt) getKeyValTypes(cb *CodeBuilder, typ types.Type) []types.Type {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return []types.Type{types.Typ[types.Int], t.Elem()}
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}
	case *types.Array:
		return []types.Type{types.Typ[types.Int], t.Elem()}
	case *types.Pointer:
		switch e := t.Elem().(type) {
		case *types.Array:
			return []types.Type{types.Typ[types.Int], e.Elem()}
		case *types.Named:
			if kv, ok := p.checkUdt(cb, e); ok {
				return kv
			}
		}
	case *types.Chan:
		return []types.Type{t.Elem(), nil}
	case *types.Basic:
		if (t.Info() & types.IsString) != 0 {
			return []types.Type{types.Typ[types.Int], types.Typ[types.Rune]}
		}
	case *types.Named:
		if kv, ok := p.checkUdt(cb, t); ok {
			return kv
		}
		typ = cb.getUnderlying(t)
		goto retry
	}
	return nil
}

func (p *forRangeStmt) checkUdt(cb *CodeBuilder, o *types.Named) ([]types.Type, bool) {
	if sig := findMethodType(cb, o, nameGopEnum); sig != nil {
		enumRet := sig.Results()
		params := sig.Params()
		switch params.Len() {
		case 0:
			// iter := obj.Gop_Enum()
			// key, val, ok := iter.Next()
		case 1:
			// obj.Gop_Enum(func(key K, val V) { ... })
			if enumRet.Len() != 0 {
				return nil, false
			}
			typ := params.At(0).Type()
			if t, ok := typ.(*types.Signature); ok && t.Results().Len() == 0 {
				kv := t.Params()
				n := kv.Len()
				if n > 0 {
					p.kvt = []types.Type{kv.At(0).Type(), nil}
					if n > 1 {
						n = 2
						p.kvt[1] = kv.At(1).Type()
					}
					p.udt = -n
					return p.kvt, true
				}
			}
			fallthrough
		default:
			return nil, false
		}
		if enumRet.Len() == 1 {
			typ := enumRet.At(0).Type()
			if t, ok := typ.(*types.Pointer); ok {
				typ = t.Elem()
			}
			if it, ok := typ.(*types.Named); ok {
				if next := findMethodType(cb, it, "Next"); next != nil {
					ret := next.Results()
					typs := make([]types.Type, 2)
					n := ret.Len()
					switch n {
					case 2: // elem, ok
						typs[0] = ret.At(0).Type()
					case 3: // key, elem, ok
						typs[0], typs[1] = ret.At(0).Type(), ret.At(1).Type()
					default:
						return nil, false
					}
					if ret.At(n-1).Type() == types.Typ[types.Bool] {
						p.udt = n
						return typs, true
					}
				}
			}
		}
	}
	return nil, false
}

func findMethodType(cb *CodeBuilder, o *types.Named, name string) mthdSignature {
	for i, n := 0, o.NumMethods(); i < n; i++ {
		method := o.Method(i)
		if method.Name() == name {
			return method.Type().(*types.Signature)
		}
	}
	if bti := cb.getBuiltinTI(o); bti != nil {
		return bti.lookupByName(name)
	}
	return nil
}

const (
	cantUseFlows = "can't use return/continue/break/goto in for range of udt.Gop_Enum(callback)"
)

func (p *forRangeStmt) End(cb *CodeBuilder, src ast.Node) {
	if p.stmt == nil {
		return
	}
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ (flowFlagBreak | flowFlagContinue))
	if n := p.udt; n == 0 {
		p.stmt.Body = p.handleFor(&ast.BlockStmt{List: stmts}, 1)
		cb.emitStmt(p.stmt)
	} else if n > 0 {
		cb.stk.Push(p.x)
		cb.MemberVal(nameGopEnum).Call(0)
		callEnum := cb.stk.Pop().Val
		/*
			for _gop_it := X.Gop_Enum();; {
				var _gop_ok bool
				k, v, _gop_ok = _gop_it.Next()
				if !_gop_ok {
					break
				}
				...
			}
		*/
		lhs := make([]ast.Expr, n)
		lhs[0] = p.stmt.Key
		lhs[1] = p.stmt.Value
		lhs[n-1] = identGopOk
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
		body[0] = stmtGopOkDecl
		body[1] = &ast.AssignStmt{Lhs: lhs, Tok: p.stmt.Tok, Rhs: exprIterNext}
		body[2] = stmtBreakIfNotGopOk
		copy(body[3:], stmts)
		stmt := &ast.ForStmt{
			Init: &ast.AssignStmt{
				Lhs: []ast.Expr{identGopIt},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callEnum},
			},
			Body: p.handleFor(&ast.BlockStmt{List: body}, 2),
		}
		cb.emitStmt(stmt)
	} else {
		/*
			X.Gop_Enum(func(k K, v V) {
				...
			})
		*/
		if flows != 0 {
			cb.panicCodeError(p.stmt.For, cantUseFlows)
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
				Fun: &ast.SelectorExpr{X: p.stmt.X, Sel: identGopEnum},
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
	nameGopEnum  = "Gop_Enum"
	identGopOk   = ident("_gop_ok")
	identGopIt   = ident("_gop_it")
	identGopEnum = ident(nameGopEnum)
)

var (
	stmtGopOkDecl = &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{Names: []*ast.Ident{identGopOk}, Type: ident("bool")},
			},
		},
	}
	stmtBreakIfNotGopOk = &ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: identGopOk},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	}
	exprIterNext = []ast.Expr{
		&ast.CallExpr{
			Fun: &ast.SelectorExpr{X: identGopIt, Sel: ident("Next")},
		},
	}
)

// ----------------------------------------------------------------------------
