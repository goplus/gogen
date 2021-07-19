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
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"reflect"

	"github.com/goplus/gox/internal"
)

const (
	DbgFlagInstruction = 1 << iota
	DbgFlagImport
	DbgFlagAll = DbgFlagInstruction | DbgFlagImport
)

var (
	debug       bool
	debugImport bool
)

func SetDebug(dbgFlags int) {
	debug = (dbgFlags & DbgFlagInstruction) != 0
	debugImport = (dbgFlags & DbgFlagImport) != 0
}

// ----------------------------------------------------------------------------

type codeBlock interface {
	End(cb *CodeBuilder)
}

type codeBlockCtx struct {
	codeBlock
	scope *types.Scope
	base  int
	stmts []ast.Stmt
	label *ast.LabeledStmt
}

type funcBodyCtx struct {
	codeBlockCtx
	fn     *Func
	labels map[string]bool
}

func (p *funcBodyCtx) checkLabels() {
	for name, define := range p.labels {
		if !define {
			log.Panicf("TODO: label name %v is not defined\n", name)
		}
	}
}

func (p *funcBodyCtx) getLabels() map[string]bool {
	if p.labels == nil {
		p.labels = make(map[string]bool)
	}
	return p.labels
}

func (p *funcBodyCtx) useLabel(name string, define bool) {
	labels := p.getLabels()
	exists := labels[name]
	if exists && define {
		log.Panicf("TODO: label name %v exists\n", name)
	}
	if !exists {
		labels[name] = define
	}
}

// CodeBuilder type
type CodeBuilder struct {
	stk     internal.Stack
	current funcBodyCtx
	pkg     *Package
	varDecl *ValueDecl
	closureParamInsts
}

func (p *CodeBuilder) init(pkg *Package) {
	p.pkg = pkg
	p.current.scope = pkg.Types.Scope()
	p.stk.Init()
	p.closureParamInsts.init()
}

// Scope returns current scope.
func (p *CodeBuilder) Scope() *types.Scope {
	return p.current.scope
}

func (p *CodeBuilder) Pkg() *Package {
	return p.pkg
}

func (p *CodeBuilder) startFuncBody(fn *Func, old *funcBodyCtx) *CodeBuilder {
	p.current.fn, old.fn = fn, p.current.fn
	p.startBlockStmt(fn, "func "+fn.Name(), &old.codeBlockCtx)
	scope := p.current.scope
	sig := fn.Type().(*types.Signature)
	insertParams(scope, sig.Params())
	insertParams(scope, sig.Results())
	if recv := sig.Recv(); recv != nil {
		scope.Insert(recv)
	}
	return p
}

func insertParams(scope *types.Scope, params *types.Tuple) {
	for i, n := 0, params.Len(); i < n; i++ {
		v := params.At(i)
		if name := v.Name(); name != "" {
			scope.Insert(v)
		}
	}
}

func (p *CodeBuilder) endFuncBody(old funcBodyCtx) []ast.Stmt {
	p.current.checkLabels()
	p.current.fn = old.fn
	return p.endBlockStmt(old.codeBlockCtx)
}

func (p *CodeBuilder) startBlockStmt(current codeBlock, comment string, old *codeBlockCtx) *CodeBuilder {
	scope := types.NewScope(p.current.scope, token.NoPos, token.NoPos, comment)
	p.current.codeBlockCtx, *old = codeBlockCtx{current, scope, p.stk.Len(), nil, nil}, p.current.codeBlockCtx
	return p
}

func (p *CodeBuilder) endBlockStmt(old codeBlockCtx) []ast.Stmt {
	if p.current.label != nil {
		p.emitStmt(&ast.EmptyStmt{})
	}
	stmts := p.current.stmts
	p.stk.SetLen(p.current.base)
	p.current.codeBlockCtx = old
	return stmts
}

func (p *CodeBuilder) clearBlockStmt() []ast.Stmt {
	stmts := p.current.stmts
	p.current.stmts = nil
	return stmts
}

func (p *CodeBuilder) startStmtAt(stmt ast.Stmt) int {
	idx := len(p.current.stmts)
	p.emitStmt(stmt)
	return idx
}

// Usage:
//   idx := cb.startStmtAt(stmt)
//   ...
//   cb.commitStmt(idx)
func (p *CodeBuilder) commitStmt(idx int) {
	stmts := p.current.stmts
	n := len(stmts) - 1
	if n > idx {
		stmt := stmts[idx]
		copy(stmts[idx:], stmts[idx+1:])
		stmts[n] = stmt
	}
}

func (p *CodeBuilder) emitStmt(stmt ast.Stmt) {
	if p.current.label != nil {
		p.current.label.Stmt = stmt
		stmt, p.current.label = p.current.label, nil
	}
	p.current.stmts = append(p.current.stmts, stmt)
}

func (p *CodeBuilder) startInitExpr(current codeBlock) (old codeBlock) {
	p.current.codeBlock, old = current, p.current.codeBlock
	return
}

func (p *CodeBuilder) endInitExpr(old codeBlock) {
	p.current.codeBlock = old
}

// ReturnErr func
func (p *CodeBuilder) ReturnErr(outer bool) *CodeBuilder {
	if debug {
		log.Println("ReturnErr", outer)
	}
	fn := p.current.fn
	if outer {
		if !fn.isInline() {
			panic("only support ReturnOuterErr in an inline call")
		}
		fn = fn.old.fn
	}
	results := fn.Type().(*types.Signature).Results()
	n := results.Len()
	if n > 0 {
		last := results.At(n - 1)
		if last.Type() == TyError { // last result is error
			err := p.stk.Pop()
			for i := 0; i < n-1; i++ {
				p.doZeroLit(results.At(i).Type(), false)
			}
			p.stk.Push(err)
			p.returnResults(n)
			return p
		}
	}
	panic("TODO: last result type isn't an error")
}

func (p *CodeBuilder) returnResults(n int) {
	var rets []ast.Expr
	if n > 0 {
		args := p.stk.GetArgs(n)
		rets = make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			rets[i] = args[i].Val
		}
		p.stk.PopN(n)
	}
	p.emitStmt(&ast.ReturnStmt{Results: rets})
}

// Return func
func (p *CodeBuilder) Return(n int) *CodeBuilder {
	if debug {
		log.Println("Return", n)
	}
	fn := p.current.fn
	results := fn.Type().(*types.Signature).Results()
	if err := matchFuncResults(p.pkg, p.stk.GetArgs(n), results); err != nil {
		panic(err)
	}
	if fn.isInline() {
		for i := n - 1; i >= 0; i-- {
			key := closureParamInst{fn, results.At(i)}
			elem := p.stk.Pop()
			p.doVarRef(p.paramInsts[key], false)
			p.stk.Push(elem)
			p.doAssign(1, nil, false)
		}
		p.Goto(p.getEndingLabel(fn))
	} else {
		p.returnResults(n)
	}
	return p
}

// Call func
func (p *CodeBuilder) Call(n int, ellipsis ...bool) *CodeBuilder {
	args := p.stk.GetArgs(n)
	n++
	fn := p.stk.Get(-n)
	var flags InstrFlags
	if ellipsis != nil && ellipsis[0] {
		flags = InstrFlagEllipsis
	}
	if debug {
		log.Println("Call", n-1, int(flags))
	}
	ret := toFuncCall(p.pkg, fn, args, flags)
	p.stk.Ret(n, ret)
	return p
}

type closureParamInst struct {
	inst  *Func
	param *types.Var
}

type closureParamInsts struct {
	paramInsts map[closureParamInst]*types.Var
}

func (p *closureParamInsts) init() {
	p.paramInsts = make(map[closureParamInst]*types.Var)
}

func (p *CodeBuilder) getEndingLabel(fn *Func) string {
	key := closureParamInst{fn, nil}
	if v, ok := p.paramInsts[key]; ok {
		return v.Name()
	}
	ending := p.pkg.autoName()
	p.paramInsts[key] = types.NewParam(token.NoPos, nil, ending, nil)
	return ending
}

func (p *CodeBuilder) needEndingLabel(fn *Func) (string, bool) {
	key := closureParamInst{fn, nil}
	if v, ok := p.paramInsts[key]; ok {
		return v.Name(), true
	}
	return "", false
}

func (p *Func) inlineClosureEnd(cb *CodeBuilder) {
	if ending, ok := cb.needEndingLabel(p); ok {
		cb.Label(ending)
	}
	sig := p.Type().(*types.Signature)
	cb.emitStmt(&ast.BlockStmt{List: cb.endFuncBody(p.old)})
	cb.stk.PopN(p.getInlineCallArity())
	results := sig.Results()
	for i, n := 0, results.Len(); i < n; i++ { // return results & clean env
		key := closureParamInst{p, results.At(i)}
		cb.pushVal(cb.paramInsts[key])
		delete(cb.paramInsts, key)
	}
	for i, n := 0, getParamLen(sig); i < n; i++ { // clean env
		key := closureParamInst{p, getParam(sig, i)}
		delete(cb.paramInsts, key)
	}
}

func (p *Func) getInlineCallArity() int {
	return int(p.Pos() &^ closureFlagInline)
}

func makeInlineCall(arity int) closureType {
	return closureFlagInline | closureType(arity)
}

// CallInlineClosureStart func
func (p *CodeBuilder) CallInlineClosureStart(sig *types.Signature, arity int, ellipsis bool) *CodeBuilder {
	if debug {
		log.Println("CallInlineClosureStart", arity, ellipsis)
	}
	pkg := p.pkg
	closure := pkg.newClosure(sig, makeInlineCall(arity))
	results := sig.Results()
	for i, n := 0, results.Len(); i < n; i++ {
		p.emitVar(pkg, closure, results.At(i), false)
	}
	p.startFuncBody(closure, &closure.old)
	args := p.stk.GetArgs(arity)
	if err := matchFuncType(pkg, args, ellipsis, sig); err != nil {
		panic(err)
	}
	n1 := getParamLen(sig) - 1
	if sig.Variadic() && !ellipsis {
		p.SliceLit(getParam(sig, n1).Type().(*types.Slice), arity-n1)
	}
	for i := n1; i >= 0; i-- {
		p.emitVar(pkg, closure, getParam(sig, i), true)
	}
	return p
}

func (p *CodeBuilder) emitVar(pkg *Package, closure *Func, param *types.Var, withInit bool) {
	name := pkg.autoName()
	if withInit {
		p.NewVarStart(param.Type(), name).EndInit(1)
	} else {
		p.NewVar(param.Type(), name)
	}
	key := closureParamInst{closure, param}
	p.paramInsts[key] = p.current.scope.Lookup(name).(*types.Var)
}

// NewClosure func
func (p *CodeBuilder) NewClosure(params, results *Tuple, variadic bool) *Func {
	sig := types.NewSignature(nil, params, results, variadic)
	return p.NewClosureWith(sig)
}

// NewClosureWith func
func (p *CodeBuilder) NewClosureWith(sig *types.Signature) *Func {
	if debug {
		t := sig.Params()
		for i, n := 0, t.Len(); i < n; i++ {
			v := t.At(i)
			if _, ok := v.Type().(*unboundType); ok {
				panic("TODO: can't use auto type in func parameters")
			}
		}
	}
	return p.pkg.newClosure(sig, closureNormal)
}

// NewType func
func (p *CodeBuilder) NewType(name string) *TypeDecl {
	return p.pkg.NewType(name)
}

// AliasType func
func (p *CodeBuilder) AliasType(name string, typ types.Type) *types.Named {
	return p.pkg.AliasType(name, typ)
}

// NewConstStart func
func (p *CodeBuilder) NewConstStart(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewConstStart", names)
	}
	return p.pkg.newValueDecl(token.CONST, typ, names...).InitStart(p.pkg)
}

// NewVar func
func (p *CodeBuilder) NewVar(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewVar", names)
	}
	p.pkg.newValueDecl(token.VAR, typ, names...)
	return p
}

// NewVarStart func
func (p *CodeBuilder) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewVarStart", names)
	}
	return p.pkg.newValueDecl(token.VAR, typ, names...).InitStart(p.pkg)
}

// DefineVarStart func
func (p *CodeBuilder) DefineVarStart(names ...string) *CodeBuilder {
	if debug {
		log.Println("DefineVarStart", names)
	}
	return p.pkg.newValueDecl(token.DEFINE, nil, names...).InitStart(p.pkg)
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(name string, pv **types.Var) *CodeBuilder {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	if debug {
		log.Println("NewAutoVar", name)
	}
	p.emitStmt(stmt)
	typ := &unboundType{ptypes: []*ast.Expr{&spec.Type}}
	*pv = types.NewVar(token.NoPos, p.pkg.Types, name, typ)
	if p.current.scope.Insert(*pv) != nil {
		log.Panicln("TODO: variable already defined -", name)
	}
	return p
}

// VarRef func: p.VarRef(nil) means underscore (_)
func (p *CodeBuilder) VarRef(ref interface{}) *CodeBuilder {
	return p.doVarRef(ref, true)
}

func (p *CodeBuilder) doVarRef(ref interface{}, allowDebug bool) *CodeBuilder {
	if ref == nil {
		if allowDebug && debug {
			log.Println("VarRef _")
		}
		p.stk.Push(internal.Elem{
			Val: underscore, // _
		})
	} else {
		switch v := ref.(type) {
		case *types.Var:
			if allowDebug && debug {
				log.Println("VarRef", v.Name())
			}
			fn := p.current.fn
			if fn != nil && fn.isInline() { // is in an inline call
				key := closureParamInst{fn, v}
				if arg, ok := p.paramInsts[key]; ok { // replace param with arg
					v = arg
				}
			}
			p.stk.Push(internal.Elem{
				Val:  toObjectExpr(p.pkg, v),
				Type: &refType{typ: v.Type()},
			})
		default:
			log.Panicln("TODO: VarRef", reflect.TypeOf(ref))
		}
	}
	return p
}

// None func
func (p *CodeBuilder) None() *CodeBuilder {
	if debug {
		log.Println("None")
	}
	p.stk.Push(internal.Elem{})
	return p
}

// ZeroLit func
func (p *CodeBuilder) ZeroLit(typ types.Type) *CodeBuilder {
	return p.doZeroLit(typ, true)
}

func (p *CodeBuilder) doZeroLit(typ types.Type, allowDebug bool) *CodeBuilder {
	if allowDebug && debug {
		log.Println("ZeroLit")
	}
	switch t := typ.(type) {
	case *types.Basic:
		switch kind := t.Kind(); kind {
		case types.Bool:
			return p.Val(false)
		case types.String:
			return p.Val("")
		case types.UnsafePointer:
			return p.Val(nil)
		default:
			return p.Val(0)
		}
	case *types.Interface:
		return p.Val(nil)
	case *types.Map:
		return p.Val(nil)
	case *types.Slice:
		return p.Val(nil)
	case *types.Pointer:
		return p.Val(nil)
	case *types.Chan:
		return p.Val(nil)
	}
	ret := &ast.CompositeLit{}
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound == nil {
			t.ptypes = append(t.ptypes, &ret.Type)
		} else {
			typ = t.tBound
			ret.Type = toType(p.pkg, typ)
		}
	default:
		ret.Type = toType(p.pkg, typ)
	}
	p.stk.Push(internal.Elem{Type: typ, Val: ret})
	return p
}

// MapLit func
func (p *CodeBuilder) MapLit(typ types.Type, arity int) *CodeBuilder {
	if debug {
		log.Println("MapLit", typ, arity)
	}
	var t *types.Map
	var typExpr ast.Expr
	if typ != nil {
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(p.pkg, tt)
			t = tt.Underlying().(*types.Map)
		case *types.Map:
			typExpr = toMapType(p.pkg, tt)
			t = tt
		default:
			log.Panicln("TODO: MapLit: typ isn't a map type -", reflect.TypeOf(typ))
		}
	}
	if arity == 0 {
		if t == nil {
			t = types.NewMap(types.Typ[types.String], TyEmptyInterface)
			typ = t
			typExpr = toMapType(p.pkg, t)
		}
		ret := &ast.CompositeLit{Type: typExpr}
		p.stk.Push(internal.Elem{Type: typ, Val: ret})
		return p
	}
	if (arity & 1) != 0 {
		panic("TODO: MapLit - invalid arity")
	}
	var key, val types.Type
	var args = p.stk.GetArgs(arity)
	var check = (t != nil)
	if check {
		key, val = t.Key(), t.Elem()
	} else {
		key = boundElementType(args, 0, arity, 2)
		val = boundElementType(args, 1, arity, 2)
		t = types.NewMap(types.Default(key), types.Default(val))
		typ = t
		typExpr = toMapType(p.pkg, t)
	}
	elts := make([]ast.Expr, arity>>1)
	for i := 0; i < arity; i += 2 {
		elts[i>>1] = &ast.KeyValueExpr{Key: args[i].Val, Value: args[i+1].Val}
		if check {
			if !AssignableTo(args[i].Type, key) {
				log.Panicf("TODO: MapLit - can't assign %v to %v\n", args[i].Type, key)
			} else if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: MapLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
		}
	}
	p.stk.Ret(arity, internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

func toBoundArrayLen(pkg *Package, elts []internal.Elem, arity int) int {
	n := -1
	max := -1
	for i := 0; i < arity; i += 2 {
		if elts[i].Val != nil {
			n = toIntVal(elts[i].CVal)
		} else {
			n++
		}
		if max < n {
			max = n
		}
	}
	return max + 1
}

func toIntVal(cval constant.Value) int {
	if cval != nil {
		if v, ok := constant.Int64Val(cval); ok {
			return int(v)
		}
	}
	panic("TODO: not a constant integer or too large")
}

func toLitElemExpr(args []internal.Elem, i int) ast.Expr {
	key := args[i].Val
	if key == nil {
		return args[i+1].Val
	}
	return &ast.KeyValueExpr{Key: key, Value: args[i+1].Val}
}

// SliceLit func
func (p *CodeBuilder) SliceLit(typ types.Type, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var keyValMode = (keyVal != nil && keyVal[0])
	if debug {
		log.Println("SliceLit", typ, arity, keyValMode)
	}
	var t *types.Slice
	var typExpr ast.Expr
	if typ != nil {
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(p.pkg, tt)
			t = tt.Underlying().(*types.Slice)
		case *types.Slice:
			typExpr = toSliceType(p.pkg, tt)
			t = tt
		default:
			log.Panicln("TODO: SliceLit: typ isn't a slice type -", reflect.TypeOf(typ))
		}
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			panic("TODO: SliceLit - invalid arity")
		}
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		n := arity >> 1
		elts = make([]ast.Expr, n)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: SliceLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
			elts[i>>1] = toLitElemExpr(args, i)
		}
	} else {
		if arity == 0 {
			if t == nil {
				t = types.NewSlice(TyEmptyInterface)
				typ = t
				typExpr = toSliceType(p.pkg, t)
			}
			p.stk.Push(internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr}})
			return p
		}
		var val types.Type
		var args = p.stk.GetArgs(arity)
		var check = (t != nil)
		if check {
			val = t.Elem()
		} else {
			val = boundElementType(args, 0, arity, 1)
			t = types.NewSlice(types.Default(val))
			typ = t
			typExpr = toSliceType(p.pkg, t)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if check {
				if !AssignableTo(arg.Type, val) {
					log.Panicf("TODO: SliceLit - can't assign %v to %v\n", arg.Type, val)
				}
			}
		}
	}
	p.stk.Ret(arity, internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// ArrayLit func
func (p *CodeBuilder) ArrayLit(typ types.Type, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var keyValMode = (keyVal != nil && keyVal[0])
	if debug {
		log.Println("ArrayLit", typ, arity, keyValMode)
	}
	var t *types.Array
	var typExpr ast.Expr
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(p.pkg, tt)
		t = tt.Underlying().(*types.Array)
	case *types.Array:
		typExpr = toArrayType(p.pkg, tt)
		t = tt
	default:
		log.Panicln("TODO: ArrayLit: typ isn't a array type -", reflect.TypeOf(typ))
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			panic("TODO: ArrayLit - invalid arity")
		}
		args := p.stk.GetArgs(arity)
		max := toBoundArrayLen(p.pkg, args, arity)
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(max))
			typ = t
		} else if int(n) < max {
			log.Panicf("TODO: array index %v out of bounds [0:%v]\n", max, n)
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: ArrayLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
			elts[i>>1] = toLitElemExpr(args, i)
		}
	} else {
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(arity))
			typ = t
		} else if int(n) < arity {
			log.Panicf("TODO: array index %v out of bounds [0:%v]\n", arity, n)
		}
		args := p.stk.GetArgs(arity)
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if !AssignableTo(arg.Type, val) {
				log.Panicf("TODO: ArrayLit - can't assign %v to %v\n", arg.Type, val)
			}
		}
	}
	p.stk.Ret(arity, internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool) *CodeBuilder {
	if debug {
		log.Println("StructLit", keyVal)
	}
	var t *types.Struct
	var typExpr ast.Expr
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(p.pkg, tt)
		t = tt.Underlying().(*types.Struct)
	case *types.Struct:
		typExpr = toStructType(p.pkg, tt)
		t = tt
	default:
		log.Panicln("TODO: StructLit: typ isn't a struct type -", reflect.TypeOf(typ))
	}
	var elts []ast.Expr
	var n = t.NumFields()
	var args = p.stk.GetArgs(arity)
	if keyVal {
		if (arity & 1) != 0 {
			panic("TODO: StructLit - invalid arity")
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			idx := toIntVal(args[i].CVal)
			if idx >= n {
				panic("TODO: invalid struct field index")
			}
			eltTy := t.Field(idx).Type()
			if !AssignableTo(args[i+1].Type, eltTy) {
				log.Panicf("TODO: StructLit - can't assign %v to %v\n", args[i+1].Type, eltTy)
			}
			eltName := ident(t.Field(idx).Name())
			elts[i>>1] = &ast.KeyValueExpr{Key: eltName, Value: args[i+1].Val}
		}
	} else if arity != n {
		if arity != 0 {
			log.Panicln("TODO: too few values in struct")
		}
	} else {
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			eltTy := t.Field(i).Type()
			if !AssignableTo(arg.Type, eltTy) {
				log.Panicf("TODO: StructLit - can't assign %v to %v\n", arg.Type, eltTy)
			}
		}
	}
	p.stk.Ret(arity, internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// Slice func
func (p *CodeBuilder) Slice(slice3 bool) *CodeBuilder { // a[i:j:k]
	if debug {
		log.Println("Slice", slice3)
	}
	n := 3
	if slice3 {
		n++
	}
	args := p.stk.GetArgs(n)
	x := args[0]
	typ := x.Type
	switch t := typ.(type) {
	case *types.Slice:
		// nothing to do
	case *types.Basic:
		if t.Kind() == types.String || t.Kind() == types.UntypedString {
			if slice3 {
				log.Panicln("TODO: invalid operation `???` (3-index slice of string)")
			}
		} else {
			log.Panicln("TODO: invalid operation: slice of", typ)
		}
	case *types.Array:
		typ = types.NewSlice(t.Elem())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			typ = types.NewSlice(tt.Elem())
		} else {
			log.Panicln("TODO: invalid operation: slice of non-array pointer -", typ)
		}
	}
	var exprMax ast.Expr
	if slice3 {
		exprMax = args[3].Val
	}
	// TODO: check type
	elem := internal.Elem{
		Val: &ast.SliceExpr{
			X: x.Val, Low: args[1].Val, High: args[2].Val, Max: exprMax, Slice3: slice3,
		},
		Type: typ,
	}
	p.stk.Ret(n, elem)
	return p
}

// Index func
func (p *CodeBuilder) Index(nidx int, twoValue bool) *CodeBuilder {
	if debug {
		log.Println("Index", nidx, twoValue)
	}
	if nidx != 1 {
		panic("TODO: IndexGet doesn't support a[i, j...] yet")
	}
	args := p.stk.GetArgs(2)
	typs, allowTwoValue := getIdxValTypes(args[0].Type)
	var tyRet types.Type
	if twoValue { // elem, ok = a[key]
		if !allowTwoValue {
			panic("TODO: doesn't return twoValue")
		}
		pkg := p.pkg
		tyRet = types.NewTuple(pkg.NewParam("", typs[1]), pkg.NewParam("", types.Typ[types.Bool]))
	} else { // elem = a[key]
		tyRet = typs[1]
	}
	elem := internal.Elem{
		Val:  &ast.IndexExpr{X: args[0].Val, Index: args[1].Val},
		Type: tyRet,
	}
	// TODO: check index type
	p.stk.Ret(2, elem)
	return p
}

// IndexRef func
func (p *CodeBuilder) IndexRef(nidx int) *CodeBuilder {
	if debug {
		log.Println("IndexRef", nidx)
	}
	if nidx != 1 {
		panic("TODO: IndexRef doesn't support a[i, j...] = val yet")
	}
	args := p.stk.GetArgs(2)
	typ := args[0].Type
	elemRef := internal.Elem{
		Val: &ast.IndexExpr{X: args[0].Val, Index: args[1].Val},
	}
	if t, ok := typ.(*unboundType); ok {
		tyMapElem := &unboundMapElemType{key: args[1].Type, typ: t}
		elemRef.Type = &refType{typ: tyMapElem}
	} else {
		typs, _ := getIdxValTypes(typ)
		elemRef.Type = &refType{typ: typs[1]}
		// TODO: check index type
	}
	p.stk.Ret(2, elemRef)
	return p
}

func getIdxValTypes(typ types.Type) ([]types.Type, bool) {
	switch t := typ.(type) {
	case *types.Slice:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}, true
	case *types.Array:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Pointer:
		if e, ok := t.Elem().(*types.Array); ok {
			return []types.Type{tyInt, e.Elem()}, false
		}
	}
	log.Panicln("TODO: can't index of type", typ)
	return nil, false
}

var (
	tyInt = types.Typ[types.Int]
)

// Typ func
func (p *CodeBuilder) Typ(typ types.Type) *CodeBuilder {
	if debug {
		log.Println("Typ", typ)
	}
	p.stk.Push(internal.Elem{
		Val:  toType(p.pkg, typ),
		Type: NewTypeType(typ),
	})
	return p
}

// Val func
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
	if debug {
		if o, ok := v.(types.Object); ok {
			log.Println("Val", o.Name())
		} else {
			log.Println("Val", v)
		}
	}
	fn := p.current.fn
	if fn != nil && fn.isInline() { // is in an inline call
		if param, ok := v.(*types.Var); ok {
			key := closureParamInst{fn, param}
			if arg, ok := p.paramInsts[key]; ok { // replace param with arg
				v = arg
			}
		}
	}
	return p.pushVal(v)
}

func (p *CodeBuilder) pushVal(v interface{}) *CodeBuilder {
	p.stk.Push(toExpr(p.pkg, v))
	return p
}

// Star func
func (p *CodeBuilder) Star() *CodeBuilder {
	if debug {
		log.Println("Star")
	}
	arg := p.stk.Get(-1)
	ret := internal.Elem{Val: &ast.StarExpr{X: arg.Val}}
	switch t := arg.Type.(type) {
	case *TypeType:
		t.typ = types.NewPointer(t.typ)
		ret.Type = arg.Type
	case *types.Pointer:
		ret.Type = t.Elem()
	default:
		log.Panicln("TODO: can't use *X to a non pointer value -", t)
	}
	p.stk.Ret(1, ret)
	return p
}

// Elem func
func (p *CodeBuilder) Elem() *CodeBuilder {
	if debug {
		log.Println("Elem")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		log.Panicln("TODO: not a pointer")
	}
	p.stk.Ret(1, internal.Elem{Val: &ast.StarExpr{X: arg.Val}, Type: t.Elem()})
	return p
}

// ElemRef func
func (p *CodeBuilder) ElemRef() *CodeBuilder {
	if debug {
		log.Println("ElemRef")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		log.Panicln("TODO: not a pointer")
	}
	p.stk.Ret(1, internal.Elem{
		Val: &ast.StarExpr{X: arg.Val}, Type: &refType{typ: t.Elem()},
	})
	return p
}

// MemberRef func
func (p *CodeBuilder) MemberRef(name string) *CodeBuilder {
	if debug {
		log.Println("MemberRef", name)
	}
	arg := p.stk.Get(-1)
	switch o := indirect(arg.Type).(type) {
	case *types.Named:
		if struc, ok := o.Underlying().(*types.Struct); ok {
			p.fieldRef(arg.Val, struc, name)
		} else {
			panic("TODO: member not found - " + name)
		}
	case *types.Struct:
		p.fieldRef(arg.Val, o, name)
	default:
		log.Panicln("TODO: MemberRef - unexpected type:", o)
	}
	return p
}

func (p *CodeBuilder) fieldRef(x ast.Expr, struc *types.Struct, name string) {
	if t := structFieldType(struc, name); t != nil {
		p.stk.Ret(1, internal.Elem{
			Val:  &ast.SelectorExpr{X: x, Sel: ident(name)},
			Type: &refType{typ: t},
		})
	} else {
		panic("TODO: member not found - " + name)
	}
}

func structFieldType(o *types.Struct, name string) types.Type {
	for i, n := 0, o.NumFields(); i < n; i++ {
		fld := o.Field(i)
		if fld.Name() == name {
			return fld.Type()
		}
	}
	return nil
}

const (
	MFlagMethod = 1 << iota
	MFlagVar
)

// MemberVal func
func (p *CodeBuilder) MemberVal(name string, mflags ...*int) *CodeBuilder {
	if debug {
		log.Println("MemberVal", name)
	}
	arg := p.stk.Get(-1)
	switch o := indirect(arg.Type).(type) {
	case *types.Named:
		for i, n := 0, o.NumMethods(); i < n; i++ {
			method := o.Method(i)
			if method.Name() == name {
				p.stk.Ret(1, internal.Elem{
					Val:  &ast.SelectorExpr{X: arg.Val, Sel: ident(name)},
					Type: methodTypeOf(method.Type()),
				})
				assignMFlag(mflags, MFlagMethod)
				return p
			}
		}
		if struc, ok := o.Underlying().(*types.Struct); ok {
			if t := structFieldType(struc, name); t != nil {
				p.stk.Ret(1, internal.Elem{
					Val:  &ast.SelectorExpr{X: arg.Val, Sel: ident(name)},
					Type: t,
				})
				assignMFlag(mflags, MFlagVar)
				return p
			}
		}
	case *types.Struct:
		if t := structFieldType(o, name); t != nil {
			p.stk.Ret(1, internal.Elem{
				Val:  &ast.SelectorExpr{X: arg.Val, Sel: ident(name)},
				Type: t,
			})
			assignMFlag(mflags, MFlagVar)
			return p
		}
	default:
		log.Panicln("TODO: MemberVal - unexpected type:", o)
	}
	if mflags == nil {
		panic("TODO: member not found - " + name)
	}
	return p
}

func assignMFlag(ret []*int, mflag int) {
	if ret != nil {
		*ret[0] = mflag
	}
}

func methodTypeOf(typ types.Type) types.Type {
	switch sig := typ.(type) {
	case *types.Signature:
		return types.NewSignature(nil, sig.Params(), sig.Results(), sig.Variadic())
	default:
		panic("TODO: methodTypeOf")
	}
}

func indirect(typ types.Type) types.Type {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	return typ
}

// Assign func
func (p *CodeBuilder) Assign(lhs int, rhs ...int) *CodeBuilder {
	return p.doAssign(lhs, rhs, true)
}

func (p *CodeBuilder) doAssign(lhs int, v []int, allowDebug bool) *CodeBuilder {
	var rhs int
	if v != nil {
		rhs = v[0]
	} else {
		rhs = lhs
	}
	if allowDebug && debug {
		log.Println("Assign", lhs, rhs)
	}
	args := p.stk.GetArgs(lhs + rhs)
	stmt := &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: make([]ast.Expr, lhs),
		Rhs: make([]ast.Expr, rhs),
	}
	pkg := p.pkg
	if lhs == rhs {
		for i := 0; i < lhs; i++ {
			matchAssignType(pkg, args[i].Type, args[lhs+i].Type)
			stmt.Lhs[i] = args[i].Val
			stmt.Rhs[i] = args[lhs+i].Val
		}
	} else if rhs == 1 {
		rhsVals, ok := args[lhs].Type.(*types.Tuple)
		if !ok || lhs != rhsVals.Len() {
			panic("TODO: unmatch assignment")
		}
		for i := 0; i < lhs; i++ {
			matchAssignType(pkg, args[i].Type, rhsVals.At(i).Type())
			stmt.Lhs[i] = args[i].Val
		}
		stmt.Rhs[0] = args[lhs].Val
	} else {
		panic("TODO: unmatch assignment")
	}
	p.emitStmt(stmt)
	p.stk.PopN(lhs + rhs)
	return p
}

func lookupMethod(t *types.Named, name string) types.Object {
	for i, n := 0, t.NumMethods(); i < n; i++ {
		m := t.Method(i)
		if m.Name() == name {
			return m
		}
	}
	return nil
}

func callOpFunc(pkg *Package, name string, args []internal.Elem, flags InstrFlags) (ret internal.Elem) {
	if t, ok := args[0].Type.(*types.Named); ok {
		op := lookupMethod(t, name)
		if op != nil {
			fn := internal.Elem{
				Val:  &ast.SelectorExpr{X: args[0].Val, Sel: ident(name)},
				Type: realType(op.Type()),
			}
			return toFuncCall(pkg, fn, args, flags)
		}
	}
	op := pkg.builtin.Scope().Lookup(name)
	if op == nil {
		panic("TODO: operator not matched")
	}
	return toFuncCall(pkg, toObject(pkg, op), args, flags)
}

// BinaryOp func
func (p *CodeBuilder) BinaryOp(op token.Token) *CodeBuilder {
	name := p.pkg.prefix + binaryOps[op]
	args := p.stk.GetArgs(2)
	if args[1].Type == types.Typ[types.UntypedNil] { // arg1 is nil
		p.stk.PopN(1)
		return p.CompareNil(op)
	} else if args[0].Type == types.Typ[types.UntypedNil] { // arg0 is nil
		args[0] = args[1]
		p.stk.PopN(1)
		return p.CompareNil(op)
	}
	if debug {
		log.Println("BinaryOp", op, name)
	}
	ret := callOpFunc(p.pkg, name, args, 0)
	p.stk.Ret(2, ret)
	return p
}

var (
	binaryOps = [...]string{
		token.ADD: "Add", // +
		token.SUB: "Sub", // -
		token.MUL: "Mul", // *
		token.QUO: "Quo", // /
		token.REM: "Rem", // %

		token.AND:     "And",    // &
		token.OR:      "Or",     // |
		token.XOR:     "Xor",    // ^
		token.AND_NOT: "AndNot", // &^
		token.SHL:     "Lsh",    // <<
		token.SHR:     "Rsh",    // >>

		token.LSS: "LT",
		token.LEQ: "LE",
		token.GTR: "GT",
		token.GEQ: "GE",
		token.EQL: "EQ",
		token.NEQ: "NE",
	}
)

// CompareNil func
func (p *CodeBuilder) CompareNil(op token.Token) *CodeBuilder {
	if op != token.EQL && op != token.NEQ {
		panic("TODO: compare nil can only be == or !=")
	}
	if debug {
		log.Println("CompareNil", op)
	}
	arg := p.stk.Get(-1)
	// TODO: type check
	ret := internal.Elem{
		Val:  &ast.BinaryExpr{X: arg.Val, Op: op, Y: identNil},
		Type: types.Typ[types.Bool],
	}
	p.stk.Ret(1, ret)
	return p
}

// UnaryOp func
func (p *CodeBuilder) UnaryOp(op token.Token, twoValue ...bool) *CodeBuilder {
	var flags InstrFlags
	if twoValue != nil && twoValue[0] {
		flags = InstrFlagTwoValue
	}
	name := p.pkg.prefix + unaryOps[op]
	if debug {
		log.Println("UnaryOp", op, flags, name)
	}
	ret := callOpFunc(p.pkg, name, p.stk.GetArgs(1), flags)
	p.stk.Ret(1, ret)
	return p
}

var (
	unaryOps = [...]string{
		token.SUB:   "Neg",
		token.XOR:   "Not",
		token.ARROW: "Recv",
		token.AND:   "Addr",
	}
)

// IncDec func
func (p *CodeBuilder) IncDec(op token.Token) *CodeBuilder {
	if debug {
		log.Println("IncDec", op)
	}
	pkg := p.pkg
	args := p.stk.GetArgs(1)
	name := pkg.prefix + incdecOps[op]
	fn := pkg.builtin.Scope().Lookup(name)
	if fn == nil {
		panic("TODO: operator not matched")
	}
	switch t := fn.Type().(type) {
	case *instructionType:
		if _, err := t.instr.Call(pkg, args, token.NoPos); err != nil {
			panic(err)
		}
	default:
		panic("TODO: IncDec not found?")
	}
	p.stk.Pop()
	return p
}

var (
	incdecOps = [...]string{
		token.INC: "Inc",
		token.DEC: "Dec",
	}
)

// Send func
func (p *CodeBuilder) Send() *CodeBuilder {
	if debug {
		log.Println("Send")
	}
	val := p.stk.Pop()
	ch := p.stk.Pop()
	// TODO: check types
	p.emitStmt(&ast.SendStmt{Chan: ch.Val, Value: val.Val})
	return p
}

// Defer func
func (p *CodeBuilder) Defer() *CodeBuilder {
	if debug {
		log.Println("Defer")
	}
	arg := p.stk.Pop()
	call, ok := arg.Val.(*ast.CallExpr)
	if !ok {
		panic("TODO: please use defer callExpr()")
	}
	p.emitStmt(&ast.DeferStmt{Call: call})
	return p
}

// Go func
func (p *CodeBuilder) Go() *CodeBuilder {
	if debug {
		log.Println("Go")
	}
	arg := p.stk.Pop()
	call, ok := arg.Val.(*ast.CallExpr)
	if !ok {
		panic("TODO: please use go callExpr()")
	}
	p.emitStmt(&ast.GoStmt{Call: call})
	return p
}

// If func
func (p *CodeBuilder) If() *CodeBuilder {
	if debug {
		log.Println("If")
	}
	stmt := &ifStmt{}
	p.startBlockStmt(stmt, "if statement", &stmt.old)
	return p
}

// Then func
func (p *CodeBuilder) Then() *CodeBuilder {
	if debug {
		log.Println("Then")
	}
	if p.stk.Len() == p.current.base {
		panic("use None() for empty expr")
	}
	if flow, ok := p.current.codeBlock.(controlFlow); ok {
		flow.Then(p)
		return p
	}
	panic("use if..then or switch..then please")
}

// Else func
func (p *CodeBuilder) Else() *CodeBuilder {
	if debug {
		log.Println("Else")
	}
	if flow, ok := p.current.codeBlock.(*ifStmt); ok {
		flow.Else(p)
		return p
	}
	panic("use if..else please")
}

// Switch func
func (p *CodeBuilder) Switch() *CodeBuilder {
	if debug {
		log.Println("Switch")
	}
	stmt := &switchStmt{}
	p.startBlockStmt(stmt, "switch statement", &stmt.old)
	return p
}

// Case func
func (p *CodeBuilder) Case(n int) *CodeBuilder { // n=0 means default case
	if debug {
		log.Println("Case", n)
	}
	if flow, ok := p.current.codeBlock.(*switchStmt); ok {
		flow.Case(p, n)
		return p
	}
	panic("use switch..case please")
}

// Label func
func (p *CodeBuilder) Label(name string) *CodeBuilder {
	if debug {
		log.Println("Label", name)
	}
	p.current.useLabel(name, true)
	p.current.label = &ast.LabeledStmt{Label: ident(name)}
	return p
}

// Goto func
func (p *CodeBuilder) Goto(name string) *CodeBuilder {
	if debug {
		log.Println("Goto", name)
	}
	p.current.useLabel(name, false)
	p.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: ident(name)})
	return p
}

// Break func
func (p *CodeBuilder) Break(name string) *CodeBuilder {
	if debug {
		log.Println("Break", name)
	}
	if name != "" {
		p.current.useLabel(name, false)
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.BREAK, Label: ident(name)})
	return p
}

// Continue func
func (p *CodeBuilder) Continue(name string) *CodeBuilder {
	if debug {
		log.Println("Continue", name)
	}
	if name != "" {
		p.current.useLabel(name, false)
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.CONTINUE, Label: ident(name)})
	return p
}

// Fallthrough func
func (p *CodeBuilder) Fallthrough() *CodeBuilder {
	if debug {
		log.Println("Fallthrough")
	}
	if flow, ok := p.current.codeBlock.(*caseStmt); ok {
		flow.Fallthrough(p)
		return p
	}
	panic("please use fallthrough in case statement")
}

// For func
func (p *CodeBuilder) For() *CodeBuilder {
	if debug {
		log.Println("For")
	}
	stmt := &forStmt{}
	p.startBlockStmt(stmt, "for statement", &stmt.old)
	return p
}

// Post func
func (p *CodeBuilder) Post() *CodeBuilder {
	if debug {
		log.Println("Post")
	}
	if flow, ok := p.current.codeBlock.(*forStmt); ok {
		flow.Post(p)
		return p
	}
	panic("please use Post() in for statement")
}

func (p *CodeBuilder) ForRange(names ...string) *CodeBuilder {
	if debug {
		log.Println("ForRange", names)
	}
	stmt := &forRangeStmt{names: names}
	p.startBlockStmt(stmt, "for range statement", &stmt.old)
	return p
}

// RangeAssignThen func
func (p *CodeBuilder) RangeAssignThen() *CodeBuilder {
	if debug {
		log.Println("RangeAssignThen")
	}
	if flow, ok := p.current.codeBlock.(*forRangeStmt); ok {
		flow.RangeAssignThen(p)
		return p
	}
	panic("please use RangeAssignThen() in for range statement")
}

// EndStmt func
func (p *CodeBuilder) EndStmt() *CodeBuilder {
	n := p.stk.Len() - p.current.base
	if n > 0 {
		if n != 1 {
			panic("syntax error: unexpected newline, expecting := or = or comma")
		}
		stmt := &ast.ExprStmt{X: p.stk.Pop().Val}
		p.emitStmt(stmt)
	}
	return p
}

// End func
func (p *CodeBuilder) End() *CodeBuilder {
	if debug {
		log.Println("End")
		if p.stk.Len() > p.current.base {
			panic("forget to call EndStmt()?")
		}
	}
	p.current.End(p)
	return p
}

// EndInit func
func (p *CodeBuilder) EndInit(n int) *CodeBuilder {
	if debug {
		log.Println("EndInit", n)
	}
	p.varDecl = p.varDecl.EndInit(p, n)
	return p
}

// ----------------------------------------------------------------------------
