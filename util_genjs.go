//go:build genjs
// +build genjs

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

// ----------------------------------------------------------------------------

func (p *CodeBuilder) emitMapStringAnyAssert(argVal js.Expr) js.Expr {
	panic("todo")
}

// TypeAssert func
func (p *CodeBuilder) TypeAssert(typ types.Type, lhs int, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

func (p *CodeBuilder) mapIndexExpr(o *types.Map, name string, lhs int, argVal js.Expr, src ast.Node) MemberKind {
	panic("todo")
}

// MapLitEx func
func (p *CodeBuilder) MapLitEx(typ types.Type, arity int, src ...ast.Node) error {
	panic("todo")
}

// SliceLitEx func
func (p *CodeBuilder) SliceLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// ArrayLitEx func
func (p *CodeBuilder) ArrayLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// Slice func
func (p *CodeBuilder) Slice(slice3 bool, src ...ast.Node) *CodeBuilder { // a[i:j:k]
	panic("todo")
}

// Index func:
//   - a[i]
//   - fn[T1, T2, ..., Tn]
//   - G[T1, T2, ..., Tn]
func (p *CodeBuilder) Index(nidx int, lhs int, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// Star func
func (p *CodeBuilder) Star(src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// Elem func
func (p *CodeBuilder) Elem(src ...ast.Node) *CodeBuilder {
	panic("todo")
}

// ElemRef func
func (p *CodeBuilder) ElemRef(src ...ast.Node) *CodeBuilder {
	panic("todo")
}

func (p *CodeBuilder) doVarRef(ref any, src ast.Node, allowDebug bool) *CodeBuilder {
	panic("todo")
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(pos, end token.Pos, name string, pv **types.Var) *CodeBuilder {
	panic("todo")
}

func methodToFuncSig(pkg *Package, o types.Object, fn *Element) *types.Signature {
	panic("todo")
}

func (p *CodeBuilder) methodSigOf(typ types.Type, flag MemberFlag, arg, ret *Element) (types.Type, bool) {
	panic("todo")
}

func boundTypeParams(p *Package, fn *Element, sig *types.Signature, args []*Element, flags InstrFlags) (*Element, *types.Signature, []*Element, error) {
	panic("todo")
}

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	panic("todo")
}

func instanceInferFunc(pkg *Package, arg *internal.Elem, tsig *inferFuncType, sig *types.Signature) error {
	panic("todo")
}

func instanceFunc(pkg *Package, arg *internal.Elem, tsig *types.Signature, sig *types.Signature) error {
	panic("todo")
}

func checkInferArgs(pkg *Package, fn *internal.Elem, sig *types.Signature, args []*internal.Elem, flags InstrFlags) ([]*internal.Elem, error) {
	panic("todo")
}

func getFunExpr(fn *internal.Elem) (caller string, pos, end token.Pos) {
	panic("todo")
}

func getCaller(expr *internal.Elem) string {
	panic("todo")
}

// NewAndInit creates variables with specified `typ` (can be nil) and `names`, and
// initializes them by `fn` (can be nil). When `fn` is nil (no initialization),
// `typ` must not be nil. When names is empty, creates an embedded field.
func (p *ClassDefs) NewAndInit(fn F, pos token.Pos, typ types.Type, names ...string) {
	panic("todo")
}

// ----------------------------------------------------------------------------

type jsDecl interface {
	declNode()
}

type funcDecl struct {
	ast.FuncDecl
	sig  *types.Signature
	Body *js.BlockStmt
}

func (*funcDecl) declNode() {}

type valueSpec struct {
	ast.ValueSpec
	Values []js.Expr
}

func asValueSpec(spec *valueSpec) *valueSpec {
	return spec
}

type valDecl struct {
	ast.GenDecl
	Specs []*valueSpec
}

func (*valDecl) declNode() {}

type typeDecl struct {
	ast.GenDecl
}

func (*typeDecl) declNode() {}

type fileDecls struct {
	goDecls []ast.Decl
	jsDecls []jsDecl
}

func (p *fileDecls) appendFuncDecl(decl *funcDecl, sig *types.Signature) {
	decl.sig = sig
	p.goDecls = append(p.goDecls, &decl.FuncDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

func (p *fileDecls) appendValDecl(decl *valDecl) {
	p.goDecls = append(p.goDecls, &decl.GenDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

func startValDeclStmtAt(cb *CodeBuilder, decl *valDecl) int {
	panic("todo")
}

func (p *fileDecls) appendTypeDecl(decl *typeDecl) {
	p.goDecls = append(p.goDecls, &decl.GenDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

func (p *File) getJSFile(_ *Package) *js.File {
	decls := make([]js.Stmt, 0, len(p.jsDecls))
	for _, decl := range p.jsDecls {
		switch d := decl.(type) {
		case *funcDecl:
			sig := d.sig
			if sig.Recv() != nil {
				panic("todo")
			}
			in := sig.Params()
			n := in.Len()
			params := make([]*js.Ident, n)
			for i := range n {
				params[i] = &js.Ident{Name: in.At(i).Name()}
			}
			decls = append(decls, &js.FuncDecl{
				Recv:   nil,
				Name:   &js.Ident{Name: d.Name.Name},
				Params: params,
				Body:   d.Body,
			})
		}
	}
	return &js.File{Stmts: decls}
}

// ----------------------------------------------------------------------------

func newIotaExpr(v int) js.Expr {
	panic("todo")
}

func newAppendStringExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newLenExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newCapExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newNewExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newMakeExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newSizeofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo")
}

func newAlignofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo")
}

func newOffsetofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo")
}

func newUnsafeAddExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo")
}

func newUnsafeDataExpr(p unsafeDataInstr, pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo")
}

func newRecvExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func newAddrExpr(args []*internal.Elem) js.Expr {
	panic("todo")
}

func zeroCompositeLit(p *Package, typ types.Type, typ0 *types.Type) js.Expr {
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

// ----------------------------------------------------------------------------

func newIncDecStmt(x js.Expr, tok token.Token) js.Stmt {
	panic("todo")
}

func newAssignOpStmt(tok token.Token, args []*internal.Elem) js.Stmt {
	panic("todo")
}

func emitAssignStmt(cb *CodeBuilder, stmt *target.AssignStmt) {
	panic("todo")
}

func commitAssignStmt(cb *CodeBuilder, p *ValueDecl) {
	panic("todo")
}

func emitTypeDeclStmt(cb *CodeBuilder, decl *typeDecl) {
	panic("todo")
}

func emitGoStmt(cb *CodeBuilder, call *js.CallExpr) {
	panic("todo")
}

func emitDeferStmt(cb *CodeBuilder, call *js.CallExpr) {
	panic("todo")
}

func emitSendStmt(cb *CodeBuilder, ch, val js.Expr) {
	panic("todo")
}

func emitGotoStmt(cb *CodeBuilder, name string) {
	panic("todo")
}

func emitReturnStmt(cb *CodeBuilder, pos token.Pos, rets ...js.Expr) {
	ret := &js.ReturnStmt{}
	switch len(rets) {
	case 0:
	case 1:
		ret.Result = rets[0]
	default:
		ret.Result = &js.ArrayLit{Values: rets}
	}
	cb.emitStmt(ret)
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

func emitForRangeStmt(cb *CodeBuilder, p *forRangeStmt, stmts []js.Stmt, flows int) {
	panic("todo")
}

// ----------------------------------------------------------------------------
