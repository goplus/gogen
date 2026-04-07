/*
 Copyright 2021 The XGo Authors (xgo.dev)
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
	"github.com/goplus/gogen/target"
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
	cb.emitStmt(&target.BlockStmt{List: stmts})
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
	init target.Stmt
	cond target.Expr
	body *target.BlockStmt
	old  codeBlockCtx
	old2 codeBlockCtx
}

func (p *ifStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	cond := cb.stk.Pop()
	if !types.AssignableTo(cond.Type, types.Typ[types.Bool]) {
		cb.panicCodeError(getPos(src), getEnd(src), "non-boolean condition in if statement")
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

	p.body = &target.BlockStmt{List: stmts}
	cb.startBlockStmt(p, src, "else body", &p.old2)
}

func (p *ifStmt) End(cb *CodeBuilder, src ast.Node) {
	stmts, flows := cb.endBlockStmt(&p.old2)
	cb.current.flows |= flows

	var blockStmt = &target.BlockStmt{List: stmts}
	var el target.Stmt
	if p.body != nil { // if..else
		el = blockStmt
		if len(stmts) == 1 { // if..else if
			if stmt, ok := stmts[0].(*target.IfStmt); ok {
				el = stmt
			}
		}
	} else { // if without else
		p.body = blockStmt
	}
	cb.endBlockStmt(&p.old)
	emitIfStmt(cb, p, el)
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
	init target.Stmt
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
		if cb != nil {
			cb.endBlockStmt(&p.old)
		}
		return
	}
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ flowFlagBreak)

	emitSWitchStmt(cb, p, stmts)
}

type caseStmt struct {
	tag  *internal.Elem
	list []target.Expr
	old  codeBlockCtx
}

func (p *caseStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	n := cb.stk.Len() - cb.current.base
	if n > 0 {
		p.list = make([]target.Expr, n)
		for i, arg := range cb.stk.GetArgs(n) {
			if p.tag.Val != nil { // switch tag {...}
				if !ComparableTo(cb.pkg, arg, p.tag) {
					src, pos, end := cb.loadExpr(arg.Src)
					cb.panicCodeErrorf(
						pos, end, "cannot use %s (type %v) as type %v", src, arg.Type, types.Default(p.tag.Type))
				}
			} else { // switch {...}
				if !types.AssignableTo(arg.Type, types.Typ[types.Bool]) && arg.Type != TyEmptyInterface {
					src, pos, end := cb.loadExpr(arg.Src)
					cb.panicCodeErrorf(pos, end, "cannot use %s (type %v) as type bool", src, arg.Type)
				}
			}
			p.list[i] = arg.Val
		}
		cb.stk.PopN(n)
	}
}

func (p *caseStmt) Fallthrough(cb *CodeBuilder) {
	emitFullthrough(cb)
}

func (p *caseStmt) End(cb *CodeBuilder, src ast.Node) {
	body, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= flows
	emitCaseClause(cb, p, body)
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
	emitSelectStmt(cb, stmts)
}

type commCase struct {
	old  codeBlockCtx
	comm target.Stmt
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
	emitCommClause(cb, p, body)
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
	init  target.Stmt
	name  string
	x     target.Expr
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
	emitTypeSwitchStmt(cb, p, stmts)
}

type typeCaseStmt struct {
	pss  *typeSwitchStmt
	list []target.Expr
	old  codeBlockCtx
}

func (p *typeCaseStmt) Then(cb *CodeBuilder, src ...ast.Node) {
	var typ types.Type
	pss := p.pss
	n := cb.stk.Len() - cb.current.base
	if n > 0 {
		p.list = make([]target.Expr, n)
		args := cb.stk.GetArgs(n)
		for i, arg := range args {
			typ = arg.Type
			if tt, ok := typ.(*TypeType); ok {
				typ = tt.Type()
				if missing := cb.missingMethod(typ, pss.xType); missing != "" {
					xsrc, _, _ := cb.loadExpr(pss.xSrc)
					pos := getSrcPos(arg.Src)
					end := getSrcEnd(arg.Src)
					cb.panicCodeErrorf(
						pos, end, "impossible type switch case: %s (type %v) cannot have dynamic type %v (missing %s method)",
						xsrc, pss.xType, typ, missing)
				}
			} else if typ != types.Typ[types.UntypedNil] {
				src, pos, end := cb.loadExpr(arg.Src)
				cb.panicCodeErrorf(pos, end, "%s (type %v) is not a type", src, typ)
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
	emitTypeCaseClause(cb, p, body)
}

// ----------------------------------------------------------------------------

type loopBodyHandler struct {
	handle func(body *target.BlockStmt, kind int)
}

func (p *loopBodyHandler) SetBodyHandler(handle func(body *target.BlockStmt, kind int)) {
	p.handle = handle
}

func (p *loopBodyHandler) handleFor(body *target.BlockStmt, kind int) *target.BlockStmt {
	if p.handle != nil {
		p.handle(body, kind)
	}
	return body
}

func InsertStmtFront(body *target.BlockStmt, stmt target.Stmt) {
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
	init target.Stmt
	cond target.Expr
	body *target.BlockStmt
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
	p.body = &target.BlockStmt{List: stmts}
}

func (p *forStmt) End(cb *CodeBuilder, src ast.Node) {
	var post target.Stmt
	if p.body != nil { // has post stmt
		stmts, _ := cb.endBlockStmt(&p.old)
		if len(stmts) != 1 {
			panic("TODO: too many post statements")
		}
		post = stmts[0]
	} else { // no post
		stmts, flows := cb.endBlockStmt(&p.old2)
		cb.current.flows |= (flows &^ (flowFlagBreak | flowFlagContinue))
		p.body = &target.BlockStmt{List: stmts}
		cb.endBlockStmt(&p.old)
	}
	cb.emitStmt(&target.ForStmt{
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
	names    []string
	stmt     *target.RangeStmt
	x        *internal.Elem
	old      codeBlockCtx
	kvt      []types.Type
	preStmts []target.Stmt // statements emitted during range expression compilation

	enumName string // XGo_Enum or Gop_Enum

	udt int // 0: non-udt, 2: (elem,ok), 3: (key,elem,ok)
	loopBodyHandler
}

func (p *forRangeStmt) RangeAssignThen(cb *CodeBuilder, pos token.Pos) {
	// Extract statements emitted during range expression compilation (e.g., auto-generated
	// type assertions like `_autoGo_1, _ := doc.(map[string]any)`). These were emitted to
	// the for-range block scope but belong in the outer scope before the for statement.
	p.preStmts = cb.clearBlockStmt()
	if names := p.names; names != nil { // for k, v := range XXX {
		var val target.Expr
		switch len(names) {
		case 1:
		case 2:
			val = ident(names[1])
		default:
			cb.panicCodeError(pos, pos, "too many variables in range")
		}
		x := cb.stk.Pop()
		pkg, scope := cb.pkg, cb.current.scope
		typs := p.getKeyValTypes(cb, x.Type)
		if typs == nil {
			src, _, _ := cb.loadExpr(x.Src)
			cb.panicCodeErrorf(pos, pos, "cannot range over %v (type %v)", src, x.Type)
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
		p.stmt = &target.RangeStmt{
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
			cb.panicCodeError(pos, pos, "too many variables in range")
		}
		cb.stk.PopN(n)
		typs := p.getKeyValTypes(cb, x.Type)
		if typs == nil {
			src, _, _ := cb.loadExpr(x.Src)
			cb.panicCodeErrorf(pos, pos, "cannot range over %v (type %v)", src, x.Type)
		}
		if p.udt != 0 {
			p.x = &x
		}
		p.stmt = &target.RangeStmt{
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
	case *types.Signature:
		// Go 1.23 range over function types:
		// func(yield func() bool)           - 0 values
		// func(yield func(V) bool)          - 1 value
		// func(yield func(K, V) bool)       - 2 values
		return checkIteratorFunc(t)
	case *types.Named:
		if kv, ok := p.checkUdt(cb, t); ok {
			return kv
		}
		typ = cb.getUnderlying(t)
		goto retry
	}
	return nil
}

// checkIteratorFunc checks if sig is a Go 1.23 iterator function signature.
// Returns [keyType, valType] if valid, nil otherwise.
// For 0-value iterators, returns [nil, nil].
// For 1-value iterators, returns [valType, nil].
// For 2-value iterators, returns [keyType, valType].
func checkIteratorFunc(sig *types.Signature) []types.Type {
	// Must have no results
	// Must have exactly 1 parameter (the yield function)
	if sig.Results().Len() != 0 || sig.Params().Len() != 1 {
		return nil
	}
	// The parameter must be a function
	paramType := sig.Params().At(0).Type()
	yieldSig, ok := types.Unalias(paramType).(*types.Signature)
	if !ok {
		return nil
	}
	// yield must return bool
	yieldRets := yieldSig.Results()
	if yieldRets.Len() != 1 {
		return nil
	}
	basic, ok := yieldRets.At(0).Type().(*types.Basic)
	if !ok || basic.Kind() != types.Bool {
		return nil
	}
	// Check yield parameters (0, 1, or 2)
	yieldParams := yieldSig.Params()
	switch yieldParams.Len() {
	case 0:
		return []types.Type{nil, nil}
	case 1:
		return []types.Type{yieldParams.At(0).Type(), nil}
	case 2:
		return []types.Type{yieldParams.At(0).Type(), yieldParams.At(1).Type()}
	}
	return nil
}

func (p *forRangeStmt) checkUdt(cb *CodeBuilder, o *types.Named) ([]types.Type, bool) {
	if enumName, sig := findEnumMethodType(cb, o); sig != nil {
		p.enumName = enumName
		enumRet := sig.Results()
		params := sig.Params()
		switch params.Len() {
		case 0:
			// iter := obj.XGo_Enum()
			// key, val, ok := iter.Next()
		case 1:
			// obj.XGo_Enum(func(key K, val V) { ... })
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
			switch t := typ.(type) {
			case *types.Pointer:
				typ = t.Elem()
				if it, ok := typ.(*types.Named); ok {
					return p.checkUdtEnumRet(cb, it)
				}
			case *types.Named:
				u := cb.getUnderlying(t)
				if sig, ok := u.(*types.Signature); ok {
					if ret := checkIteratorFunc(sig); ret != nil {
						return ret, true
					}
				} else {
					return p.checkUdtEnumRet(cb, t)
				}
			case *types.Signature:
				if ret := checkIteratorFunc(t); ret != nil {
					return ret, true
				}
			}
		}
	}
	return nil, false
}

// checkUdtEnumRet checks if a named type 'it' returned by XGo_Enum()
// implements the iterator pattern with a Next() method.
// Returns the key/value types and true if valid, nil and false otherwise.
func (p *forRangeStmt) checkUdtEnumRet(cb *CodeBuilder, it *types.Named) ([]types.Type, bool) {
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

func findEnumMethodType(cb *CodeBuilder, o *types.Named) (string, mthdSignature) {
	for i, n := 0, o.NumMethods(); i < n; i++ {
		method := o.Method(i)
		if v := method.Name(); v == nameXGoEnum1 || v == nameXGoEnum2 {
			return v, method.Type().(*types.Signature)
		}
	}
	if bti := cb.getBuiltinTI(o); bti != nil {
		return nameXGoEnum1, bti.lookupByName(nameXGoEnum1)
	}
	return "", nil
}

var (
	nameXGoEnum1 = "XGo_Enum"
	nameXGoEnum2 = "Gop_Enum"
)

func (p *forRangeStmt) End(cb *CodeBuilder, src ast.Node) {
	if p.stmt == nil {
		if cb != nil {
			cb.endBlockStmt(&p.old)
		}
		return
	}
	stmts, flows := cb.endBlockStmt(&p.old)
	cb.current.flows |= (flows &^ (flowFlagBreak | flowFlagContinue))
	// Emit pre-range statements (e.g., type assertions) to the outer scope
	// before the for-range statement.
	for _, s := range p.preStmts {
		cb.emitStmt(s)
	}
	emitForRangeStmt(cb, p, stmts, flows)
}

// ----------------------------------------------------------------------------
