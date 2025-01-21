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
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/goplus/gogen/internal"
	xtoken "github.com/goplus/gogen/token"
	"golang.org/x/tools/go/types/typeutil"
)

func getSrc(node []ast.Node) ast.Node {
	if node != nil {
		return node[0]
	}
	return nil
}

func getSrcPos(node ast.Node) token.Pos {
	if node != nil {
		return node.Pos()
	}
	return token.NoPos
}

func getPos(src []ast.Node) token.Pos {
	if src == nil {
		return token.NoPos
	}
	return getSrcPos(src[0])
}

// ----------------------------------------------------------------------------

type posNode struct {
	pos, end token.Pos
}

func NewPosNode(pos token.Pos, end ...token.Pos) ast.Node {
	ret := &posNode{pos: pos, end: pos}
	if end != nil {
		ret.end = end[0]
	}
	return ret
}

func (p *posNode) Pos() token.Pos { return p.pos }
func (p *posNode) End() token.Pos { return p.end }

// ----------------------------------------------------------------------------

type codeBlock interface {
	End(cb *CodeBuilder, src ast.Node)
}

type vblockCtx struct {
	codeBlock
	scope *types.Scope
}

type codeBlockCtx struct {
	codeBlock
	scope *types.Scope
	base  int
	stmts []ast.Stmt
	label *ast.LabeledStmt
	flows int // flow flags
}

const (
	flowFlagBreak = 1 << iota
	flowFlagContinue
	flowFlagReturn
	flowFlagGoto
	flowFlagWithLabel
)

type Label struct {
	types.Label
	used bool
}

type funcBodyCtx struct {
	codeBlockCtx
	fn     *Func
	labels map[string]*Label
}

func (p *funcBodyCtx) checkLabels(cb *CodeBuilder) {
	for name, l := range p.labels {
		if !l.used {
			cb.handleCodeErrorf(l.Pos(), "label %s defined and not used", name)
		}
	}
}

type CodeError struct {
	Fset dbgPositioner
	Pos  token.Pos
	Msg  string
}

func (p *CodeError) Error() string {
	pos := p.Fset.Position(p.Pos)
	return fmt.Sprintf("%v: %s", pos, p.Msg)
}

// CodeBuilder type
type CodeBuilder struct {
	stk       internal.Stack
	current   funcBodyCtx
	fset      dbgPositioner
	comments  *ast.CommentGroup
	pkg       *Package
	btiMap    *typeutil.Map
	valDecl   *ValueDecl
	ctxt      *typesContext
	interp    NodeInterpreter
	rec       Recorder
	loadNamed LoadNamedFunc
	handleErr func(err error)
	closureParamInsts
	iotav       int
	commentOnce bool
	noSkipConst bool
}

func (p *CodeBuilder) init(pkg *Package) {
	conf := pkg.conf
	p.pkg = pkg
	p.fset = conf.DbgPositioner
	if p.fset == nil {
		p.fset = conf.Fset
	}
	p.noSkipConst = conf.NoSkipConstant
	p.handleErr = conf.HandleErr
	if p.handleErr == nil {
		p.handleErr = defaultHandleErr
	}
	p.rec = conf.Recorder
	p.interp = conf.NodeInterpreter
	if p.interp == nil {
		p.interp = nodeInterp{}
	}
	p.ctxt = newTypesContext()
	p.loadNamed = conf.LoadNamed
	if p.loadNamed == nil {
		p.loadNamed = defaultLoadNamed
	}
	p.current.scope = pkg.Types.Scope()
	p.stk.Init()
	p.closureParamInsts.init()
}

func defaultLoadNamed(at *Package, t *types.Named) {
	// no delay-loaded named types
}

func defaultHandleErr(err error) {
	panic(err)
}

type nodeInterp struct{}

func (p nodeInterp) LoadExpr(expr ast.Node) string {
	return ""
}

func getFunExpr(fn *internal.Elem) (caller string, pos token.Pos) {
	if fn == nil {
		return "the closure call", token.NoPos
	}
	caller = types.ExprString(fn.Val)
	pos = getSrcPos(fn.Src)
	return
}

func getCaller(expr *internal.Elem) string {
	if ce, ok := expr.Val.(*ast.CallExpr); ok {
		return types.ExprString(ce.Fun)
	}
	return "the function call"
}

func (p *CodeBuilder) loadExpr(expr ast.Node) (string, token.Pos) {
	if expr == nil {
		return "", token.NoPos
	}
	return p.interp.LoadExpr(expr), expr.Pos()
}

func (p *CodeBuilder) newCodeError(pos token.Pos, msg string) *CodeError {
	return &CodeError{Msg: msg, Pos: pos, Fset: p.fset}
}

func (p *CodeBuilder) newCodeErrorf(pos token.Pos, format string, args ...interface{}) *CodeError {
	return p.newCodeError(pos, fmt.Sprintf(format, args...))
}

func (p *CodeBuilder) handleCodeError(pos token.Pos, msg string) {
	p.handleErr(p.newCodeError(pos, msg))
}

func (p *CodeBuilder) handleCodeErrorf(pos token.Pos, format string, args ...interface{}) {
	p.handleErr(p.newCodeError(pos, fmt.Sprintf(format, args...)))
}

func (p *CodeBuilder) panicCodeError(pos token.Pos, msg string) {
	panic(p.newCodeError(pos, msg))
}

func (p *CodeBuilder) panicCodeErrorf(pos token.Pos, format string, args ...interface{}) {
	panic(p.newCodeError(pos, fmt.Sprintf(format, args...)))
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

func (p *CodeBuilder) startFuncBody(fn *Func, src []ast.Node, old *funcBodyCtx) *CodeBuilder {
	p.current.fn, old.fn = fn, p.current.fn
	p.current.labels, old.labels = nil, p.current.labels
	p.startBlockStmt(fn, src, "func "+fn.Name(), &old.codeBlockCtx)
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
		if name := v.Name(); name != "" && name != "_" {
			scope.Insert(v)
		}
	}
}

func (p *CodeBuilder) endFuncBody(old funcBodyCtx) []ast.Stmt {
	p.current.checkLabels(p)
	p.current.fn = old.fn
	p.current.labels = old.labels
	stmts, _ := p.endBlockStmt(&old.codeBlockCtx)
	return stmts
}

func (p *CodeBuilder) startBlockStmt(current codeBlock, src []ast.Node, comment string, old *codeBlockCtx) *CodeBuilder {
	var start, end token.Pos
	if src != nil {
		start, end = src[0].Pos(), src[0].End()
	}
	scope := types.NewScope(p.current.scope, start, end, comment)
	p.current.codeBlockCtx, *old = codeBlockCtx{current, scope, p.stk.Len(), nil, nil, 0}, p.current.codeBlockCtx
	return p
}

func (p *CodeBuilder) endBlockStmt(old *codeBlockCtx) ([]ast.Stmt, int) {
	flows := p.current.flows
	if p.current.label != nil {
		p.emitStmt(&ast.EmptyStmt{})
	}
	stmts := p.current.stmts
	p.stk.SetLen(p.current.base)
	p.current.codeBlockCtx = *old
	return stmts, flows
}

func (p *CodeBuilder) clearBlockStmt() []ast.Stmt {
	stmts := p.current.stmts
	p.current.stmts = nil
	return stmts
}

func (p *CodeBuilder) startVBlockStmt(current codeBlock, comment string, old *vblockCtx) *CodeBuilder {
	*old = vblockCtx{codeBlock: p.current.codeBlock, scope: p.current.scope}
	scope := types.NewScope(p.current.scope, token.NoPos, token.NoPos, comment)
	p.current.codeBlock, p.current.scope = current, scope
	return p
}

func (p *CodeBuilder) endVBlockStmt(old *vblockCtx) {
	p.current.codeBlock, p.current.scope = old.codeBlock, old.scope
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
//
//	idx := cb.startStmtAt(stmt)
//	...
//	cb.commitStmt(idx)
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
		p.pkg.setStmtComments(stmt, p.comments)
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

func (p *CodeBuilder) BackupComments() (*ast.CommentGroup, bool) {
	return p.comments, p.commentOnce
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
			p.current.flows |= flowFlagReturn
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
		p.current.flows |= flowFlagReturn
		p.returnResults(n)
	}
	return p
}

// Call func
func (p *CodeBuilder) Call(n int, ellipsis ...bool) *CodeBuilder {
	var flags InstrFlags
	if ellipsis != nil && ellipsis[0] {
		flags = InstrFlagEllipsis
	}
	return p.CallWith(n, flags)
}

// CallWith always panics on error, while CallWithEx returns err if match function call failed.
func (p *CodeBuilder) CallWith(n int, flags InstrFlags, src ...ast.Node) *CodeBuilder {
	if err := p.CallWithEx(n, flags, src...); err != nil {
		panic(err)
	}
	return p
}

// CallWith always panics on error, while CallWithEx returns err if match function call failed.
// If an error ocurs, CallWithEx pops all function arguments from the CodeBuilder stack.
// In most case, you should call CallWith instead of CallWithEx.
func (p *CodeBuilder) CallWithEx(n int, flags InstrFlags, src ...ast.Node) error {
	fn := p.stk.Get(-(n + 1))
	if t, ok := fn.Type.(*btiMethodType); ok {
		n++
		fn.Type = t.Type
		fn = p.stk.Get(-(n + 1))
		if t.eargs != nil {
			for _, arg := range t.eargs {
				p.Val(arg)
			}
			n += len(t.eargs)
		}
	}
	args := p.stk.GetArgs(n)
	if debugInstr {
		log.Println("Call", n, int(flags), "//", fn.Type)
	}
	s := getSrc(src)
	fn.Src = s
	ret, err := matchFuncCall(p.pkg, fn, args, flags)
	if err != nil {
		p.stk.PopN(n)
		return err
	}
	ret.Src = s
	p.stk.Ret(n+1, ret)
	return nil
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

func (p *CodeBuilder) getEndingLabel(fn *Func) *Label {
	key := closureParamInst{fn, nil}
	if v, ok := p.paramInsts[key]; ok {
		return p.current.labels[v.Name()]
	}
	ending := p.pkg.autoName()
	p.paramInsts[key] = types.NewParam(token.NoPos, nil, ending, nil)
	return p.NewLabel(token.NoPos, ending)
}

func (p *CodeBuilder) needEndingLabel(fn *Func) (*Label, bool) {
	key := closureParamInst{fn, nil}
	if v, ok := p.paramInsts[key]; ok {
		return p.current.labels[v.Name()], true
	}
	return nil, false
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

// CallInlineClosureStart func
func (p *CodeBuilder) CallInlineClosureStart(sig *types.Signature, arity int, ellipsis bool) *CodeBuilder {
	if debugInstr {
		log.Println("CallInlineClosureStart", arity, ellipsis)
	}
	pkg := p.pkg
	closure := pkg.newInlineClosure(sig, arity)
	results := sig.Results()
	for i, n := 0, results.Len(); i < n; i++ {
		p.emitVar(pkg, closure, results.At(i), false)
	}
	p.startFuncBody(closure, nil, &closure.old)
	args := p.stk.GetArgs(arity)
	var flags InstrFlags
	if ellipsis {
		flags = InstrFlagEllipsis
	}
	if err := matchFuncType(pkg, args, flags, sig, nil); err != nil {
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
	sig := types.NewSignatureType(nil, nil, nil, params, results, variadic)
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
	return p.pkg.newClosure(sig)
}

// NewType func
func (p *CodeBuilder) NewType(name string, src ...ast.Node) *TypeDecl {
	return p.NewTypeDefs().NewType(name, src...)
}

// AliasType func
func (p *CodeBuilder) AliasType(name string, typ types.Type, src ...ast.Node) *types.Named {
	decl := p.NewTypeDefs().AliasType(name, typ, src...)
	return decl.typ
}

// NewConstStart func
func (p *CodeBuilder) NewConstStart(typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewConstStart", names)
	}
	defs := p.valueDefs(token.CONST)
	return p.pkg.newValueDecl(defs.NewPos(), defs.scope, token.NoPos, token.CONST, typ, names...).InitStart(p.pkg)
}

// NewVar func
func (p *CodeBuilder) NewVar(typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVar", names)
	}
	defs := p.valueDefs(token.VAR)
	p.pkg.newValueDecl(defs.NewPos(), defs.scope, token.NoPos, token.VAR, typ, names...)
	return p
}

// NewVarStart func
func (p *CodeBuilder) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVarStart", names)
	}
	defs := p.valueDefs(token.VAR)
	return p.pkg.newValueDecl(defs.NewPos(), defs.scope, token.NoPos, token.VAR, typ, names...).InitStart(p.pkg)
}

// DefineVarStart func
func (p *CodeBuilder) DefineVarStart(pos token.Pos, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("DefineVarStart", names)
	}
	return p.pkg.newValueDecl(
		ValueAt{}, p.current.scope, pos, token.DEFINE, nil, names...).InitStart(p.pkg)
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
		oldPos := p.fset.Position(old.Pos())
		p.panicCodeErrorf(
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
		if v, ok := ref.(string); ok {
			_, ref = p.Scope().LookupParent(v, token.NoPos)
		}
		if v, ok := ref.(*types.Var); ok {
			if allowDebug && debugInstr {
				log.Println("VarRef", v.Name(), v.Type())
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
		} else {
			code, pos := p.loadExpr(src)
			p.panicCodeErrorf(pos, "%s is not a variable", code)
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
		log.Println("ZeroLit //", typ)
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
	case *types.Interface, *types.Map, *types.Slice, *types.Pointer, *types.Signature, *types.Chan:
		return p.Val(nil)
	case *types.Named:
		typ = p.getUnderlying(t)
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
func (p *CodeBuilder) MapLit(typ types.Type, arity int, src ...ast.Node) *CodeBuilder {
	if err := p.MapLitEx(typ, arity, src...); err != nil {
		panic(err)
	}
	return p
}

// MapLit func
func (p *CodeBuilder) MapLitEx(typ types.Type, arity int, src ...ast.Node) error {
	if debugInstr {
		log.Println("MapLit", typ, arity)
	}
	var t *types.Map
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		var ok bool
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(pkg, tt)
			t, ok = p.getUnderlying(tt).(*types.Map)
		case *types.Map:
			typExpr = toMapType(pkg, tt)
			t, ok = tt, true
		}
		if !ok {
			return p.newCodeErrorf(getPos(src), "type %v isn't a map", typ)
		}
	}
	if arity == 0 {
		if t == nil {
			t = types.NewMap(types.Typ[types.String], TyEmptyInterface)
			typ = t
			typExpr = toMapType(pkg, t)
		}
		ret := &ast.CompositeLit{Type: typExpr}
		p.stk.Push(&internal.Elem{Type: typ, Val: ret, Src: getSrc(src)})
		return nil
	}
	if (arity & 1) != 0 {
		return p.newCodeErrorf(getPos(src), "MapLit: invalid arity, can't be odd - %d", arity)
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
				return p.newCodeErrorf(
					pos, "cannot use %s (type %v) as type %v in map key", src, args[i].Type, key)
			} else if !AssignableTo(pkg, args[i+1].Type, val) {
				src, pos := p.loadExpr(args[i+1].Src)
				return p.newCodeErrorf(
					pos, "cannot use %s (type %v) as type %v in map value", src, args[i+1].Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return nil
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
				pos := getSrcPos(elts[i+1].Src)
				p.panicCodeErrorf(pos, "array index %d out of bounds [0:%d]", n, limit)
			}
			src, pos := p.loadExpr(elts[i].Src)
			p.panicCodeErrorf(pos, "array index %s (value %d) out of bounds [0:%d]", src, n, limit)
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
	p.panicCodeErrorf(pos, "cannot use %s as %s", code, msg)
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
	var keyValMode = (keyVal != nil && keyVal[0])
	return p.SliceLitEx(typ, arity, keyValMode)
}

// SliceLitEx func
func (p *CodeBuilder) SliceLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	var elts []ast.Expr
	if debugInstr {
		log.Println("SliceLit", typ, arity, keyVal)
	}
	var t *types.Slice
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		switch tt := typ.(type) {
		case *types.Named:
			typExpr = toNamedType(pkg, tt)
			t = p.getUnderlying(tt).(*types.Slice)
		case *types.Slice:
			typExpr = toSliceType(pkg, tt)
			t = tt
		default:
			log.Panicln("SliceLit: typ isn't a slice type -", reflect.TypeOf(typ))
		}
	}
	if keyVal { // in keyVal mode
		if (arity & 1) != 0 {
			log.Panicln("SliceLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		n := arity >> 1
		elts = make([]ast.Expr, n)
		for i := 0; i < arity; i += 2 {
			arg := args[i+1]
			if !AssignableConv(pkg, arg.Type, val, arg) {
				src, pos := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					pos, "cannot use %s (type %v) as type %v in slice literal", src, args[i+1].Type, val)
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
			p.stk.Push(&internal.Elem{
				Type: typ, Val: &ast.CompositeLit{Type: typExpr}, Src: getSrc(src),
			})
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
				if !AssignableConv(pkg, arg.Type, val, arg) {
					src, pos := p.loadExpr(arg.Src)
					p.panicCodeErrorf(
						pos, "cannot use %s (type %v) as type %v in slice literal", src, arg.Type, val)
				}
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return p
}

// ArrayLit func
func (p *CodeBuilder) ArrayLit(typ types.Type, arity int, keyVal ...bool) *CodeBuilder {
	var keyValMode = (keyVal != nil && keyVal[0])
	return p.ArrayLitEx(typ, arity, keyValMode)
}

// ArrayLitEx func
func (p *CodeBuilder) ArrayLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	var elts []ast.Expr
	if debugInstr {
		log.Println("ArrayLit", typ, arity, keyVal)
	}
	var t *types.Array
	var typExpr ast.Expr
	var pkg = p.pkg
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(pkg, tt)
		t = p.getUnderlying(tt).(*types.Array)
	case *types.Array:
		typExpr = toArrayType(pkg, tt)
		t = tt
	default:
		log.Panicln("ArrayLit: typ isn't a array type -", reflect.TypeOf(typ))
	}
	if keyVal { // in keyVal mode
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
					pos, "cannot use %s (type %v) as type %v in array literal", src, args[i+1].Type, val)
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
			pos := getSrcPos(args[n].Src)
			p.panicCodeErrorf(pos, "array index %d out of bounds [0:%d]", n, n)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if !AssignableConv(pkg, arg.Type, val, arg) {
				src, pos := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					pos, "cannot use %s (type %v) as type %v in array literal", src, arg.Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return p
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("StructLit", typ, arity, keyVal)
	}
	var t *types.Struct
	var typExpr ast.Expr
	var pkg = p.pkg
	switch tt := typ.(type) {
	case *types.Named:
		typExpr = toNamedType(pkg, tt)
		t = p.getUnderlying(tt).(*types.Struct)
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
					pos, "cannot use %s (type %v) as type %v in value of field %s",
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
			pos := getSrcPos(args[arity-1].Src)
			p.panicCodeErrorf(pos, "too %s values in %v{...}", fewOrMany, typ)
		}
	} else {
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			eltTy := t.Field(i).Type()
			if !AssignableTo(pkg, arg.Type, eltTy) {
				src, pos := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					pos, "cannot use %s (type %v) as type %v in value of field %s",
					src, arg.Type, eltTy, t.Field(i).Name())
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
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
				p.panicCodeErrorf(pos, "invalid operation %s (3-index slice of string)", code)
			}
		} else {
			code, pos := p.loadExpr(x.Src)
			p.panicCodeErrorf(pos, "cannot slice %s (type %v)", code, typ)
		}
	case *types.Array:
		typ = types.NewSlice(t.Elem())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			typ = types.NewSlice(tt.Elem())
		} else {
			code, pos := p.loadExpr(x.Src)
			p.panicCodeErrorf(pos, "cannot slice %s (type %v)", code, typ)
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

// Index func:
//   - a[i]
//   - fn[T1, T2, ..., Tn]
//   - G[T1, T2, ..., Tn]
func (p *CodeBuilder) Index(nidx int, twoValue bool, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Index", nidx, twoValue)
	}
	args := p.stk.GetArgs(nidx + 1)
	if nidx > 0 {
		if _, ok := args[1].Type.(*TypeType); ok {
			return p.instantiate(nidx, args, src...)
		}
	}
	if nidx != 1 {
		panic("Index doesn't support a[i, j...] yet")
	}
	srcExpr := getSrc(src)
	typs, allowTwoValue := p.getIdxValTypes(args[0].Type, false, srcExpr)
	var tyRet types.Type
	if twoValue { // elem, ok = a[key]
		if !allowTwoValue {
			pos := getSrcPos(srcExpr)
			p.panicCodeError(pos, "assignment mismatch: 2 variables but 1 values")
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
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}, true
	case *types.Array:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Pointer:
		elem := t.Elem()
		if named, ok := elem.(*types.Named); ok {
			elem = p.getUnderlying(named)
		}
		if e, ok := elem.(*types.Array); ok {
			return []types.Type{tyInt, e.Elem()}, false
		}
	case *types.Basic:
		if (t.Info() & types.IsString) != 0 {
			if ref {
				src, pos := p.loadExpr(idxSrc)
				p.panicCodeErrorf(pos, "cannot assign to %s (strings are immutable)", src)
			}
			return []types.Type{tyInt, TyByte}, false
		}
	case *types.Named:
		typ = p.getUnderlying(t)
		goto retry
	}
	src, pos := p.loadExpr(idxSrc)
	p.panicCodeErrorf(pos, "invalid operation: %s (type %v does not support indexing)", src, typ)
	return nil, false
}

var (
	tyInt = types.Typ[types.Int]
)

// Typ func
func (p *CodeBuilder) Typ(typ types.Type, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Typ", typ)
	}
	p.stk.Push(&internal.Elem{
		Val:  toType(p.pkg, typ),
		Type: NewTypeType(typ),
		Src:  getSrc(src),
	})
	return p
}

// UntypedBigInt func
func (p *CodeBuilder) UntypedBigInt(v *big.Int, src ...ast.Node) *CodeBuilder {
	pkg := p.pkg
	bigPkg := pkg.big()
	if v.IsInt64() {
		val := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(v.Int64(), 10)}
		p.Val(bigPkg.Ref("NewInt")).Val(val).Call(1)
	} else {
		/*
			func() *typ {
				v, _ := new(typ).SetString(strVal, 10)
				return v
			}()
		*/
		typ := bigPkg.Ref("Int").Type()
		retTyp := types.NewPointer(typ)
		ret := pkg.NewParam(token.NoPos, "", retTyp)
		p.NewClosure(nil, types.NewTuple(ret), false).BodyStart(pkg).
			DefineVarStart(token.NoPos, "v", "_").
			Val(pkg.builtin.Ref("new")).Typ(typ).Call(1).
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
	bigPkg := pkg.big()
	a, b := v.Num(), v.Denom()
	if a.IsInt64() && b.IsInt64() {
		va := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(a.Int64(), 10)}
		vb := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(b.Int64(), 10)}
		p.Val(bigPkg.Ref("NewRat")).Val(va).Val(vb).Call(2)
	} else {
		// new(big.Rat).SetFrac(a, b)
		p.Val(p.pkg.builtin.Ref("new")).Typ(bigPkg.Ref("Rat").Type()).Call(1).
			MemberVal("SetFrac").UntypedBigInt(a).UntypedBigInt(b).Call(2)
	}
	ret := p.stk.Get(-1)
	ret.Type, ret.CVal, ret.Src = pkg.utBigRat, constant.Make(v), getSrc(src)
	return p
}

func (p *CodeBuilder) VarVal(name string, src ...ast.Node) *CodeBuilder {
	_, o := p.Scope().LookupParent(name, token.NoPos)
	if o == nil {
		log.Panicf("VarVal: variable `%v` not found\n", name)
	}
	return p.Val(o, src...)
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
	argType := arg.Type
retry:
	switch t := argType.(type) {
	case *TypeType:
		ret.Type = t.Pointer()
	case *types.Pointer:
		ret.Type = t.Elem()
	case *types.Named:
		argType = p.getUnderlying(t)
		goto retry
	default:
		code, pos := p.loadExpr(arg.Src)
		p.panicCodeErrorf(pos, "invalid indirect of %s (type %v)", code, t)
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
		p.panicCodeErrorf(pos, "invalid indirect of %s (type %v)", code, arg.Type)
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
		p.panicCodeErrorf(pos, "invalid indirect of %s (type %v)", code, arg.Type)
	}
	p.stk.Ret(1, &internal.Elem{
		Val: &ast.StarExpr{X: arg.Val}, Type: &refType{typ: t.Elem()}, Src: getSrc(src),
	})
	return p
}

// MemberVal func
func (p *CodeBuilder) MemberVal(name string, src ...ast.Node) *CodeBuilder {
	_, err := p.Member(name, MemberFlagVal, src...)
	if err != nil {
		panic(err)
	}
	return p
}

// MemberRef func
func (p *CodeBuilder) MemberRef(name string, src ...ast.Node) *CodeBuilder {
	_, err := p.Member(name, MemberFlagRef, src...)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *CodeBuilder) refMember(typ types.Type, name string, argVal ast.Expr, src ast.Node) MemberKind {
	switch o := indirect(typ).(type) {
	case *types.Named:
		if struc, ok := p.getUnderlying(o).(*types.Struct); ok {
			if p.fieldRef(argVal, struc, name, src) {
				return MemberField
			}
		}
	case *types.Struct:
		if p.fieldRef(argVal, o, name, src) {
			return MemberField
		}
	}
	return MemberInvalid
}

func (p *CodeBuilder) fieldRef(x ast.Expr, o *types.Struct, name string, src ast.Node) bool {
	var embed []*types.Var
	for i, n := 0, o.NumFields(); i < n; i++ {
		fld := o.Field(i)
		if fld.Name() == name {
			if p.rec != nil {
				p.rec.Member(src, fld)
			}
			p.stk.Ret(1, &internal.Elem{
				Val:  &ast.SelectorExpr{X: x, Sel: ident(name)},
				Type: &refType{typ: fld.Type()},
			})
			return true
		} else if fld.Embedded() {
			embed = append(embed, fld)
		}
	}
	for _, fld := range embed {
		fldt := fld.Type()
		if o, ok := fldt.(*types.Pointer); ok {
			fldt = o.Elem()
		}
		if t, ok := fldt.(*types.Named); ok {
			u := p.getUnderlying(t)
			if struc, ok := u.(*types.Struct); ok {
				if p.fieldRef(x, struc, name, src) {
					return true
				}
			}
		}
	}
	return false
}

type (
	MemberKind int
	MemberFlag int
)

const (
	MemberInvalid MemberKind = iota
	MemberMethod
	MemberAutoProperty
	MemberField
	memberBad MemberKind = -1
)

const (
	MemberFlagVal MemberFlag = iota
	MemberFlagMethodAlias
	MemberFlagAutoProperty
	MemberFlagRef MemberFlag = -1

	// private state
	memberFlagMethodToFunc MemberFlag = 0x80
)

func aliasNameOf(name string, flag MemberFlag) (string, MemberFlag) {
	// flag > 0 means:
	//   flag == MemberFlagMethodAlias ||
	//   flag == MemberFlagAutoProperty ||
	//   flag == memberFlagMethodToFunc
	if flag > 0 && name != "" {
		if c := name[0]; c >= 'a' && c <= 'z' {
			return string(rune(c)+('A'-'a')) + name[1:], flag
		}
		if flag != memberFlagMethodToFunc {
			flag = MemberFlagVal
		}
	}
	return "", flag
}

// Member access member by its name.
// src should point to the full source node `x.sel`
func (p *CodeBuilder) Member(name string, flag MemberFlag, src ...ast.Node) (kind MemberKind, err error) {
	srcExpr := getSrc(src)
	arg := p.stk.Get(-1)
	if debugInstr {
		log.Println("Member", name, flag, "//", arg.Type)
	}
	switch arg.Type {
	case p.pkg.utBigInt, p.pkg.utBigRat, p.pkg.utBigFlt:
		arg.Type = DefaultConv(p.pkg, arg.Type, arg)
	}
	at := arg.Type
	if flag == MemberFlagRef {
		kind = p.refMember(at, name, arg.Val, srcExpr)
	} else {
		var aliasName string
		var t, isType = at.(*TypeType)
		if isType { // (T).method or (*T).method
			at = t.Type()
			flag = memberFlagMethodToFunc
		}
		aliasName, flag = aliasNameOf(name, flag)
		kind = p.findMember(at, name, aliasName, flag, arg, srcExpr, nil)
		if isType && kind != MemberMethod {
			code, pos := p.loadExpr(srcExpr)
			return MemberInvalid, p.newCodeError(
				pos, fmt.Sprintf("%s undefined (type %v has no method %s)", code, at, name))
		}
	}
	if kind > 0 {
		return
	}
	code, pos := p.loadExpr(srcExpr)
	return MemberInvalid, p.newCodeError(
		pos, fmt.Sprintf("%s undefined (type %v has no field or method %s)", code, arg.Type, name))
}

func (p *CodeBuilder) getUnderlying(t *types.Named) types.Type {
	u := t.Underlying()
	if u == nil {
		p.loadNamed(p.pkg, t)
		u = t.Underlying()
	}
	return u
}

func (p *CodeBuilder) ensureLoaded(typ types.Type) {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	if t, ok := typ.(*types.Named); ok && (t.NumMethods() == 0 || t.Underlying() == nil) {
		if debugMatch {
			log.Println("==> EnsureLoaded", typ)
		}
		p.loadNamed(p.pkg, t)
	}
}

func getUnderlying(pkg *Package, typ types.Type) types.Type {
	u := typ.Underlying()
	if u == nil {
		if t, ok := typ.(*types.Named); ok {
			pkg.cb.loadNamed(pkg, t)
			u = t.Underlying()
		}
	}
	return u
}

func (p *CodeBuilder) findMember(
	typ types.Type, name, aliasName string, flag MemberFlag, arg *Element, srcExpr ast.Node, visited map[*types.Struct]none) MemberKind {
	var named *types.Named
retry:
	switch o := typ.(type) {
	case *types.Pointer:
		switch t := o.Elem().(type) {
		case *types.Named:
			u := p.getUnderlying(t) // may cause to loadNamed (delay-loaded)
			struc, fstruc := u.(*types.Struct)
			if fstruc {
				if kind := p.normalField(struc, name, arg, srcExpr); kind != MemberInvalid {
					return kind
				}
			}
			if kind := p.method(t, name, aliasName, flag, arg, srcExpr); kind != MemberInvalid {
				return kind
			}
			if fstruc {
				return p.embeddedField(struc, name, aliasName, flag, arg, srcExpr, visited)
			}
		case *types.Struct:
			if kind := p.field(t, name, aliasName, flag, arg, srcExpr, visited); kind != MemberInvalid {
				return kind
			}
		}
	case *types.Named:
		named, typ = o, p.getUnderlying(o) // may cause to loadNamed (delay-loaded)
		if kind := p.method(o, name, aliasName, flag, arg, srcExpr); kind != MemberInvalid {
			return kind
		}
		goto retry
	case *types.Struct:
		if kind := p.field(o, name, aliasName, flag, arg, srcExpr, visited); kind != MemberInvalid {
			return kind
		}
		if named != nil {
			return MemberInvalid
		}
	case *types.Interface:
		o.Complete()
		if kind := p.method(o, name, aliasName, flag, arg, srcExpr); kind != MemberInvalid {
			return kind
		}
	case *types.Basic, *types.Slice, *types.Map, *types.Chan:
		return p.btiMethod(p.getBuiltinTI(o), name, aliasName, flag, srcExpr)
	}
	return MemberInvalid
}

type methodList interface {
	NumMethods() int
	Method(i int) *types.Func
}

func selector(arg *Element, name string) *ast.SelectorExpr {
	denoted := &ast.Object{Data: arg}
	return &ast.SelectorExpr{X: arg.Val, Sel: &ast.Ident{Name: name, Obj: denoted}}
}

func denoteRecv(v *ast.SelectorExpr) *Element {
	if o := v.Sel.Obj; o != nil {
		if e, ok := o.Data.(*Element); ok {
			return e
		}
	}
	return nil
}

func getDenoted(expr ast.Expr) *ast.Object {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return v.Sel.Obj
	case *ast.Ident:
		return v.Obj
	default:
		return nil
	}
}

func setDenoted(expr ast.Expr, denoted *ast.Object) {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		v.Sel.Obj = denoted
	case *ast.Ident:
		v.Obj = denoted
	}
}

func (p *CodeBuilder) allowAccess(pkg *types.Package, name string) bool {
	if !ast.IsExported(name) && pkg != nil && pkg.Path() != p.pkg.Path() {
		return false
	}
	return true
}

func (p *CodeBuilder) method(
	o methodList, name, aliasName string, flag MemberFlag, arg *Element, src ast.Node) (kind MemberKind) {
	var found *types.Func
	var exact bool
	for i, n := 0, o.NumMethods(); i < n; i++ {
		method := o.Method(i)
		v := method.Name()
		if !p.allowAccess(method.Pkg(), v) {
			continue
		}
		if v == name {
			found, exact = method, true
			break
		} else if flag > 0 && v == aliasName {
			found = method
			if method.Pkg() != p.pkg.Types {
				break
			}
		}
	}
	if found != nil {
		autoprop := !exact && flag == MemberFlagAutoProperty
		typ := found.Type()
		if autoprop && !methodHasAutoProperty(typ, 0) {
			return memberBad
		}

		sel := selector(arg, found.Name())
		ret := &internal.Elem{Val: sel, Src: src}
		if t, set := p.methodSigOf(typ, flag, arg, ret); set {
			ret.Type = t
		}
		p.stk.Ret(1, ret)

		if p.rec != nil {
			p.rec.Member(src, found)
		}
		if autoprop {
			p.CallWith(0, 0, src)
			return MemberAutoProperty
		}
		return MemberMethod
	}
	if t, ok := o.(*types.Named); ok {
		kind = p.btiMethod(p.getBuiltinTI(t), name, aliasName, flag, src)
	}
	return
}

func isTypeConvert(otyp, typ types.Type) (string, bool) {
	if otyp != typ {
		if t, ok := typ.Underlying().(*types.Basic); ok &&
			(t.Info()&types.IsUntyped) == 0 {
			return t.Name(), true
		}
	}
	return "", false
}

func (p *CodeBuilder) btiMethod(
	o *BuiltinTI, name, aliasName string, flag MemberFlag, src ast.Node) MemberKind {
	if o != nil {
		for i, n := 0, o.numMethods(); i < n; i++ {
			method := o.method(i)
			v := method.Name
			if v == name || (flag > 0 && v == aliasName) {
				autoprop := flag == MemberFlagAutoProperty && v == aliasName
				this := p.stk.Pop()
				if fn, ok := isTypeConvert(o.typ, this.Type); ok {
					this.Val = &ast.CallExpr{
						Fun:  ast.NewIdent(fn),
						Args: []ast.Expr{this.Val},
					}
					this.Type = &btiMethodType{Type: o.typ, eargs: method.Exargs}
				} else {
					this.Type = &btiMethodType{Type: this.Type, eargs: method.Exargs}
				}
				p.Val(method.Fn, src)
				p.stk.Push(this)
				if p.rec != nil {
					p.rec.Member(src, method.Fn)
				}
				if autoprop {
					p.CallWith(0, 0, src)
					return MemberAutoProperty
				}
				return MemberMethod
			}
		}
	}
	return MemberInvalid
}

func (p *CodeBuilder) normalField(
	o *types.Struct, name string, arg *Element, src ast.Node) MemberKind {
	for i, n := 0, o.NumFields(); i < n; i++ {
		fld := o.Field(i)
		v := fld.Name()
		if !p.allowAccess(fld.Pkg(), v) {
			continue
		}
		if v == name {
			p.stk.Ret(1, &internal.Elem{
				Val:  selector(arg, name),
				Type: fld.Type(),
				Src:  src,
			})
			if p.rec != nil {
				p.rec.Member(src, fld)
			}
			return MemberField
		}
	}
	return MemberInvalid
}

func (p *CodeBuilder) embeddedField(
	o *types.Struct, name, aliasName string, flag MemberFlag, arg *Element, src ast.Node, visited map[*types.Struct]none) MemberKind {
	if visited == nil {
		visited = make(map[*types.Struct]none)
	} else if _, ok := visited[o]; ok {
		return MemberInvalid
	}
	visited[o] = none{}

	for i, n := 0, o.NumFields(); i < n; i++ {
		if fld := o.Field(i); fld.Embedded() {
			if kind := p.findMember(fld.Type(), name, aliasName, flag, arg, src, visited); kind != MemberInvalid {
				return kind
			}
		}
	}
	return MemberInvalid
}

func (p *CodeBuilder) field(
	o *types.Struct, name, aliasName string, flag MemberFlag, arg *Element, src ast.Node, visited map[*types.Struct]none) MemberKind {
	if kind := p.normalField(o, name, arg, src); kind != MemberInvalid {
		return kind
	}
	return p.embeddedField(o, name, aliasName, flag, arg, src, visited)
}

func toFuncSig(sig *types.Signature, recv *types.Var) *types.Signature {
	sp := sig.Params()
	spLen := sp.Len()
	vars := make([]*types.Var, spLen+1)
	vars[0] = recv
	for i := 0; i < spLen; i++ {
		vars[i+1] = sp.At(i)
	}
	return types.NewSignatureType(nil, nil, nil, types.NewTuple(vars...), sig.Results(), sig.Variadic())
}

func methodToFuncSig(pkg *Package, o types.Object, fn *Element) *types.Signature {
	sig := o.Type().(*types.Signature)
	recv := sig.Recv()
	if recv == nil { // special signature
		fn.Val = toObjectExpr(pkg, o)
		return sig
	}

	sel := fn.Val.(*ast.SelectorExpr)
	sel.Sel = ident(o.Name())
	sel.X = &ast.ParenExpr{X: sel.X}
	return toFuncSig(sig, recv)
}

func (p *CodeBuilder) methodSigOf(typ types.Type, flag MemberFlag, arg, ret *Element) (types.Type, bool) {
	if flag != memberFlagMethodToFunc {
		return methodCallSig(typ), true
	}

	sig := typ.(*types.Signature)
	if t, ok := CheckFuncEx(sig); ok {
		switch ext := t.(type) {
		case *TyStaticMethod:
			return p.funcExSigOf(ext.Func, ret)
		case *TyTemplateRecvMethod:
			return p.funcExSigOf(ext.Func, ret)
		}
		// TODO: We should take `methodSigOf` more seriously
		return typ, true
	}

	sel := ret.Val.(*ast.SelectorExpr)
	at := arg.Type.(*TypeType).typ
	recv := sig.Recv().Type()
	_, isPtr := recv.(*types.Pointer) // recv is a pointer
	if t, ok := at.(*types.Pointer); ok {
		if !isPtr {
			if _, ok := recv.Underlying().(*types.Interface); !ok { // and recv isn't a interface
				log.Panicf("recv of method %v.%s isn't a pointer\n", t.Elem(), sel.Sel.Name)
			}
		}
	} else if isPtr { // use *T
		at = types.NewPointer(at)
		sel.X = &ast.StarExpr{X: sel.X}
	}
	sel.X = &ast.ParenExpr{X: sel.X}

	return toFuncSig(sig, types.NewVar(token.NoPos, nil, "", at)), true
}

func (p *CodeBuilder) funcExSigOf(o types.Object, ret *Element) (types.Type, bool) {
	ret.Val = toObjectExpr(p.pkg, o)
	ret.Type = o.Type()
	return nil, false
}

func methodCallSig(typ types.Type) types.Type {
	sig := typ.(*types.Signature)
	if _, ok := CheckFuncEx(sig); ok {
		// TODO: We should take `methodSigOf` more seriously
		return typ
	}
	return types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic())
}

func indirect(typ types.Type) types.Type {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	return typ
}

// IncDec func
func (p *CodeBuilder) IncDec(op token.Token, src ...ast.Node) *CodeBuilder {
	name := goxPrefix + incdecOps[op]
	if debugInstr {
		log.Println("IncDec", op)
	}
	pkg := p.pkg
	arg := p.stk.Pop()
	if t, ok := arg.Type.(*refType).typ.(*types.Named); ok {
		op := lookupMethod(t, name)
		if op != nil {
			fn := &internal.Elem{
				Val:  &ast.SelectorExpr{X: arg.Val, Sel: ident(name)},
				Type: realType(op.Type()),
			}
			ret := toFuncCall(pkg, fn, []*Element{arg}, 0)
			if ret.Type != nil {
				p.shouldNoResults(name, src)
			}
			p.emitStmt(&ast.ExprStmt{X: ret.Val})
			return p
		}
	}
	fn := pkg.builtin.Ref(name)
	t := fn.Type().(*TyInstruction)
	if _, err := t.instr.Call(pkg, []*Element{arg}, 0, nil); err != nil {
		panic(err)
	}
	return p
}

var (
	incdecOps = [...]string{
		token.INC: "Inc",
		token.DEC: "Dec",
	}
)

// AssignOp func
func (p *CodeBuilder) AssignOp(op token.Token, src ...ast.Node) *CodeBuilder {
	args := p.stk.GetArgs(2)
	stmt := callAssignOp(p.pkg, op, args, src)
	p.emitStmt(stmt)
	p.stk.PopN(2)
	return p
}

func checkDivisionByZero(cb *CodeBuilder, a, b *internal.Elem) {
	if a.CVal == nil {
		if isNormalInt(cb, a) {
			if c := b.CVal; c != nil {
				switch c.Kind() {
				case constant.Int, constant.Float, constant.Complex:
					if constant.Sign(c) == 0 {
						pos := getSrcPos(b.Src)
						cb.panicCodeError(pos, "invalid operation: division by zero")
					}
				}
			}
		}
	} else if c := b.CVal; c != nil {
		switch c.Kind() {
		case constant.Int, constant.Float, constant.Complex:
			if constant.Sign(c) == 0 {
				pos := getSrcPos(b.Src)
				cb.panicCodeError(pos, "invalid operation: division by zero")
			}
		}
	}
}

func callAssignOp(pkg *Package, tok token.Token, args []*internal.Elem, src []ast.Node) ast.Stmt {
	name := goxPrefix + assignOps[tok]
	if debugInstr {
		log.Println("AssignOp", tok, name)
	}
	if t, ok := args[0].Type.(*refType).typ.(*types.Named); ok {
		op := lookupMethod(t, name)
		if op != nil {
			fn := &internal.Elem{
				Val:  &ast.SelectorExpr{X: args[0].Val, Sel: ident(name)},
				Type: realType(op.Type()),
			}
			ret := toFuncCall(pkg, fn, args, 0)
			if ret.Type != nil {
				pkg.cb.shouldNoResults(name, src)
			}
			return &ast.ExprStmt{X: ret.Val}
		}
	}
	op := pkg.builtin.Ref(name)
	if tok == token.QUO_ASSIGN {
		checkDivisionByZero(&pkg.cb, &internal.Elem{Val: args[0].Val, Type: args[0].Type.(*refType).typ}, args[1])
	}
	fn := &internal.Elem{
		Val: ident(op.Name()), Type: op.Type(),
	}
	toFuncCall(pkg, fn, args, 0)
	return &ast.AssignStmt{
		Tok: tok,
		Lhs: []ast.Expr{args[0].Val},
		Rhs: []ast.Expr{args[1].Val},
	}
}

func (p *CodeBuilder) shouldNoResults(name string, src []ast.Node) {
	pos := token.NoPos
	if src != nil {
		pos = src[0].Pos()
	}
	p.panicCodeErrorf(pos, "operator %s should return no results\n", name)
}

var (
	assignOps = [...]string{
		token.ADD_ASSIGN: "AddAssign", // +=
		token.SUB_ASSIGN: "SubAssign", // -=
		token.MUL_ASSIGN: "MulAssign", // *=
		token.QUO_ASSIGN: "QuoAssign", // /=
		token.REM_ASSIGN: "RemAssign", // %=

		token.AND_ASSIGN:     "AndAssign",    // &=
		token.OR_ASSIGN:      "OrAssign",     // |=
		token.XOR_ASSIGN:     "XorAssign",    // ^=
		token.AND_NOT_ASSIGN: "AndNotAssign", // &^=
		token.SHL_ASSIGN:     "LshAssign",    // <<=
		token.SHR_ASSIGN:     "RshAssign",    // >>=
	}
)

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
	mkBlockStmt := false
	args := p.stk.GetArgs(lhs + rhs)
	stmt := &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: make([]ast.Expr, lhs),
		Rhs: make([]ast.Expr, rhs),
	}
	if rhs == 1 {
		if rhsVals, ok := args[lhs].Type.(*types.Tuple); ok {
			if lhs != rhsVals.Len() {
				pos := getSrcPos(src)
				caller := getCaller(args[lhs])
				p.panicCodeErrorf(
					pos, "assignment mismatch: %d variables but %v returns %d values",
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
			lhsType := args[i].Type
			checkAssignType(p.pkg, lhsType, args[lhs+i])
			stmt.Lhs[i] = args[i].Val
			stmt.Rhs[i] = args[lhs+i].Val
		}
	} else {
		pos := getSrcPos(src)
		p.panicCodeErrorf(
			pos, "assignment mismatch: %d variables but %d values", lhs, rhs)
	}
done:
	p.emitStmt(stmt)
	if mkBlockStmt { // }
		p.End()
	} else {
		p.stk.PopN(lhs + rhs)
	}
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

func doUnaryOp(cb *CodeBuilder, op token.Token, args []*internal.Elem, flags InstrFlags) (ret *internal.Elem, err error) {
	name := goxPrefix + unaryOps[op]
	pkg := cb.pkg
	typ := args[0].Type
retry:
	switch t := typ.(type) {
	case *types.Named:
		lm := lookupMethod(t, name)
		if lm != nil {
			fn := &internal.Elem{
				Val:  &ast.SelectorExpr{X: args[0].Val, Sel: ident(name)},
				Type: realType(lm.Type()),
			}
			return matchFuncCall(pkg, fn, args, flags|instrFlagOpFunc)
		}
	case *types.Pointer:
		typ = t.Elem()
		goto retry
	}
	lm := pkg.builtin.Ref(name)
	return matchFuncCall(pkg, toObject(pkg, lm, nil), args, flags)
}

// UnaryOp:
//   - cb.UnaryOp(op token.Token)
//   - cb.UnaryOp(op token.Token, twoValue bool)
//   - cb.UnaryOp(op token.Token, twoValue bool, src ast.Node)
func (p *CodeBuilder) UnaryOp(op token.Token, params ...interface{}) *CodeBuilder {
	var src ast.Node
	var flags InstrFlags
	switch len(params) {
	case 2:
		src, _ = params[1].(ast.Node)
		fallthrough
	case 1:
		if params[0].(bool) {
			flags = InstrFlagTwoValue
		}
	}
	if debugInstr {
		log.Println("UnaryOp", op, "flags:", flags)
	}
	ret, err := doUnaryOp(p, op, p.stk.GetArgs(1), flags)
	if err != nil {
		panic(err)
	}
	ret.Src = src
	p.stk.Ret(1, ret)
	return p
}

// BinaryOp func
func (p *CodeBuilder) BinaryOp(op token.Token, src ...ast.Node) *CodeBuilder {
	const (
		errNotFound = syscall.ENOENT
	)
	if debugInstr {
		log.Println("BinaryOp", xtoken.String(op))
	}
	pkg := p.pkg
	name := goxPrefix + binaryOps[op]
	args := p.stk.GetArgs(2)

	var ret *internal.Elem
	var err error = errNotFound
	isUserDef := false
	arg0 := args[0].Type
	named0, ok0 := checkNamed(arg0)
	if ok0 {
		if fn, e := pkg.MethodToFunc(arg0, name, src...); e == nil {
			ret, err = matchFuncCall(pkg, fn, args, instrFlagBinaryOp)
			isUserDef = true
		}
	}
	if err != nil {
		arg1 := args[1].Type
		if named1, ok1 := checkNamed(arg1); ok1 && named0 != named1 {
			if fn, e := pkg.MethodToFunc(arg1, name, src...); e == nil {
				ret, err = matchFuncCall(pkg, fn, args, instrFlagBinaryOp)
				isUserDef = true
			}
		}
	}
	if err != nil && !isUserDef {
		if op == token.QUO {
			checkDivisionByZero(p, args[0], args[1])
		}
		if op == token.EQL || op == token.NEQ {
			if !ComparableTo(pkg, args[0], args[1]) {
				err = errors.New("mismatched types")
			} else {
				ret, err = &internal.Elem{
					Val: &ast.BinaryExpr{
						X: checkParenExpr(args[0].Val), Op: op,
						Y: checkParenExpr(args[1].Val),
					},
					Type: types.Typ[types.UntypedBool],
					CVal: binaryOp(p, op, args),
				}, nil
			}
		} else if lm := pkg.builtin.TryRef(name); lm != nil {
			ret, err = matchFuncCall(pkg, toObject(pkg, lm, nil), args, 0)
		} else {
			err = errNotFound
		}
	}

	expr := getSrc(src)
	if err != nil {
		opstr := xtoken.String(op)
		src, pos := p.loadExpr(expr)
		if src == "" {
			src = opstr
		}
		if err != errNotFound {
			p.panicCodeErrorf(
				pos, "invalid operation: %s (mismatched types %v and %v)", src, arg0, args[1].Type)
		} else {
			arg0Src, _ := p.loadExpr(args[0].Src)
			p.panicCodeErrorf(
				pos, "invalid operation: operator %s not defined on %s (%v)", opstr, arg0Src, arg0)
		}
	}
	ret.Src = expr
	p.stk.Ret(2, ret)
	return p
}

func checkNamed(typ types.Type) (ret *types.Named, ok bool) {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	ret, ok = typ.(*types.Named)
	return
}

var (
	unaryOps = [...]string{
		token.SUB:   "Neg",
		token.ADD:   "Dup",
		token.XOR:   "Not",
		token.NOT:   "LNot",
		token.ARROW: "Recv",
		token.AND:   "Addr",
	}
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

		xtoken.SRARROW:   "PointTo", // ->
		xtoken.BIDIARROW: "PointBi", // <>
	}
)

// CompareNil func
func (p *CodeBuilder) CompareNil(op token.Token, src ...ast.Node) *CodeBuilder {
	return p.Val(nil).BinaryOp(op)
}

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

// Block starts a block statement.
func (p *CodeBuilder) Block(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Block")
	}
	stmt := &blockStmt{}
	p.startBlockStmt(stmt, src, "block statement", &stmt.old)
	return p
}

// VBlock starts a vblock statement.
func (p *CodeBuilder) VBlock() *CodeBuilder {
	if debugInstr {
		log.Println("VBlock")
	}
	stmt := &vblockStmt{}
	p.startVBlockStmt(stmt, "vblock statement", &stmt.old)
	return p
}

// InVBlock checks if current statement is in vblock or not.
func (p *CodeBuilder) InVBlock() bool {
	_, ok := p.current.codeBlock.(*vblockStmt)
	return ok
}

// Block starts a if statement.
func (p *CodeBuilder) If(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("If")
	}
	stmt := &ifStmt{}
	p.startBlockStmt(stmt, src, "if statement", &stmt.old)
	return p
}

// Then starts body of a if/switch/for/case statement.
func (p *CodeBuilder) Then(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Then")
	}
	if flow, ok := p.current.codeBlock.(controlFlow); ok {
		flow.Then(p, src...)
		return p
	}
	panic("use if..then or switch..then or for..then please")
}

// Else starts else body of a if..else statement.
func (p *CodeBuilder) Else(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Else")
	}
	if flow, ok := p.current.codeBlock.(*ifStmt); ok {
		flow.Else(p, src...)
		return p
	}
	panic("use if..else please")
}

// TypeSwitch starts a type switch statement.
//
// <pre>
// typeSwitch(name) init; expr typeAssertThen
// type1, type2, ... typeN typeCase(N)
//
//	...
//	end
//
// type1, type2, ... typeM typeCase(M)
//
//	...
//	end
//
// end
// </pre>
func (p *CodeBuilder) TypeSwitch(name string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("TypeSwitch")
	}
	stmt := &typeSwitchStmt{name: name}
	p.startBlockStmt(stmt, src, "type switch statement", &stmt.old)
	return p
}

// TypeAssert func
func (p *CodeBuilder) TypeAssert(typ types.Type, twoValue bool, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("TypeAssert", typ, twoValue)
	}
	arg := p.stk.Get(-1)
	xType, ok := p.checkInterface(arg.Type)
	if !ok {
		text, pos := p.loadExpr(getSrc(src))
		p.panicCodeErrorf(
			pos, "invalid type assertion: %s (non-interface type %v on left)", text, arg.Type)
	}
	if missing := p.missingMethod(typ, xType); missing != "" {
		pos := getSrcPos(getSrc(src))
		p.panicCodeErrorf(
			pos, "impossible type assertion:\n\t%v does not implement %v (missing %s method)",
			typ, arg.Type, missing)
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

func (p *CodeBuilder) missingMethod(T types.Type, V *types.Interface) (missing string) {
	p.ensureLoaded(T)
	if m, _ := types.MissingMethod(T, V, false); m != nil {
		missing = m.Name()
	}
	return
}

func (p *CodeBuilder) checkInterface(typ types.Type) (*types.Interface, bool) {
retry:
	switch t := typ.(type) {
	case *types.Interface:
		return t, true
	case *types.Named:
		typ = p.getUnderlying(t)
		goto retry
	}
	return nil, false
}

// TypeAssertThen starts body of a type switch statement.
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

// TypeCase starts case body of a type switch statement.
func (p *CodeBuilder) TypeCase(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("TypeCase")
	}
	if flow, ok := p.current.codeBlock.(*typeSwitchStmt); ok {
		flow.TypeCase(p, src...)
		return p
	}
	panic("use switch x.(type) .. case please")
}

// TypeDefaultThen starts default clause of a type switch statement.
func (p *CodeBuilder) TypeDefaultThen(src ...ast.Node) *CodeBuilder {
	return p.TypeCase(src...).Then(src...)
}

// Select starts a select statement.
func (p *CodeBuilder) Select(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Select")
	}
	stmt := &selectStmt{}
	p.startBlockStmt(stmt, src, "select statement", &stmt.old)
	return p
}

// CommCase starts case clause of a select statement.
func (p *CodeBuilder) CommCase(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("CommCase")
	}
	if flow, ok := p.current.codeBlock.(*selectStmt); ok {
		flow.CommCase(p, src...)
		return p
	}
	panic("use select..case please")
}

// CommDefaultThen starts default clause of a select statement.
func (p *CodeBuilder) CommDefaultThen(src ...ast.Node) *CodeBuilder {
	return p.CommCase(src...).Then(src...)
}

// Switch starts a switch statement.
func (p *CodeBuilder) Switch(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Switch")
	}
	stmt := &switchStmt{}
	p.startBlockStmt(stmt, src, "switch statement", &stmt.old)
	return p
}

// Case starts case clause of a switch statement.
func (p *CodeBuilder) Case(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Case")
	}
	if flow, ok := p.current.codeBlock.(*switchStmt); ok {
		flow.Case(p, src...)
		return p
	}
	panic("use switch..case please")
}

// DefaultThen starts default clause of a switch statement.
func (p *CodeBuilder) DefaultThen(src ...ast.Node) *CodeBuilder {
	return p.Case(src...).Then(src...)
}

func (p *CodeBuilder) NewLabel(pos token.Pos, name string) *Label {
	if p.current.fn == nil {
		panic(p.newCodeError(pos, "syntax error: non-declaration statement outside function body"))
	}
	if old, ok := p.current.labels[name]; ok {
		oldPos := p.fset.Position(old.Pos())
		p.handleCodeErrorf(pos, "label %s already defined at %v", name, oldPos)
		return nil
	}
	if p.current.labels == nil {
		p.current.labels = make(map[string]*Label)
	}
	l := &Label{Label: *types.NewLabel(pos, p.pkg.Types, name)}
	p.current.labels[name] = l
	return l
}

// LookupLabel func
func (p *CodeBuilder) LookupLabel(name string) (l *Label, ok bool) {
	l, ok = p.current.labels[name]
	return
}

// Label func
func (p *CodeBuilder) Label(l *Label) *CodeBuilder {
	name := l.Name()
	if debugInstr {
		log.Println("Label", name)
	}
	if p.current.label != nil {
		p.current.label.Stmt = &ast.EmptyStmt{}
		p.current.stmts = append(p.current.stmts, p.current.label)
	}
	p.current.label = &ast.LabeledStmt{Label: ident(name)}
	return p
}

// Goto func
func (p *CodeBuilder) Goto(l *Label) *CodeBuilder {
	name := l.Name()
	if debugInstr {
		log.Println("Goto", name)
	}
	l.used = true
	p.current.flows |= flowFlagGoto
	p.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: ident(name)})
	return p
}

func (p *CodeBuilder) labelFlow(flow int, l *Label) (string, *ast.Ident) {
	if l != nil {
		l.used = true
		p.current.flows |= (flow | flowFlagWithLabel)
		return l.Name(), ident(l.Name())
	}
	p.current.flows |= flow
	return "", nil
}

// Break func
func (p *CodeBuilder) Break(l *Label) *CodeBuilder {
	name, label := p.labelFlow(flowFlagBreak, l)
	if debugInstr {
		log.Println("Break", name)
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.BREAK, Label: label})
	return p
}

// Continue func
func (p *CodeBuilder) Continue(l *Label) *CodeBuilder {
	name, label := p.labelFlow(flowFlagContinue, l)
	if debugInstr {
		log.Println("Continue", name)
	}
	p.emitStmt(&ast.BranchStmt{Tok: token.CONTINUE, Label: label})
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
func (p *CodeBuilder) For(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("For")
	}
	stmt := &forStmt{}
	p.startBlockStmt(stmt, src, "for statement", &stmt.old)
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
	return p.ForRangeEx(names)
}

// ForRangeEx func
func (p *CodeBuilder) ForRangeEx(names []string, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("ForRange", names)
	}
	stmt := &forRangeStmt{names: names}
	p.startBlockStmt(stmt, src, "for range statement", &stmt.old)
	return p
}

// RangeAssignThen func
func (p *CodeBuilder) RangeAssignThen(pos token.Pos) *CodeBuilder {
	if debugInstr {
		log.Println("RangeAssignThen")
	}
	if flow, ok := p.current.codeBlock.(*forRangeStmt); ok {
		flow.RangeAssignThen(p, pos)
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
		if e := p.stk.Pop(); p.noSkipConst || e.CVal == nil { // skip constant
			p.emitStmt(&ast.ExprStmt{X: e.Val})
		}
	}
	return p
}

// End func
func (p *CodeBuilder) End(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		typ := reflect.TypeOf(p.current.codeBlock)
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		name := strings.TrimSuffix(strings.Title(typ.Name()), "Stmt")
		log.Println("End //", name)
		if p.stk.Len() > p.current.base {
			panic("forget to call EndStmt()?")
		}
	}
	p.current.End(p, getSrc(src))
	return p
}

func (p *CodeBuilder) SetBodyHandler(handle func(body *ast.BlockStmt, kind int)) *CodeBuilder {
	if ini, ok := p.current.codeBlock.(interface {
		SetBodyHandler(func(body *ast.BlockStmt, kind int))
	}); ok {
		ini.SetBodyHandler(handle)
	}
	return p
}

// ResetInit resets the variable init state of CodeBuilder.
func (p *CodeBuilder) ResetInit() {
	if debugInstr {
		log.Println("ResetInit")
	}
	p.valDecl = p.valDecl.resetInit(p)
}

// EndInit func
func (p *CodeBuilder) EndInit(n int) *CodeBuilder {
	if debugInstr {
		log.Println("EndInit", n)
	}
	p.valDecl = p.valDecl.endInit(p, n)
	return p
}

// Debug func
func (p *CodeBuilder) Debug(dbg func(cb *CodeBuilder)) *CodeBuilder {
	dbg(p)
	return p
}

// Get func
func (p *CodeBuilder) Get(idx int) *Element {
	return p.stk.Get(idx)
}

// ----------------------------------------------------------------------------

type InternalStack = internal.Stack

// InternalStack: don't call it (only for internal use)
func (p *CodeBuilder) InternalStack() *InternalStack {
	return &p.stk
}

// ----------------------------------------------------------------------------
