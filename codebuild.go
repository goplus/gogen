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
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"math/big"
	"reflect"
	"strconv"

	"github.com/goplus/gox/internal"
	"github.com/goplus/gox/internal/go/printer"
)

const (
	DbgFlagInstruction = 1 << iota
	DbgFlagImport
	DbgFlagMatch
	DbgFlagComments
	DbgFlagAll = DbgFlagInstruction | DbgFlagImport | DbgFlagMatch | DbgFlagComments
)

var (
	debugInstr    bool
	debugMatch    bool
	debugImport   bool
	debugComments bool
)

func SetDebug(dbgFlags int) {
	debugInstr = (dbgFlags & DbgFlagInstruction) != 0
	debugImport = (dbgFlags & DbgFlagImport) != 0
	debugMatch = (dbgFlags & DbgFlagMatch) != 0
	debugComments = (dbgFlags & DbgFlagComments) != 0
}

func getSrc(node []ast.Node) ast.Node {
	if node != nil {
		return node[0]
	}
	return nil
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
	labels map[string]*label
}

type label struct {
	at   *token.Position
	refs []*token.Position
}

func (p *funcBodyCtx) checkLabels(cb *CodeBuilder) {
	for name, l := range p.labels {
		if l.at == nil {
			for _, ref := range l.refs {
				cb.handleErr(cb.newCodeError(ref, fmt.Sprintf("label %s is not defined", name)))
			}
		} else if l.refs == nil {
			cb.handleErr(cb.newCodeError(l.at, fmt.Sprintf("label %s defined and not used", name)))
		}
	}
}

func (p *funcBodyCtx) getLabel(name string) *label {
	if p.labels == nil {
		p.labels = make(map[string]*label)
	}
	l, ok := p.labels[name]
	if !ok {
		l = new(label)
		p.labels[name] = l
	}
	return l
}

func (p *funcBodyCtx) useLabel(cb *CodeBuilder, name string, pos token.Position) {
	l := p.getLabel(name)
	l.refs = append(l.refs, &pos)
}

func (p *funcBodyCtx) defineLabel(cb *CodeBuilder, name string, pos token.Position) {
	l := p.getLabel(name)
	if l.at != nil {
		cb.panicCodeErrorf(&pos, "label %s already defined at %v", name, *l.at)
	}
	l.at = &pos
}

type CodeError struct {
	Msg   string
	Pos   *token.Position
	Scope *types.Scope
	Func  *Func
}

func (p *CodeError) Error() string {
	if p.Pos != nil {
		return fmt.Sprintf("%v %s", *p.Pos, p.Msg)
	}
	return p.Msg
}

// CodeBuilder type
type CodeBuilder struct {
	stk       internal.Stack
	current   funcBodyCtx
	comments  *ast.CommentGroup
	pkg       *Package
	varDecl   *ValueDecl
	interp    NodeInterpreter
	handleErr func(err error)
	closureParamInsts
	commentOnce bool
}

func (p *CodeBuilder) init(pkg *Package) {
	conf := pkg.conf
	p.pkg = pkg
	p.handleErr = conf.HandleErr
	if p.handleErr == nil {
		p.handleErr = defaultHandleErr
	}
	p.interp = conf.NodeInterpreter
	if p.interp == nil {
		p.interp = nodeInterp{}
	}
	p.current.scope = pkg.Types.Scope()
	p.stk.Init()
	p.closureParamInsts.init()
}

func defaultHandleErr(err error) {
	panic(err)
}

type nodeInterp struct{}

func (p nodeInterp) Position(pos token.Pos) (ret token.Position) {
	return
}

func (p nodeInterp) Caller(expr ast.Node) string {
	return "the function call"
}

func (p nodeInterp) LoadExpr(expr ast.Node) (src string, pos token.Position) {
	return
}

func (p *CodeBuilder) position(pos token.Pos) (ret token.Position) {
	return p.interp.Position(pos)
}

func (p *CodeBuilder) nodePosition(expr ast.Node) (ret token.Position) {
	if expr == nil {
		return
	}
	_, ret = p.interp.LoadExpr(expr) // TODO: optimize
	return
}

func (p *CodeBuilder) getCaller(expr ast.Node) string {
	if expr == nil {
		return ""
	}
	return p.interp.Caller(expr)
}

func (p *CodeBuilder) loadExpr(expr ast.Node) (src string, pos token.Position) {
	if expr == nil {
		return
	}
	return p.interp.LoadExpr(expr)
}

func (p *CodeBuilder) newCodeError(pos *token.Position, msg string) *CodeError {
	return &CodeError{Msg: msg, Pos: pos, Scope: p.Scope(), Func: p.Func()}
}

func (p *CodeBuilder) newCodePosError(pos token.Pos, msg string) *CodeError {
	tpos := p.position(pos)
	return &CodeError{Msg: msg, Pos: &tpos, Scope: p.Scope(), Func: p.Func()}
}

func (p *CodeBuilder) newCodePosErrorf(pos token.Pos, format string, args ...interface{}) *CodeError {
	return p.newCodePosError(pos, fmt.Sprintf(format, args...))
}

func (p *CodeBuilder) panicCodeError(pos *token.Position, msg string) {
	panic(p.newCodeError(pos, msg))
}

/*
func (p *CodeBuilder) panicCodePosError(pos token.Pos, msg string) {
	panic(p.newCodePosError(pos, msg))
}
*/

func (p *CodeBuilder) panicCodeErrorf(pos *token.Position, format string, args ...interface{}) {
	panic(p.newCodeError(pos, fmt.Sprintf(format, args...)))
}

func (p *CodeBuilder) panicCodePosErrorf(pos token.Pos, format string, args ...interface{}) {
	panic(p.newCodePosError(pos, fmt.Sprintf(format, args...)))
}

// Scope returns current scope.
func (p *CodeBuilder) Scope() *types.Scope {
	return p.current.scope
}

// Func returns current func (nil means in global scope).
func (p *CodeBuilder) Func() *Func {
	return p.current.fn
}

// Pkg returns the package instance.
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
	p.current.checkLabels(p)
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

func (p *CodeBuilder) popStmt() ast.Stmt {
	stmts := p.current.stmts
	n := len(stmts) - 1
	stmt := stmts[n]
	p.current.stmts = stmts[:n]
	return stmt
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
	if p.comments != nil {
		stmt = &printer.CommentedStmt{Comments: p.comments, Stmt: stmt}
		if p.commentOnce {
			p.comments = nil
		}
	}
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

// Comments returns the comments of next statement.
func (p *CodeBuilder) Comments() *ast.CommentGroup {
	return p.comments
}

// SetComments sets comments to next statement.
func (p *CodeBuilder) SetComments(comments *ast.CommentGroup, once bool) *CodeBuilder {
	if debugComments && comments != nil {
		for i, c := range comments.List {
			log.Println("SetComments", i, c.Text)
		}
	}
	p.comments, p.commentOnce = comments, once
	return p
}

// ReturnErr func
func (p *CodeBuilder) ReturnErr(outer bool) *CodeBuilder {
	if debugInstr {
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
func (p *CodeBuilder) Return(n int, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Return", n)
	}
	fn := p.current.fn
	results := fn.Type().(*types.Signature).Results()
	checkFuncResults(p.pkg, p.stk.GetArgs(n), results, getSrc(src))
	if fn.isInline() {
		for i := n - 1; i >= 0; i-- {
			key := closureParamInst{fn, results.At(i)}
			elem := p.stk.Pop()
			p.doVarRef(p.paramInsts[key], nil, false)
			p.stk.Push(elem)
			p.doAssignWith(1, 1, nil)
		}
		p.Goto(p.getEndingLabel(fn))
	} else {
		p.returnResults(n)
	}
	return p
}

// Call func
func (p *CodeBuilder) Call(n int, ellipsis ...bool) *CodeBuilder {
	return p.CallWith(n, ellipsis != nil && ellipsis[0])
}

// CallWith func
func (p *CodeBuilder) CallWith(n int, ellipsis bool, src ...ast.Node) *CodeBuilder {
	args := p.stk.GetArgs(n)
	n++
	fn := p.stk.Get(-n)
	var flags InstrFlags
	if ellipsis {
		flags = InstrFlagEllipsis
	}
	if debugInstr {
		log.Println("Call", n-1, int(flags))
	}
	ret := toFuncCall(p.pkg, fn, args, flags)
	ret.Src = getSrc(src)
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
		cb.pushVal(cb.paramInsts[key], nil)
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
	if debugInstr {
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
	if err := matchFuncType(pkg, args, ellipsis, sig, "closure argument"); err != nil {
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
	if debugInstr {
		t := sig.Params()
		for i, n := 0, t.Len(); i < n; i++ {
			v := t.At(i)
			if _, ok := v.Type().(*unboundType); ok {
				panic("can't use unbound type in func parameters")
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
	if debugInstr {
		log.Println("NewConstStart", names)
	}
	return p.pkg.newValueDecl(token.NoPos, token.CONST, typ, names...).InitStart(p.pkg)
}

// NewVar func
func (p *CodeBuilder) NewVar(typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVar", names)
	}
	p.pkg.newValueDecl(token.NoPos, token.VAR, typ, names...)
	return p
}

// NewVarStart func
func (p *CodeBuilder) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVarStart", names)
	}
	return p.pkg.newValueDecl(token.NoPos, token.VAR, typ, names...).InitStart(p.pkg)
}

// DefineVarStart func
func (p *CodeBuilder) DefineVarStart(names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("DefineVarStart", names)
	}
	return p.pkg.newValueDecl(token.NoPos, token.DEFINE, nil, names...).InitStart(p.pkg)
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(pos token.Pos, name string, pv **types.Var) *CodeBuilder {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	if debugInstr {
		log.Println("NewAutoVar", name)
	}
	p.emitStmt(stmt)
	typ := &unboundType{ptypes: []*ast.Expr{&spec.Type}}
	*pv = types.NewVar(pos, p.pkg.Types, name, typ)
	if old := p.current.scope.Insert(*pv); old != nil {
		oldPos := p.position(old.Pos())
		p.panicCodePosErrorf(
			pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldPos)
	}
	return p
}

// VarRef func: p.VarRef(nil) means underscore (_)
func (p *CodeBuilder) VarRef(ref interface{}, src ...ast.Node) *CodeBuilder {
	return p.doVarRef(ref, getSrc(src), true)
}

func (p *CodeBuilder) doVarRef(ref interface{}, src ast.Node, allowDebug bool) *CodeBuilder {
	if ref == nil {
		if allowDebug && debugInstr {
			log.Println("VarRef _")
		}
		p.stk.Push(&internal.Elem{
			Val: underscore, // _
		})
	} else {
		switch v := ref.(type) {
		case *types.Var:
			if allowDebug && debugInstr {
				log.Println("VarRef", v.Name())
			}
			fn := p.current.fn
			if fn != nil && fn.isInline() { // is in an inline call
				key := closureParamInst{fn, v}
				if arg, ok := p.paramInsts[key]; ok { // replace param with arg
					v = arg
				}
			}
			p.stk.Push(&internal.Elem{
				Val: toObjectExpr(p.pkg, v), Type: &refType{typ: v.Type()}, Src: src,
			})
		default:
			log.Panicln("TODO: VarRef", reflect.TypeOf(ref))
		}
	}
	return p
}

var (
	elemNone = &internal.Elem{}
)

// None func
func (p *CodeBuilder) None() *CodeBuilder {
	if debugInstr {
		log.Println("None")
	}
	p.stk.Push(elemNone)
	return p
}

// ZeroLit func
func (p *CodeBuilder) ZeroLit(typ types.Type) *CodeBuilder {
	return p.doZeroLit(typ, true)
}

func (p *CodeBuilder) doZeroLit(typ types.Type, allowDebug bool) *CodeBuilder {
	typ0 := typ
	if allowDebug && debugInstr {
		log.Println("ZeroLit")
	}
retry:
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
	case *types.Named:
		typ = t.Underlying()
		goto retry
	}
	ret := &ast.CompositeLit{}
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound == nil {
			t.ptypes = append(t.ptypes, &ret.Type)
		} else {
			typ = t.tBound
			typ0 = typ
			ret.Type = toType(p.pkg, typ)
		}
	default:
		ret.Type = toType(p.pkg, typ)
	}
	p.stk.Push(&internal.Elem{Type: typ0, Val: ret})
	return p
}

// MapLit func
func (p *CodeBuilder) MapLit(typ types.Type, arity int) *CodeBuilder {
	if debugInstr {
		log.Println("MapLit", typ, arity)
	}
	var t *types.Map
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(pkg, tt)
			t = pkg.getUnderlying(tt).(*types.Map)
		case *types.Map:
			typExpr = toMapType(pkg, tt)
			t = tt
		default:
			log.Panicln("MapLit: typ isn't a map type -", reflect.TypeOf(typ))
		}
	}
	if arity == 0 {
		if t == nil {
			t = types.NewMap(types.Typ[types.String], TyEmptyInterface)
			typ = t
			typExpr = toMapType(pkg, t)
		}
		ret := &ast.CompositeLit{Type: typExpr}
		p.stk.Push(&internal.Elem{Type: typ, Val: ret})
		return p
	}
	if (arity & 1) != 0 {
		log.Panicln("MapLit: invalid arity, can't be odd -", arity)
	}
	var key, val types.Type
	var args = p.stk.GetArgs(arity)
	var check = (t != nil)
	if check {
		key, val = t.Key(), t.Elem()
	} else {
		key = boundElementType(pkg, args, 0, arity, 2)
		val = boundElementType(pkg, args, 1, arity, 2)
		t = types.NewMap(Default(pkg, key), Default(pkg, val))
		typ = t
		typExpr = toMapType(pkg, t)
	}
	elts := make([]ast.Expr, arity>>1)
	for i := 0; i < arity; i += 2 {
		elts[i>>1] = &ast.KeyValueExpr{Key: args[i].Val, Value: args[i+1].Val}
		if check {
			if !AssignableTo(pkg, args[i].Type, key) {
				src, pos := p.loadExpr(args[i].Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in map key", src, args[i].Type, key)
			} else if !AssignableTo(pkg, args[i+1].Type, val) {
				src, pos := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in map value", src, args[i+1].Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

func (p *CodeBuilder) toBoundArrayLen(elts []*internal.Elem, arity, limit int) int {
	n := -1
	max := -1
	for i := 0; i < arity; i += 2 {
		if elts[i].Val != nil {
			n = p.toIntVal(elts[i], "index which must be non-negative integer constant")
		} else {
			n++
		}
		if limit >= 0 && n >= limit { // error message
			if elts[i].Src == nil {
				_, pos := p.loadExpr(elts[i+1].Src)
				p.panicCodeErrorf(&pos, "array index %d out of bounds [0:%d]", n, limit)
			}
			src, pos := p.loadExpr(elts[i].Src)
			p.panicCodeErrorf(&pos, "array index %s (value %d) out of bounds [0:%d]", src, n, limit)
		}
		if max < n {
			max = n
		}
	}
	return max + 1
}

func (p *CodeBuilder) toIntVal(v *internal.Elem, msg string) int {
	if cval := v.CVal; cval != nil && cval.Kind() == constant.Int {
		if v, ok := constant.Int64Val(cval); ok {
			return int(v)
		}
	}
	code, pos := p.loadExpr(v.Src)
	p.panicCodeErrorf(&pos, "cannot use %s as %s", code, msg)
	return 0
}

func (p *CodeBuilder) indexElemExpr(args []*internal.Elem, i int) ast.Expr {
	key := args[i].Val
	if key == nil { // none
		return args[i+1].Val
	}
	p.toIntVal(args[i], "index which must be non-negative integer constant")
	return &ast.KeyValueExpr{Key: key, Value: args[i+1].Val}
}

// SliceLit func
func (p *CodeBuilder) SliceLit(typ types.Type, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var keyValMode = (keyVal != nil && keyVal[0])
	if debugInstr {
		log.Println("SliceLit", typ, arity, keyValMode)
	}
	var t *types.Slice
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(pkg, tt)
			t = pkg.getUnderlying(tt).(*types.Slice)
		case *types.Slice:
			typExpr = toSliceType(pkg, tt)
			t = tt
		default:
			log.Panicln("SliceLit: typ isn't a slice type -", reflect.TypeOf(typ))
		}
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			log.Panicln("SliceLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		n := arity >> 1
		elts = make([]ast.Expr, n)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(pkg, args[i+1].Type, val) {
				src, pos := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in slice literal", src, args[i+1].Type, val)
			}
			elts[i>>1] = p.indexElemExpr(args, i)
		}
	} else {
		if arity == 0 {
			if t == nil {
				t = types.NewSlice(TyEmptyInterface)
				typ = t
				typExpr = toSliceType(pkg, t)
			}
			p.stk.Push(&internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr}})
			return p
		}
		var val types.Type
		var args = p.stk.GetArgs(arity)
		var check = (t != nil)
		if check {
			val = t.Elem()
		} else {
			val = boundElementType(pkg, args, 0, arity, 1)
			t = types.NewSlice(Default(pkg, val))
			typ = t
			typExpr = toSliceType(pkg, t)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if check {
				if !AssignableTo(pkg, arg.Type, val) {
					src, pos := p.loadExpr(arg.Src)
					p.panicCodeErrorf(
						&pos, "cannot use %s (type %v) as type %v in slice literal", src, arg.Type, val)
				}
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// ArrayLit func
func (p *CodeBuilder) ArrayLit(typ types.Type, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var keyValMode = (keyVal != nil && keyVal[0])
	if debugInstr {
		log.Println("ArrayLit", typ, arity, keyValMode)
	}
	var t *types.Array
	var typExpr ast.Expr
	var pkg = p.pkg
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(pkg, tt)
		t = pkg.getUnderlying(tt).(*types.Array)
	case *types.Array:
		typExpr = toArrayType(pkg, tt)
		t = tt
	default:
		log.Panicln("ArrayLit: typ isn't a array type -", reflect.TypeOf(typ))
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			log.Panicln("ArrayLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		n := int(t.Len())
		args := p.stk.GetArgs(arity)
		max := p.toBoundArrayLen(args, arity, n)
		val := t.Elem()
		if n < 0 {
			t = types.NewArray(val, int64(max))
			typ = t
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(pkg, args[i+1].Type, val) {
				src, pos := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in array literal", src, args[i+1].Type, val)
			}
			elts[i>>1] = p.indexElemExpr(args, i)
		}
	} else {
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(arity))
			typ = t
		} else if int(n) < arity {
			_, pos := p.loadExpr(args[n].Src)
			p.panicCodeErrorf(&pos, "array index %d out of bounds [0:%d]", n, n)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if !AssignableTo(pkg, arg.Type, val) {
				src, pos := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in array literal", src, arg.Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool) *CodeBuilder {
	if debugInstr {
		log.Println("StructLit", typ, arity, keyVal)
	}
	var t *types.Struct
	var typExpr ast.Expr
	var pkg = p.pkg
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(pkg, tt)
		t = pkg.getUnderlying(tt).(*types.Struct)
	case *types.Struct:
		typExpr = toStructType(pkg, tt)
		t = tt
	default:
		log.Panicln("StructLit: typ isn't a struct type -", reflect.TypeOf(typ))
	}
	var elts []ast.Expr
	var n = t.NumFields()
	var args = p.stk.GetArgs(arity)
	if keyVal {
		if (arity & 1) != 0 {
			log.Panicln("StructLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			idx := p.toIntVal(args[i], "field which must be non-negative integer constant")
			if idx >= n {
				panic("invalid struct field index")
			}
			elt := t.Field(idx)
			eltTy, eltName := elt.Type(), elt.Name()
			if !AssignableTo(pkg, args[i+1].Type, eltTy) {
				src, pos := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in value of field %s",
					src, args[i+1].Type, eltTy, eltName)
			}
			elts[i>>1] = &ast.KeyValueExpr{Key: ident(eltName), Value: args[i+1].Val}
		}
	} else if arity != n {
		if arity != 0 {
			fewOrMany := "few"
			if arity > n {
				fewOrMany = "many"
			}
			_, pos := p.loadExpr(args[arity-1].Src)
			p.panicCodeErrorf(&pos, "too %s values in %v{...}", fewOrMany, typ)
		}
	} else {
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			eltTy := t.Field(i).Type()
			if !AssignableTo(pkg, arg.Type, eltTy) {
				src, pos := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					&pos, "cannot use %s (type %v) as type %v in value of field %s",
					src, arg.Type, eltTy, t.Field(i).Name())
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}})
	return p
}

// Slice func
func (p *CodeBuilder) Slice(slice3 bool, src ...ast.Node) *CodeBuilder { // a[i:j:k]
	if debugInstr {
		log.Println("Slice", slice3)
	}
	n := 3
	if slice3 {
		n++
	}
	srcExpr := getSrc(src)
	args := p.stk.GetArgs(n)
	x := args[0]
	typ := x.Type
	switch t := typ.(type) {
	case *types.Slice:
		// nothing to do
	case *types.Basic:
		if t.Kind() == types.String || t.Kind() == types.UntypedString {
			if slice3 {
				code, pos := p.loadExpr(srcExpr)
				p.panicCodeErrorf(&pos, "invalid operation %s (3-index slice of string)", code)
			}
		} else {
			code, pos := p.loadExpr(x.Src)
			p.panicCodeErrorf(&pos, "cannot slice %s (type %v)", code, typ)
		}
	case *types.Array:
		typ = types.NewSlice(t.Elem())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			typ = types.NewSlice(tt.Elem())
		} else {
			code, pos := p.loadExpr(x.Src)
			p.panicCodeErrorf(&pos, "cannot slice %s (type %v)", code, typ)
		}
	}
	var exprMax ast.Expr
	if slice3 {
		exprMax = args[3].Val
	}
	// TODO: check type
	elem := &internal.Elem{
		Val: &ast.SliceExpr{
			X: x.Val, Low: args[1].Val, High: args[2].Val, Max: exprMax, Slice3: slice3,
		},
		Type: typ, Src: srcExpr,
	}
	p.stk.Ret(n, elem)
	return p
}

// Index func
func (p *CodeBuilder) Index(nidx int, twoValue bool, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Index", nidx, twoValue)
	}
	if nidx != 1 {
		panic("Index doesn't support a[i, j...] yet")
	}
	args := p.stk.GetArgs(2)
	srcExpr := getSrc(src)
	typs, allowTwoValue := p.getIdxValTypes(args[0].Type, false, srcExpr)
	var tyRet types.Type
	if twoValue { // elem, ok = a[key]
		if !allowTwoValue {
			_, pos := p.loadExpr(srcExpr)
			p.panicCodeError(&pos, "assignment mismatch: 2 variables but 1 values")
		}
		pkg := p.pkg
		tyRet = types.NewTuple(
			pkg.NewParam(token.NoPos, "", typs[1]),
			pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
	} else { // elem = a[key]
		tyRet = typs[1]
	}
	elem := &internal.Elem{
		Val: &ast.IndexExpr{X: args[0].Val, Index: args[1].Val}, Type: tyRet, Src: srcExpr,
	}
	// TODO: check index type
	p.stk.Ret(2, elem)
	return p
}

// IndexRef func
func (p *CodeBuilder) IndexRef(nidx int, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("IndexRef", nidx)
	}
	if nidx != 1 {
		panic("IndexRef doesn't support a[i, j...] = val yet")
	}
	args := p.stk.GetArgs(2)
	typ := args[0].Type
	elemRef := &internal.Elem{
		Val: &ast.IndexExpr{X: args[0].Val, Index: args[1].Val},
		Src: getSrc(src),
	}
	if t, ok := typ.(*unboundType); ok {
		tyMapElem := &unboundMapElemType{key: args[1].Type, typ: t}
		elemRef.Type = &refType{typ: tyMapElem}
	} else {
		typs, _ := p.getIdxValTypes(typ, true, elemRef.Src)
		elemRef.Type = &refType{typ: typs[1]}
		// TODO: check index type
	}
	p.stk.Ret(2, elemRef)
	return p
}

func (p *CodeBuilder) getIdxValTypes(typ types.Type, ref bool, idxSrc ast.Node) ([]types.Type, bool) {
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
	case *types.Basic:
		if t.Kind() == types.String {
			if ref {
				src, pos := p.loadExpr(idxSrc)
				p.panicCodeErrorf(&pos, "cannot assign to %s (strings are immutable)", src)
			}
			return []types.Type{tyInt, TyByte}, false
		}
	}
	src, pos := p.loadExpr(idxSrc)
	p.panicCodeErrorf(&pos, "invalid operation: %s (type %v does not support indexing)", src, typ)
	return nil, false
}

var (
	tyInt = types.Typ[types.Int]
)

// Typ func
func (p *CodeBuilder) Typ(typ types.Type) *CodeBuilder {
	if debugInstr {
		log.Println("Typ", typ)
	}
	p.stk.Push(&internal.Elem{
		Val:  toType(p.pkg, typ),
		Type: NewTypeType(typ),
	})
	return p
}

// UntypedBigInt func
func (p *CodeBuilder) UntypedBigInt(v *big.Int, src ...ast.Node) *CodeBuilder {
	pkg := p.pkg
	big := pkg.big()
	if v.IsInt64() {
		val := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(v.Int64(), 10)}
		p.Val(big.Ref("NewInt")).Val(val).Call(1)
	} else {
		/*
			func() *typ {
				v, _ := new(typ).SetString(strVal, 10)
				return v
			}()
		*/
		typ := big.Ref("Int").Type()
		retTyp := types.NewPointer(typ)
		ret := pkg.NewParam(token.NoPos, "", retTyp)
		p.NewClosure(nil, types.NewTuple(ret), false).BodyStart(pkg).
			DefineVarStart("v", "_").
			Val(pkg.builtin.Scope().Lookup("new")).Typ(typ).Call(1).
			MemberVal("SetString").Val(v.String()).Val(10).Call(2).EndInit(1).
			Val(p.Scope().Lookup("v")).Return(1).
			End().Call(0)
	}
	ret := p.stk.Get(-1)
	ret.Type, ret.CVal, ret.Src = pkg.utBigInt, constant.Make(v), getSrc(src)
	return p
}

// UntypedBigRat func
func (p *CodeBuilder) UntypedBigRat(v *big.Rat, src ...ast.Node) *CodeBuilder {
	pkg := p.pkg
	big := pkg.big()
	a, b := v.Num(), v.Denom()
	if a.IsInt64() && b.IsInt64() {
		va := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(a.Int64(), 10)}
		vb := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(b.Int64(), 10)}
		p.Val(big.Ref("NewRat")).Val(va).Val(vb).Call(2)
	} else {
		// new(big.Rat).SetFrac(a, b)
		p.Val(p.pkg.builtin.Scope().Lookup("new")).Typ(big.Ref("Rat").Type()).Call(1).
			MemberVal("SetFrac").UntypedBigInt(a).UntypedBigInt(b).Call(2)
	}
	ret := p.stk.Get(-1)
	ret.Type, ret.CVal, ret.Src = pkg.utBigRat, constant.Make(v), getSrc(src)
	return p
}

// Val func
func (p *CodeBuilder) Val(v interface{}, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		if o, ok := v.(types.Object); ok {
			log.Println("Val", o.Name(), o.Type())
		} else {
			log.Println("Val", v, reflect.TypeOf(v))
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
	return p.pushVal(v, getSrc(src))
}

func (p *CodeBuilder) pushVal(v interface{}, src ast.Node) *CodeBuilder {
	p.stk.Push(toExpr(p.pkg, v, src))
	return p
}

// Star func
func (p *CodeBuilder) Star(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Star")
	}
	arg := p.stk.Get(-1)
	ret := &internal.Elem{Val: &ast.StarExpr{X: arg.Val}, Src: getSrc(src)}
	switch t := arg.Type.(type) {
	case *TypeType:
		t.typ = types.NewPointer(t.typ)
		ret.Type = arg.Type
	case *types.Pointer:
		ret.Type = t.Elem()
	default:
		code, pos := p.loadExpr(arg.Src)
		p.panicCodeErrorf(&pos, "invalid indirect of %s (type %v)", code, t)
	}
	p.stk.Ret(1, ret)
	return p
}

// Elem func
func (p *CodeBuilder) Elem(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Elem")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		code, pos := p.loadExpr(arg.Src)
		p.panicCodeErrorf(&pos, "invalid indirect of %s (type %v)", code, arg.Type)
	}
	p.stk.Ret(1, &internal.Elem{Val: &ast.StarExpr{X: arg.Val}, Type: t.Elem(), Src: getSrc(src)})
	return p
}

// ElemRef func
func (p *CodeBuilder) ElemRef(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("ElemRef")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		code, pos := p.loadExpr(arg.Src)
		p.panicCodeErrorf(&pos, "invalid indirect of %s (type %v)", code, arg.Type)
	}
	p.stk.Ret(1, &internal.Elem{
		Val: &ast.StarExpr{X: arg.Val}, Type: &refType{typ: t.Elem()}, Src: getSrc(src),
	})
	return p
}

// MemberRef func
func (p *CodeBuilder) MemberRef(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("MemberRef", name)
	}
	arg := p.stk.Get(-1)
	switch o := indirect(arg.Type).(type) {
	case *types.Named:
		if struc, ok := p.pkg.getUnderlying(o).(*types.Struct); ok {
			if p.fieldRef(arg.Val, struc, name) {
				return p
			}
		}
	case *types.Struct:
		if p.fieldRef(arg.Val, o, name) {
			return p
		}
	}
	code, pos := p.loadExpr(getSrc(src))
	p.panicCodeErrorf(
		&pos, fmt.Sprintf("%s undefined (type %v has no field or method %s)", code, arg.Type, name))
	return p
}

func (p *CodeBuilder) fieldRef(x ast.Expr, struc *types.Struct, name string) bool {
	if t := structFieldType(struc, name); t != nil {
		p.stk.Ret(1, &internal.Elem{
			Val:  &ast.SelectorExpr{X: x, Sel: ident(name)},
			Type: &refType{typ: t},
		})
		return true
	}
	return false
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

type MemberKind int

const (
	MemberInvalid MemberKind = iota
	MemberMethod
	MemberField
)

// MemberVal func
func (p *CodeBuilder) MemberVal(name string) *CodeBuilder {
	_, err := p.Member(name)
	if err != nil {
		panic(err)
	}
	return p
}

// Member func
func (p *CodeBuilder) Member(name string, src ...ast.Node) (kind MemberKind, err error) {
	if debugInstr {
		log.Println("Member", name)
	}
	srcExpr := getSrc(src)
	arg := p.stk.Get(-1)
	if kind = p.findMember(arg.Type, name, arg.Val, srcExpr); kind != 0 {
		return
	}
	code, pos := p.loadExpr(srcExpr)
	return MemberInvalid, p.newCodeError(
		&pos, fmt.Sprintf("%s undefined (type %v has no field or method %s)", code, arg.Type, name))
}

func (p *CodeBuilder) findMember(typ types.Type, name string, argVal ast.Expr, srcExpr ast.Node) MemberKind {
	switch o := typ.(type) {
	case *types.Pointer:
		switch t := o.Elem().(type) {
		case *types.Named:
			if p.method(t, name, argVal, srcExpr) {
				return MemberMethod
			}
			if struc, ok := p.pkg.getUnderlying(t).(*types.Struct); ok {
				if kind := p.field(struc, name, argVal, srcExpr); kind != 0 {
					return kind
				}
			}
		case *types.Struct:
			if kind := p.field(t, name, argVal, srcExpr); kind != 0 {
				return kind
			}
		}
	case *types.Named:
		if p.method(o, name, argVal, srcExpr) {
			return MemberMethod
		}
		switch t := p.pkg.getUnderlying(o).(type) {
		case *types.Struct:
			if kind := p.field(t, name, argVal, srcExpr); kind != 0 {
				return kind
			}
		case *types.Interface:
			t.Complete()
			if p.method(t, name, argVal, srcExpr) {
				return MemberMethod
			}
		}
	case *types.Struct:
		if kind := p.field(o, name, argVal, srcExpr); kind != 0 {
			return kind
		}
	case *types.Interface:
		o.Complete()
		if p.method(o, name, argVal, srcExpr) {
			return MemberMethod
		}
	}
	return 0
}

type methodList interface {
	NumMethods() int
	Method(i int) *types.Func
}

func (p *CodeBuilder) method(o methodList, name string, argVal ast.Expr, src ast.Node) bool {
	for i, n := 0, o.NumMethods(); i < n; i++ {
		method := o.Method(i)
		if method.Name() == name {
			p.stk.Ret(1, &internal.Elem{
				Val:  &ast.SelectorExpr{X: argVal, Sel: ident(name)},
				Type: methodTypeOf(method.Type()),
				Src:  src,
			})
			return true
		}
	}
	return false
}

func (p *CodeBuilder) field(o *types.Struct, name string, argVal ast.Expr, src ast.Node) MemberKind {
	for i, n := 0, o.NumFields(); i < n; i++ {
		fld := o.Field(i)
		if fld.Name() == name {
			p.stk.Ret(1, &internal.Elem{
				Val:  &ast.SelectorExpr{X: argVal, Sel: ident(name)},
				Type: fld.Type(),
				Src:  src,
			})
			return MemberField
		} else if fld.Embedded() {
			if kind := p.findMember(fld.Type(), name, argVal, src); kind != 0 {
				return kind
			}
		}
	}
	return 0
}

func methodTypeOf(typ types.Type) types.Type {
	sig := typ.(*types.Signature)
	return types.NewSignature(nil, sig.Params(), sig.Results(), sig.Variadic())
}

func indirect(typ types.Type) types.Type {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	return typ
}

// AssignOp func
func (p *CodeBuilder) AssignOp(tok token.Token) *CodeBuilder {
	if debugInstr {
		log.Println("AssignOp", tok)
	}
	args := p.stk.GetArgs(2)
	stmt := &ast.AssignStmt{
		Tok: tok,
		Lhs: []ast.Expr{args[0].Val},
		Rhs: []ast.Expr{args[1].Val},
	}
	// TODO: type check
	p.emitStmt(stmt)
	p.stk.PopN(2)
	return p
}

// Assign func
func (p *CodeBuilder) Assign(lhs int, rhs ...int) *CodeBuilder {
	var v int
	if rhs != nil {
		v = rhs[0]
	} else {
		v = lhs
	}
	if debugInstr {
		log.Println("Assign", lhs, v)
	}
	return p.doAssignWith(lhs, v, nil)
}

// AssignWith func
func (p *CodeBuilder) AssignWith(lhs, rhs int, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Assign", lhs, rhs)
	}
	return p.doAssignWith(lhs, rhs, getSrc(src))
}

func (p *CodeBuilder) doAssignWith(lhs, rhs int, src ast.Node) *CodeBuilder {
	args := p.stk.GetArgs(lhs + rhs)
	stmt := &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: make([]ast.Expr, lhs),
		Rhs: make([]ast.Expr, rhs),
	}
	if rhs == 1 {
		if rhsVals, ok := args[lhs].Type.(*types.Tuple); ok {
			if lhs != rhsVals.Len() {
				pos := p.nodePosition(src)
				caller := p.getCaller(args[lhs].Src)
				p.panicCodeErrorf(
					&pos, "assignment mismatch: %d variables but %v returns %d values",
					lhs, caller, rhsVals.Len())
			}
			for i := 0; i < lhs; i++ {
				val := &internal.Elem{Type: rhsVals.At(i).Type()}
				checkAssignType(p.pkg, args[i].Type, val)
				stmt.Lhs[i] = args[i].Val
			}
			stmt.Rhs[0] = args[lhs].Val
			goto done
		}
	}
	if lhs == rhs {
		for i := 0; i < lhs; i++ {
			checkAssignType(p.pkg, args[i].Type, args[lhs+i])
			stmt.Lhs[i] = args[i].Val
			stmt.Rhs[i] = args[lhs+i].Val
		}
	} else {
		pos := p.nodePosition(src)
		p.panicCodeErrorf(
			&pos, "assignment mismatch: %d variables but %d values", lhs, rhs)
	}
done:
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

func callOpFunc(pkg *Package, name string, args []*internal.Elem, flags InstrFlags) (ret *internal.Elem) {
	if t, ok := args[0].Type.(*types.Named); ok {
		op := lookupMethod(t, name)
		if op != nil {
			fn := &internal.Elem{
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
	return toFuncCall(pkg, toObject(pkg, op, nil), args, flags)
}

// BinaryOp func
func (p *CodeBuilder) BinaryOp(op token.Token, src ...ast.Node) *CodeBuilder {
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
	if debugInstr {
		log.Println("BinaryOp", op, name)
	}
	ret := callOpFunc(p.pkg, name, args, 0)
	ret.Src = getSrc(src)
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

		token.LAND: "LAnd", // &&
		token.LOR:  "LOr",  // ||

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
	if debugInstr {
		log.Println("CompareNil", op)
	}
	arg := p.stk.Get(-1)
	// TODO: type check
	ret := &internal.Elem{
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
	if debugInstr {
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
		token.NOT:   "LNot",
		token.ARROW: "Recv",
		token.AND:   "Addr",
	}
)

// IncDec func
func (p *CodeBuilder) IncDec(op token.Token) *CodeBuilder {
	if debugInstr {
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
	if debugInstr {
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
	if debugInstr {
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
	if debugInstr {
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
	if debugInstr {
		log.Println("If")
	}
	stmt := &ifStmt{}
	p.startBlockStmt(stmt, "if statement", &stmt.old)
	return p
}

// Then func
func (p *CodeBuilder) Then() *CodeBuilder {
	if debugInstr {
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
	if debugInstr {
		log.Println("Else")
	}
	if flow, ok := p.current.codeBlock.(*ifStmt); ok {
		flow.Else(p)
		return p
	}
	panic("use if..else please")
}

// TypeSwitch func
func (p *CodeBuilder) TypeSwitch(name string) *CodeBuilder {
	if debugInstr {
		log.Println("TypeSwitch")
	}
	stmt := &typeSwitchStmt{name: name}
	p.startBlockStmt(stmt, "type switch statement", &stmt.old)
	return p
}

// TypeAssert func
func (p *CodeBuilder) TypeAssert(typ types.Type, twoValue bool) *CodeBuilder {
	arg := p.stk.Get(-1)
	xType, ok := arg.Type.(*types.Interface)
	if !ok {
		panic("TODO: can't type assert on non interface expr")
	}
	if !types.AssertableTo(xType, typ) {
		log.Panicf("TODO: can't assert type %v to %v\n", xType, typ)
	}
	pkg := p.pkg
	ret := &ast.TypeAssertExpr{X: arg.Val, Type: toType(pkg, typ)}
	if twoValue {
		tyRet := types.NewTuple(
			pkg.NewParam(token.NoPos, "", typ),
			pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
		p.stk.Ret(1, &internal.Elem{Type: tyRet, Val: ret})
	} else {
		p.stk.Ret(1, &internal.Elem{Type: typ, Val: ret})
	}
	return p
}

// TypeAssertThen func
func (p *CodeBuilder) TypeAssertThen() *CodeBuilder {
	if debugInstr {
		log.Println("TypeAssertThen")
	}
	if flow, ok := p.current.codeBlock.(*typeSwitchStmt); ok {
		flow.TypeAssertThen(p)
		return p
	}
	panic("use typeSwitch..typeAssertThen please")
}

// TypeCase func
func (p *CodeBuilder) TypeCase(n int) *CodeBuilder { // n=0 means default case
	if debugInstr {
		log.Println("TypeCase", n)
	}
	if flow, ok := p.current.codeBlock.(*typeSwitchStmt); ok {
		flow.TypeCase(p, n)
		return p
	}
	panic("use switch x.(type) .. case please")
}

// Select
func (p *CodeBuilder) Select() *CodeBuilder {
	if debugInstr {
		log.Println("Select")
	}
	stmt := &selectStmt{}
	p.startBlockStmt(stmt, "select statement", &stmt.old)
	return p
}

// CommCase
func (p *CodeBuilder) CommCase(n int) *CodeBuilder {
	if debugInstr {
		log.Println("CommCase", n)
	}
	if n > 1 {
		panic("TODO: multi commStmt in select..case?")
	}
	if flow, ok := p.current.codeBlock.(*selectStmt); ok {
		flow.CommCase(p, n)
		return p
	}
	panic("use select..case please")
}

// Switch func
func (p *CodeBuilder) Switch() *CodeBuilder {
	if debugInstr {
		log.Println("Switch")
	}
	stmt := &switchStmt{}
	p.startBlockStmt(stmt, "switch statement", &stmt.old)
	return p
}

// Case func
func (p *CodeBuilder) Case(n int) *CodeBuilder { // n=0 means default case
	if debugInstr {
		log.Println("Case", n)
	}
	if flow, ok := p.current.codeBlock.(*switchStmt); ok {
		flow.Case(p, n)
		return p
	}
	panic("use switch..case please")
}

// Label func
func (p *CodeBuilder) Label(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Label", name)
	}
	p.current.defineLabel(p, name, p.nodePosition(getSrc(src)))
	p.current.label = &ast.LabeledStmt{Label: ident(name)}
	return p
}

// Goto func
func (p *CodeBuilder) Goto(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Goto", name)
	}
	p.current.useLabel(p, name, p.nodePosition(getSrc(src)))
	p.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: ident(name)})
	return p
}

// Break func
func (p *CodeBuilder) Break(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Break", name)
	}
	if name != "" {
		p.current.useLabel(p, name, p.nodePosition(getSrc(src)))
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.BREAK, Label: ident(name)})
	return p
}

// Continue func
func (p *CodeBuilder) Continue(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Continue", name)
	}
	if name != "" {
		p.current.useLabel(p, name, p.nodePosition(getSrc(src)))
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.CONTINUE, Label: ident(name)})
	return p
}

// Fallthrough func
func (p *CodeBuilder) Fallthrough() *CodeBuilder {
	if debugInstr {
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
	if debugInstr {
		log.Println("For")
	}
	stmt := &forStmt{}
	p.startBlockStmt(stmt, "for statement", &stmt.old)
	return p
}

// Post func
func (p *CodeBuilder) Post() *CodeBuilder {
	if debugInstr {
		log.Println("Post")
	}
	if flow, ok := p.current.codeBlock.(*forStmt); ok {
		flow.Post(p)
		return p
	}
	panic("please use Post() in for statement")
}

// ForRange func
func (p *CodeBuilder) ForRange(names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("ForRange", names)
	}
	stmt := &forRangeStmt{names: names}
	p.startBlockStmt(stmt, "for range statement", &stmt.old)
	return p
}

// RangeAssignThen func
func (p *CodeBuilder) RangeAssignThen() *CodeBuilder {
	if debugInstr {
		log.Println("RangeAssignThen")
	}
	if flow, ok := p.current.codeBlock.(*forRangeStmt); ok {
		flow.RangeAssignThen(p)
		return p
	}
	panic("please use RangeAssignThen() in for range statement")
}

// ResetStmt resets the statement state of CodeBuilder.
func (p *CodeBuilder) ResetStmt() {
	if debugInstr {
		log.Println("ResetStmt")
	}
	p.stk.SetLen(p.current.base)
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
	if debugInstr {
		log.Println("End")
		if p.stk.Len() > p.current.base {
			panic("forget to call EndStmt()?")
		}
	}
	p.current.End(p)
	return p
}

// ResetInit resets the variable init state of CodeBuilder.
func (p *CodeBuilder) ResetInit() {
	if debugInstr {
		log.Println("ResetInit")
	}
	p.varDecl = p.varDecl.resetInit(p)
}

// EndInit func
func (p *CodeBuilder) EndInit(n int) *CodeBuilder {
	if debugInstr {
		log.Println("EndInit", n)
	}
	p.varDecl = p.varDecl.endInit(p, n)
	return p
}

// Debug func
func (p *CodeBuilder) Debug(dbg func(cb *CodeBuilder)) *CodeBuilder {
	dbg(p)
	return p
}

// ----------------------------------------------------------------------------
