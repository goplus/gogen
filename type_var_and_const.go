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
	"go/token"
	"go/types"
	"log"
	"reflect"
	"syscall"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

// ConstStart starts a constant expression.
func (p *Package) ConstStart() *CodeBuilder {
	return &p.cb
}

func (p *CodeBuilder) EndConst() *Element {
	return p.stk.Pop()
}

// ----------------------------------------------------------------------------

type TyState int

const (
	TyStateUninited TyState = iota
	TyStateInited
	TyStateDeleted
)

// TypeDecl type
type TypeDecl struct {
	typ   *types.Named
	decl  *ast.GenDecl
	scope *types.Scope
}

// SetComments sets associated documentation.
func (p *TypeDecl) SetComments(doc *ast.CommentGroup) *TypeDecl {
	p.decl.Doc = doc
	return p
}

// Type returns the type.
func (p *TypeDecl) Type() *types.Named {
	return p.typ
}

// State checkes state of this type.
// If Delete is called, it returns TyStateDeleted.
// If InitType is called (but not deleted), it returns TyStateInited.
// Otherwise it returns TyStateUninited.
func (p *TypeDecl) State() TyState {
	if spec := p.decl.Specs; len(spec) > 0 {
		if spec[0].(*ast.TypeSpec).Type != nil {
			return TyStateInited
		}
		return TyStateUninited
	}
	return TyStateDeleted
}

// Delete deletes this type.
// NOTE: It panics if you call InitType after Delete.
func (p *TypeDecl) Delete() {
	p.decl.Specs = p.decl.Specs[:0]
}

// Inited checkes if InitType is called or not.
// Will panic if this type is deleted (please use State to check).
func (p *TypeDecl) Inited() bool {
	return p.decl.Specs[0].(*ast.TypeSpec).Type != nil
}

// InitType initializes a uncompleted type.
func (p *TypeDecl) InitType(pkg *Package, typ types.Type, tparams ...*TypeParam) *types.Named {
	if debugInstr {
		log.Println("InitType", p.typ.Obj().Name(), typ)
	}
	spec := p.decl.Specs[0].(*ast.TypeSpec)
	if spec.Type != nil {
		log.Panicln("TODO: type already defined -", typ)
	}
	if named, ok := typ.(*types.Named); ok {
		p.typ.SetUnderlying(pkg.cb.getUnderlying(named))
	} else {
		p.typ.SetUnderlying(typ)
	}
	setTypeParams(pkg, p.typ, spec, tparams)
	spec.Type = toType(pkg, typ)
	pkg.appendGenDecl(p.scope, p.decl)
	return p.typ
}

// AliasType gives a specified type with a new name
func (p *Package) AliasType(name string, typ types.Type, pos ...token.Pos) *types.Named {
	if debugInstr {
		log.Println("AliasType", name, typ)
	}
	decl := p.doNewType(p.Types.Scope(), getPos(pos), name, typ, 1)
	return decl.typ
}

// NewType creates a new type (which need to call InitType later).
func (p *Package) NewType(name string, pos ...token.Pos) *TypeDecl {
	if debugInstr {
		log.Println("NewType", name)
	}
	return p.doNewType(p.Types.Scope(), getPos(pos), name, nil, 0)
}

func getPos(pos []token.Pos) token.Pos {
	if pos == nil {
		return 0
	}
	return pos[0]
}

func (p *Package) appendGenDecl(scope *types.Scope, decl *ast.GenDecl) {
	if scope == p.Types.Scope() {
		p.file.decls = append(p.file.decls, decl)
	} else {
		p.cb.emitStmt(&ast.DeclStmt{Decl: decl})
	}
}

func (p *Package) doNewType(
	scope *types.Scope, pos token.Pos, name string, typ types.Type, alias token.Pos) *TypeDecl {
	typName := types.NewTypeName(pos, p.Types, name, typ)
	if old := scope.Insert(typName); old != nil {
		oldPos := p.cb.position(old.Pos())
		p.cb.panicCodePosErrorf(
			pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldPos)
	}
	spec := &ast.TypeSpec{Name: ident(name), Assign: alias}
	decl := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
	if alias != 0 { // alias don't need to call InitType
		spec.Type = toType(p, typ)
		typ = typ.Underlying() // typ.Underlying() may delay load and can be nil, it's reasonable
		p.appendGenDecl(scope, decl)
	}
	named := types.NewNamed(typName, typ, nil)
	return &TypeDecl{typ: named, decl: decl, scope: scope}
}

// ----------------------------------------------------------------------------

// ValueDecl type
type ValueDecl struct {
	names []string
	typ   types.Type
	old   codeBlock
	oldv  *ValueDecl
	scope *types.Scope
	vals  *[]ast.Expr
	tok   token.Token
	pos   token.Pos
	at    int // commitStmt(at)
}

// Inited checkes if `InitStart` is called or not.
func (p *ValueDecl) Inited() bool {
	return p.oldv != nil
}

// InitStart initializes a uninitialized variable or constant.
func (p *ValueDecl) InitStart(pkg *Package) *CodeBuilder {
	p.oldv, pkg.cb.valDecl = pkg.cb.valDecl, p
	p.old = pkg.cb.startInitExpr(p)
	return &pkg.cb
}

func (p *ValueDecl) Ref(name string) Ref {
	return p.scope.Lookup(name)
}

// End is provided for internal usage.
// Don't call it at any time. Please use (*CodeBuilder).EndInit instead.
func (p *ValueDecl) End(cb *CodeBuilder, src ast.Node) {
	fatal("don't call End(), please use EndInit() instead")
}

func (p *ValueDecl) resetInit(cb *CodeBuilder) *ValueDecl {
	cb.endInitExpr(p.old)
	if p.at >= 0 {
		cb.commitStmt(p.at) // to support inline call, we must emitStmt at ResetInit stage
	}
	return p.oldv
}

func checkTuple(t **types.Tuple, typ types.Type) (ok bool) {
	*t, ok = typ.(*types.Tuple)
	return
}

func (p *ValueDecl) endInit(cb *CodeBuilder, arity int) *ValueDecl {
	var t *types.Tuple
	var values []ast.Expr
	n := len(p.names)
	rets := cb.stk.GetArgs(arity)
	defer func() {
		cb.stk.PopN(arity)
		cb.endInitExpr(p.old)
		if p.at >= 0 {
			cb.commitStmt(p.at) // to support inline call, we must emitStmt at EndInit stage
		}
	}()
	if arity == 1 && checkTuple(&t, rets[0].Type) {
		if n != t.Len() {
			caller := cb.getCaller(rets[0].Src)
			cb.panicCodePosErrorf(
				p.pos, "assignment mismatch: %d variables but %s returns %d values", n, caller, t.Len())
		}
		*p.vals = []ast.Expr{rets[0].Val}
		rets = make([]*internal.Elem, n)
		for i := 0; i < n; i++ {
			rets[i] = &internal.Elem{Type: t.At(i).Type()}
		}
	} else if n != arity {
		if p.tok == token.CONST {
			if n > arity {
				cb.panicCodePosError(p.pos, "missing value in const declaration")
			}
			cb.panicCodePosError(p.pos, "extra expression in const declaration")
		}
		cb.panicCodePosErrorf(p.pos, "assignment mismatch: %d variables but %d values", n, arity)
	} else {
		values = make([]ast.Expr, arity)
		for i, ret := range rets {
			values[i] = ret.Val
		}
		*p.vals = values
	}
	pkg, typ := cb.pkg, p.typ
	if typ != nil {
		for i, ret := range rets {
			if err := matchType(pkg, ret, typ, "assignment"); err != nil {
				panic(err)
			}
			if values != nil { // ret.Val may be changed
				values[i] = ret.Val
			}
		}
	}
	for i, name := range p.names {
		if name == "_" { // skip underscore
			continue
		}
		if p.tok == token.CONST {
			tv := rets[i]
			if tv.CVal == nil {
				src, _ := cb.loadExpr(tv.Src)
				cb.panicCodePosErrorf(
					p.pos, "const initializer %s is not a constant", src)
			}
			tvType := typ
			if tvType == nil {
				tvType = tv.Type
			}
			if old := p.scope.Insert(types.NewConst(p.pos, pkg.Types, name, tvType, tv.CVal)); old != nil {
				oldpos := cb.position(old.Pos())
				cb.panicCodePosErrorf(
					p.pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
			}
		} else if typ == nil {
			var retType = rets[i].Type
			var parg *Element
			if values != nil {
				parg = &Element{Type: retType, Val: values[i]}
			}
			retType = DefaultConv(pkg, retType, parg)
			if values != nil {
				values[i] = parg.Val
			}
			if old := p.scope.Insert(types.NewVar(p.pos, pkg.Types, name, retType)); old != nil {
				if p.tok != token.DEFINE {
					oldpos := cb.position(old.Pos())
					cb.panicCodePosErrorf(
						p.pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
				}
				if err := matchType(pkg, rets[i], old.Type(), "assignment"); err != nil {
					panic(err)
				}
			}
		}
	}
	return p.oldv
}

// VarDecl type
type VarDecl = ValueDecl

func (p *Package) newValueDecl(
	vdecl *ValueDefs, scope *types.Scope, pos token.Pos, tok token.Token, typ types.Type, names ...string) *ValueDecl {
	n := len(names)
	if tok == token.DEFINE { // a, b := expr
		noNewVar := true
		nameIdents := make([]ast.Expr, n)
		for i, name := range names {
			nameIdents[i] = ident(name)
			if noNewVar && scope.Lookup(name) == nil {
				noNewVar = false
			}
		}
		if noNewVar {
			err := p.cb.newCodePosError(pos, "no new variables on left side of :=")
			p.cb.handleErr(err)
		}
		stmt := &ast.AssignStmt{Tok: token.DEFINE, Lhs: nameIdents}
		at := p.cb.startStmtAt(stmt)
		return &ValueDecl{names: names, tok: tok, pos: pos, scope: scope, vals: &stmt.Rhs, at: at}
	}
	// var a, b = expr
	// const a, b = expr
	nameIdents := make([]*ast.Ident, n)
	for i, name := range names {
		nameIdents[i] = ident(name)
		if name == "_" { // skip underscore
			continue
		}
		if typ != nil && tok == token.VAR {
			if old := scope.Insert(types.NewVar(pos, p.Types, name, typ)); old != nil {
				allowRedecl := p.allowRedecl && scope == p.Types.Scope()
				if !(allowRedecl && types.Identical(old.Type(), typ)) { // for c2go
					oldpos := p.cb.position(old.Pos())
					p.cb.panicCodePosErrorf(
						pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
				}
			}
		}
	}
	spec := &ast.ValueSpec{Names: nameIdents}
	if typ != nil {
		if ut, ok := typ.(*unboundType); ok && ut.tBound == nil {
			ut.ptypes = append(ut.ptypes, &spec.Type)
		} else {
			spec.Type = toType(p, typ)
		}
	}
	at := -1
	if vdecl != nil {
		decl := vdecl.decl
		decl.Specs = append(decl.Specs, spec)
	} else {
		decl := &ast.GenDecl{Tok: tok, Specs: []ast.Spec{spec}}
		if scope == p.Types.Scope() {
			p.file.decls = append(p.file.decls, decl)
		} else {
			at = p.cb.startStmtAt(&ast.DeclStmt{Decl: decl})
		}
	}
	return &ValueDecl{
		typ: typ, names: names, tok: tok, pos: pos, scope: scope, vals: &spec.Values, at: at}
}

func (p *Package) newValueDefs(scope *types.Scope, tok token.Token) *ValueDefs {
	at := -1
	decl := &ast.GenDecl{Tok: tok}
	if scope == p.Types.Scope() {
		p.file.decls = append(p.file.decls, decl)
	} else {
		at = p.cb.startStmtAt(&ast.DeclStmt{Decl: decl})
	}
	return &ValueDefs{pkg: p, scope: scope, decl: decl, at: at}
}

// NewConstStart creates constants with names.
//
// Deprecated: Use NewConstDefs instead.
func (p *Package) NewConstStart(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewConst", names)
	}
	return p.newValueDecl(nil, scope, pos, token.CONST, typ, names...).InitStart(p)
}

// NewConstDecl starts a constant declaration block.
//
// Deprecated: Use NewConstDefs instead.
func (p *Package) NewConstDecl(scope *types.Scope) *ConstDefs {
	return p.NewConstDefs(scope)
}

// NewConstDefs starts a constant declaration block.
func (p *Package) NewConstDefs(scope *types.Scope) *ConstDefs {
	if debugInstr {
		log.Println("NewConstDefs")
	}
	return &ConstDefs{ValueDefs: *p.newValueDefs(scope, token.CONST)}
}

// NewVar starts a var declaration block and creates uninitialized variables with
// specified `typ` (can be nil) and `names`.
//
// This is a shortcut for creating variables. `NewVarDefs` is more powerful and
// more recommended.
func (p *Package) NewVar(pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, p.Types.Scope(), pos, token.VAR, typ, names...)
}

// NewVarEx starts a var declaration block and creates uninitialized variables with
// specified `typ` (can be nil) and `names`.
//
// This is a shortcut for creating variables. `NewVarDefs` is more powerful and
// more recommended.
func (p *Package) NewVarEx(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, scope, pos, token.VAR, typ, names...)
}

// NewVarStart creates variables with specified `typ` (can be nil) and `names` and starts
// to initialize them. You should call `CodeBuilder.EndInit` to end initialization.
//
// This is a shortcut for creating variables. `NewVarDefs` is more powerful and more
// recommended.
func (p *Package) NewVarStart(pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, p.Types.Scope(), pos, token.VAR, typ, names...).InitStart(p)
}

// NewVarDefs starts a var declaration block.
func (p *Package) NewVarDefs(scope *types.Scope) *VarDefs {
	if debugInstr {
		log.Println("NewVarDefs")
	}
	return (*VarDefs)(p.newValueDefs(scope, token.VAR))
}

// ----------------------------------------------------------------------------

type ValueDefs struct {
	decl  *ast.GenDecl
	scope *types.Scope
	pkg   *Package
	at    int
}

type VarDefs ValueDefs

// New creates uninitialized variables with specified `typ` (can be nil) and `names`.
func (p *VarDefs) New(pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.pkg.newValueDecl((*ValueDefs)(p), p.scope, pos, token.VAR, typ, names...)
}

// NewAndInit creates variables with specified `typ` (can be nil) and `names`, and initializes them by `fn`.
func (p *VarDefs) NewAndInit(fn func(cb *CodeBuilder) int, pos token.Pos, typ types.Type, names ...string) *VarDefs {
	if debugInstr {
		log.Println("NewAndInit", names)
	}
	decl := p.pkg.newValueDecl((*ValueDefs)(p), p.scope, pos, token.VAR, typ, names...)
	if fn != nil {
		cb := decl.InitStart(p.pkg)
		n := fn(cb)
		cb.EndInit(n)
	}
	return p
}

// Delete deletes an uninitialized variable who was created by `New`.
// If the variable is initialized, it fails to delete and returns `syscall.EACCES`.
// If the variable is not found, it returns `syscall.ENOENT`.
func (p *VarDefs) Delete(name string) error {
	for i, spec := range p.decl.Specs {
		vspec := spec.(*ast.ValueSpec)
		for j, ident := range vspec.Names {
			if ident.Name == name {
				if vspec.Values != nil { // can't remove an initialized variable
					return syscall.EACCES
				}
				if len(vspec.Names) == 1 {
					p.decl.Specs = append(p.decl.Specs[:i], p.decl.Specs[i+1:]...)
					return nil
				}
				vspec.Names = append(vspec.Names[:j], vspec.Names[j+1:]...)
				return nil
			}
		}
	}
	return syscall.ENOENT
}

// ----------------------------------------------------------------------------

type ConstDefs struct {
	ValueDefs
	fn  func(cb *CodeBuilder) int
	typ types.Type
}

func constInitFn(cb *CodeBuilder, iotav int, fn func(cb *CodeBuilder) int) int {
	oldv := cb.iotav
	cb.iotav = iotav
	defer func() {
		cb.iotav = oldv
	}()
	return fn(cb)
}

func (p *ConstDefs) New(
	fn func(cb *CodeBuilder) int, iotav int, pos token.Pos, typ types.Type, names ...string) *ConstDefs {
	if debugInstr {
		log.Println("NewConst", names, iotav)
	}
	pkg := p.pkg
	cb := pkg.newValueDecl(&p.ValueDefs, p.scope, pos, token.CONST, typ, names...).InitStart(pkg)
	n := constInitFn(cb, iotav, fn)
	cb.EndInit(n)
	p.fn, p.typ = fn, typ
	return p
}

func (p *ConstDefs) Next(iotav int, pos token.Pos, names ...string) *ConstDefs {
	pkg := p.pkg
	cb := pkg.CB()
	n := constInitFn(cb, iotav, p.fn)
	if len(names) != n {
		if len(names) < n {
			cb.panicCodePosError(pos, "extra expression in const declaration")
		}
		cb.panicCodePosError(pos, "missing value in const declaration")
	}

	ret := cb.stk.GetArgs(n)
	defer cb.stk.PopN(n)

	idents := make([]*ast.Ident, n)
	for i, name := range names {
		typ := p.typ
		if typ == nil {
			typ = ret[i].Type
		}
		if name != "_" {
			if old := p.scope.Insert(types.NewConst(pos, pkg.Types, name, typ, ret[i].CVal)); old != nil {
				oldpos := cb.position(old.Pos())
				cb.panicCodePosErrorf(
					pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
			}
		}
		idents[i] = ident(name)
	}
	spec := &ast.ValueSpec{Names: idents}
	p.decl.Specs = append(p.decl.Specs, spec)
	return p
}

// ConstDecl type
//
// Deprecated: Use ConstDefs instead.
type ConstDecl = ConstDefs

// ----------------------------------------------------------------------------

var (
	TyByte = types.Universe.Lookup("byte").Type().(*types.Basic)
	TyRune = types.Universe.Lookup("rune").Type().(*types.Basic)
)

var (
	TyEmptyInterface = types.NewInterfaceType(nil, nil)
	TyError          = types.Universe.Lookup("error").Type()
)

// refType: &T
type refType struct {
	typ types.Type
}

func (p *refType) Elem() types.Type {
	return p.typ
}

func (p *refType) Underlying() types.Type {
	fatal("ref type")
	return nil
}

func (p *refType) String() string {
	return fmt.Sprintf("refType{typ: %v}", p.typ)
}

func DerefType(typ types.Type) (types.Type, bool) {
	switch t := typ.(type) {
	case *refType:
		return t.Elem(), true
	case *bfRefType:
		return t.typ, true
	}
	return typ, false
}

// bfRefType: bit field refType
type bfRefType struct {
	typ  *types.Basic
	off  int
	bits int
}

func (p *bfRefType) Underlying() types.Type {
	fatal("bit field refType")
	return nil
}

func (p *bfRefType) String() string {
	return fmt.Sprintf("bfRefType{typ: %v:%d off: %d}", p.typ, p.bits, p.off)
}

// unboundType: unbound type
type unboundType struct {
	tBound types.Type
	ptypes []*ast.Expr
}

func (p *unboundType) boundTo(pkg *Package, arg types.Type) {
	if p.tBound != nil {
		fatal("TODO: type is already bounded")
	}
	p.tBound = arg
	for _, pt := range p.ptypes {
		*pt = toType(pkg, arg)
	}
	p.ptypes = nil
}

func (p *unboundType) Underlying() types.Type {
	fatal("unbound type")
	return nil
}

func (p *unboundType) String() string {
	return fmt.Sprintf("unboundType{typ: %v}", p.tBound)
}

func realType(typ types.Type) types.Type {
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound != nil {
			return t.tBound
		}
	case *types.Named:
		if tn := t.Obj(); tn.IsAlias() {
			return tn.Type()
		}
	}
	return typ
}

type unboundMapElemType struct {
	key types.Type
	typ *unboundType
}

func (p *unboundMapElemType) Underlying() types.Type {
	fatal("unbound map elem type")
	return nil
}

func (p *unboundMapElemType) String() string {
	return fmt.Sprintf("unboundMapElemType{key: %v}", p.key)
}

// ----------------------------------------------------------------------------

type btiMethodType struct {
	types.Type
	eargs []interface{}
}

// ----------------------------------------------------------------------------

type instructionType struct {
	instr Instruction
}

func (p *instructionType) Underlying() types.Type {
	fatal("instruction type")
	return nil
}

func (p *instructionType) String() string {
	return fmt.Sprintf("instructionType{instr: %v}", reflect.TypeOf(p.instr))
}

func isType(t types.Type) bool {
	switch sig := t.(type) {
	case *types.Signature:
		if _, ok := CheckOverloadFunc(sig); ok {
			// builtin may be implemented as OverloadFunc
			return false
		}
	case *instructionType:
		return false
	}
	return true
}

// ----------------------------------------------------------------------------

type TypeType struct {
	typ types.Type
}

func NewTypeType(typ types.Type) *TypeType {
	return &TypeType{typ: typ}
}

func (p *TypeType) Pointer() *TypeType {
	return &TypeType{typ: types.NewPointer(p.typ)}
}

func (p *TypeType) Type() types.Type {
	return p.typ
}

func (p *TypeType) Underlying() types.Type {
	fatal("type of type")
	return nil
}

func (p *TypeType) String() string {
	return fmt.Sprintf("TypeType{typ: %v}", p.typ)
}

// ----------------------------------------------------------------------------

type SubstType struct {
	Real types.Object
}

func (p *SubstType) Underlying() types.Type {
	fatal("substitute type")
	return nil
}

func (p *SubstType) String() string {
	return fmt.Sprintf("substType{real: %v}", p.Real)
}

func NewSubst(pos token.Pos, pkg *types.Package, name string, real types.Object) *types.Var {
	return types.NewVar(pos, pkg, name, &SubstType{Real: real})
}

func LookupParent(scope *types.Scope, name string, pos token.Pos) (at *types.Scope, obj types.Object) {
	if at, obj = scope.LookupParent(name, pos); obj != nil {
		if t, ok := obj.Type().(*SubstType); ok {
			obj = t.Real
		}
	}
	return
}

func Lookup(scope *types.Scope, name string) (obj types.Object) {
	if obj = scope.Lookup(name); obj != nil {
		if t, ok := obj.Type().(*SubstType); ok {
			obj = t.Real
		}
	}
	return
}

// ----------------------------------------------------------------------------
