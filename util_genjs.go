//go:build genjs

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
	"github.com/goplus/gogen/target"
	"github.com/goplus/gogen/target/js"
)

// ----------------------------------------------------------------------------

func (p *CodeBuilder) emitMapStringAnyAssert(argVal js.Expr) js.Expr {
	panic("todo emitMapStringAnyAssert")
}

// TypeAssert func
func (p *CodeBuilder) TypeAssert(typ types.Type, lhs int, src ...ast.Node) *CodeBuilder {
	panic("todo TypeAssert")
}

func (p *CodeBuilder) mapIndexExpr(o *types.Map, name string, lhs int, argVal js.Expr, src ast.Node) MemberKind {
	panic("todo mapIndexExpr")
}

// MapLitEx func
func (p *CodeBuilder) MapLitEx(typ types.Type, arity int, src ...ast.Node) error {
	panic("todo MapLitEx")
}

// SliceLitEx func
func (p *CodeBuilder) SliceLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo SliceLitEx")
}

// ArrayLitEx func
func (p *CodeBuilder) ArrayLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo ArrayLitEx")
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	panic("todo StructLit")
}

// Slice func
func (p *CodeBuilder) Slice(slice3 bool, src ...ast.Node) *CodeBuilder { // a[i:j:k]
	panic("todo Slice")
}

// Index func:
//   - a[i]
//   - fn[T1, T2, ..., Tn]
//   - G[T1, T2, ..., Tn]
func (p *CodeBuilder) Index(nidx int, lhs int, src ...ast.Node) *CodeBuilder {
	panic("todo Index")
}

// Star func
func (p *CodeBuilder) Star(src ...ast.Node) *CodeBuilder {
	panic("todo Star")
}

// Elem func
func (p *CodeBuilder) Elem(src ...ast.Node) *CodeBuilder {
	panic("todo Elem")
}

// ElemRef func
func (p *CodeBuilder) ElemRef(src ...ast.Node) *CodeBuilder {
	panic("todo ElemRef")
}

func (p *CodeBuilder) doVarRef(ref any, src ast.Node, allowDebug bool) *CodeBuilder {
	panic("todo doVarRef")
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(pos, end token.Pos, name string, pv **types.Var) *CodeBuilder {
	panic("todo NewAutoVar")
}

func methodToFuncSig(pkg *Package, o types.Object, fn *Element) *types.Signature {
	panic("todo methodToFuncSig")
}

func (p *CodeBuilder) methodSigOf(typ types.Type, flag MemberFlag, arg, ret *Element) (types.Type, bool) {
	panic("todo methodSigOf")
}

func boundTypeParams(p *Package, fn *Element, sig *types.Signature, args []*Element, flags InstrFlags) (*Element, *types.Signature, []*Element, error) {
	panic("todo boundTypeParams")
}

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	panic("todo instantiate")
}

func instanceInferFunc(pkg *Package, arg *internal.Elem, tsig *inferFuncType, sig *types.Signature) error {
	panic("todo instanceInferFunc")
}

func instanceFunc(pkg *Package, arg *internal.Elem, tsig *types.Signature, sig *types.Signature) error {
	panic("todo instanceFunc")
}

func checkInferArgs(pkg *Package, fn *internal.Elem, sig *types.Signature, args []*internal.Elem, flags InstrFlags) ([]*internal.Elem, error) {
	panic("todo checkInferArgs")
}

func getFunExpr(fn *internal.Elem) (caller string, pos, end token.Pos) {
	panic("todo getFunExpr")
}

func getCaller(expr *internal.Elem) string {
	panic("todo getCaller")
}

// NewAndInit creates variables with specified `typ` (can be nil) and `names`, and
// initializes them by `fn` (can be nil). When `fn` is nil (no initialization),
// `typ` must not be nil. When names is empty, creates an embedded field.
func (p *ClassDefs) NewAndInit(fn F, pos token.Pos, typ types.Type, names ...string) {
	panic("todo NewAndInit")
}

// ----------------------------------------------------------------------------

func newIotaExpr(v int) js.Expr {
	panic("todo newIotaExpr")
}

func newAppendStringExpr(args []*internal.Elem) js.Expr {
	panic("todo newAppendStringExpr")
}

func newLenExpr(args []*internal.Elem) js.Expr {
	panic("todo newLenExpr")
}

func newCapExpr(args []*internal.Elem) js.Expr {
	panic("todo newCapExpr")
}

func newNewExpr(args []*internal.Elem) js.Expr {
	panic("todo newNewExpr")
}

func newMakeExpr(args []*internal.Elem) js.Expr {
	panic("todo newMakeExpr")
}

func newSizeofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo newSizeofExpr")
}

func newAlignofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo newAlignofExpr")
}

func newOffsetofExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo newOffsetofExpr")
}

func newUnsafeAddExpr(pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo newUnsafeAddExpr")
}

func newUnsafeDataExpr(p unsafeDataInstr, pkg *Package, args []*internal.Elem) js.Expr {
	panic("todo newUnsafeDataExpr")
}

func newRecvExpr(args []*internal.Elem) js.Expr {
	panic("todo newRecvExpr")
}

func newAddrExpr(args []*internal.Elem) js.Expr {
	panic("todo newAddrExpr")
}

func zeroCompositeLit(p *Package, typ types.Type, typ0 *types.Type) js.Expr {
	panic("todo zeroCompositeLit")
}

func newFuncLit(pkg *Package, t *types.Signature, body *js.BlockStmt) *js.FuncLit {
	panic("todo newFuncLit")
}

func newCommentedNodes(p *Package, f *ast.File) *printer.CommentedNodes {
	return &printer.CommentedNodes{
		Node: f,
	}
}

// ----------------------------------------------------------------------------

func newIncDecStmt(x js.Expr, tok token.Token) js.Stmt {
	panic("todo newIncDecStmt")
}

func newAssignOpStmt(tok token.Token, args []*internal.Elem) js.Stmt {
	panic("todo newAssignOpStmt")
}

func emitAssignStmt(cb *CodeBuilder, stmt *target.AssignStmt) {
	panic("todo emitAssignStmt")
}

func commitAssignStmt(cb *CodeBuilder, p *ValueDecl) {
	panic("todo commitAssignStmt")
}

func emitTypeDeclStmt(cb *CodeBuilder, decl *typeDecl) {
	panic("todo emitTypeDeclStmt")
}

func emitGoStmt(cb *CodeBuilder, call *js.CallExpr) {
	panic("todo emitGoStmt")
}

func emitDeferStmt(cb *CodeBuilder, call *js.CallExpr) {
	panic("todo emitDeferStmt")
}

func emitSendStmt(cb *CodeBuilder, ch, val js.Expr) {
	panic("todo emitSendStmt")
}

func emitGotoStmt(cb *CodeBuilder, name string) {
	panic("todo emitGotoStmt")
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
	if p.init != nil {
		cb.emitStmt(p.init)
	}
	cb.emitStmt(&js.IfStmt{Cond: p.cond, Body: p.body, Else: el})
}

func emitSWitchStmt(cb *CodeBuilder, p *switchStmt, stmts []js.Stmt) {
	panic("todo emitSWitchStmt")
}

func emitFullthrough(cb *CodeBuilder) {
	panic("todo emitFullthrough")
}

func emitCaseClause(cb *CodeBuilder, p *caseStmt, body []js.Stmt) {
	panic("todo emitCaseClause")
}

func emitSelectStmt(cb *CodeBuilder, stmts []js.Stmt) {
	panic("todo emitSelectStmt")
}

func emitCommClause(cb *CodeBuilder, p *commCase, body []js.Stmt) {
	panic("todo emitCommClause")
}

func emitTypeSwitchStmt(cb *CodeBuilder, p *typeSwitchStmt, stmts []js.Stmt) {
	panic("todo emitTypeSwitchStmt")
}

func emitTypeCaseClause(cb *CodeBuilder, p *typeCaseStmt, body []js.Stmt) {
	panic("todo emitTypeCaseClause")
}

func emitForRangeStmt(cb *CodeBuilder, p *forRangeStmt, stmts []js.Stmt, flows int) {
	panic("todo emitForRangeStmt")
}

// ----------------------------------------------------------------------------
