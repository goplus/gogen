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

// TypeDecl type
type TypeDecl struct {
	typ  *types.Named
	decl *ast.GenDecl
	spec *ast.TypeSpec
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

// InitType initializes a uncompleted type.
func (p *TypeDecl) InitType(pkg *Package, typ types.Type) *types.Named {
	if debugInstr {
		log.Println("InitType", p.typ.Obj().Name(), typ)
	}
	if named, ok := typ.(*types.Named); ok {
		p.typ.SetUnderlying(pkg.cb.getUnderlying(named))
	} else {
		p.typ.SetUnderlying(typ)
	}
	p.spec.Type = toType(pkg, typ)
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

func (p *Package) doNewType(
	scope *types.Scope, pos token.Pos, name string, typ types.Type, alias token.Pos) *TypeDecl {
	typName := types.NewTypeName(pos, p.Types, name, typ)
	if scope.Insert(typName) != nil {
		log.Panicln("TODO: type already defined -", name)
	}
	spec := &ast.TypeSpec{Name: ident(name), Assign: alias}
	decl := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
	if scope == p.Types.Scope() {
		idx := p.testingFile
		p.files[idx].decls = append(p.files[idx].decls, decl)
	} else {
		p.cb.emitStmt(&ast.DeclStmt{Decl: decl})
	}
	if alias != 0 { // alias don't need to call InitType
		spec.Type = toType(p, typ)
		typ = typ.Underlying() // typ.Underlying() may delay load and can be nil, it's reasonable
	}
	named := types.NewNamed(typName, typ, nil)
	return &TypeDecl{typ: named, decl: decl, spec: spec}
}

// ----------------------------------------------------------------------------

// VarDecl type
type VarDecl struct {
	names []string
	typ   types.Type
	old   codeBlock
	oldv  *VarDecl
	scope *types.Scope
	vals  *[]ast.Expr
	tok   token.Token
	pos   token.Pos
	at    int // commitStmt(at)
}

func (p *VarDecl) InitStart(pkg *Package) *CodeBuilder {
	p.oldv, pkg.cb.varDecl = pkg.cb.varDecl, p
	p.old = pkg.cb.startInitExpr(p)
	return &pkg.cb
}

func (p *VarDecl) End(cb *CodeBuilder) {
	fatal("don't call End(), please use EndInit() instead")
}

func (p *VarDecl) resetInit(cb *CodeBuilder) *VarDecl {
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

func (p *VarDecl) endInit(cb *CodeBuilder, arity int) *VarDecl {
	var t *types.Tuple
	var expr *ast.Expr
	var values []ast.Expr
	n := len(p.names)
	rets := cb.stk.GetArgs(arity)
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
			if values != nil {
				expr = &values[i]
			}
			retType := DefaultConv(pkg, rets[i].Type, expr)
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
	cb.stk.PopN(arity)
	cb.endInitExpr(p.old)
	if p.at >= 0 {
		cb.commitStmt(p.at) // to support inline call, we must emitStmt at EndInit stage
	}
	return p.oldv
}

// ValueDecl type (only for internal use).
//
// Deprecated: Use VarDecl instead.
type ValueDecl = VarDecl

func (p *Package) newValueDecl(
	cdecl *ConstDecl, scope *types.Scope, pos token.Pos, tok token.Token, typ types.Type, names ...string) *ValueDecl {
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
			scope.Insert(types.NewVar(pos, p.Types, name, typ))
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
	var inAST bool
	var decl *ast.GenDecl
	if cdecl != nil {
		decl, inAST, cdecl.inAST = cdecl.decl, cdecl.inAST, true
		decl.Specs = append(decl.Specs, spec)
	} else {
		decl = &ast.GenDecl{Tok: tok, Specs: []ast.Spec{spec}}
	}
	at := -1
	if !inAST {
		if scope == p.Types.Scope() {
			idx := p.testingFile
			p.files[idx].decls = append(p.files[idx].decls, decl)
		} else {
			at = p.cb.startStmtAt(&ast.DeclStmt{Decl: decl})
		}
	}
	return &ValueDecl{
		typ: typ, names: names, tok: tok, pos: pos, scope: scope, vals: &spec.Values, at: at}
}

// NewConstStart creates constants with names.
//
// Deprecated: Use NewConstDecl instead.
func (p *Package) NewConstStart(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewConst", names)
	}
	return p.newValueDecl(nil, scope, pos, token.CONST, typ, names...).InitStart(p)
}

func (p *Package) NewConstDecl(scope *types.Scope) *ConstDecl {
	if debugInstr {
		log.Println("NewConstDecl")
	}
	decl := &ast.GenDecl{Tok: token.CONST}
	return &ConstDecl{pkg: p, scope: scope, decl: decl}
}

func (p *Package) NewVar(pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, p.Types.Scope(), pos, token.VAR, typ, names...)
}

func (p *Package) NewVarEx(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, scope, pos, token.VAR, typ, names...)
}

func (p *Package) NewVarStart(pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.newValueDecl(nil, p.Types.Scope(), pos, token.VAR, typ, names...).InitStart(p)
}

// ----------------------------------------------------------------------------

type ConstDecl struct {
	decl  *ast.GenDecl
	scope *types.Scope
	pkg   *Package
	fn    func(cb *CodeBuilder) int
	typ   types.Type
	inAST bool
}

func constInitFn(cb *CodeBuilder, iotav int, fn func(cb *CodeBuilder) int) int {
	oldv := cb.iotav
	cb.iotav = iotav
	defer func() {
		cb.iotav = oldv
	}()
	return fn(cb)
}

func (p *ConstDecl) New(
	fn func(cb *CodeBuilder) int, iotav int, pos token.Pos, typ types.Type, names ...string) *ConstDecl {
	if debugInstr {
		log.Println("NewConst", names, iotav)
	}
	pkg := p.pkg
	cb := pkg.newValueDecl(p, p.scope, pos, token.CONST, typ, names...).InitStart(pkg)
	n := constInitFn(cb, iotav, fn)
	cb.EndInit(n)
	p.fn, p.typ = fn, typ
	return p
}

func (p *ConstDecl) Next(iotav int, pos token.Pos, names ...string) *ConstDecl {
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

// overloadFuncType: overload function type
type overloadFuncType struct {
	funcs []types.Object
}

func (p *overloadFuncType) Underlying() types.Type {
	fatal("overload function type")
	return nil
}

func (p *overloadFuncType) String() string {
	return fmt.Sprintf("overloadFuncType{funcs: %v}", p.funcs)
}

// ----------------------------------------------------------------------------

type btiMethodType struct {
	types.Type
	eargs []interface{}
}

type templateRecvMethodType struct {
	fn types.Object
}

func (p *templateRecvMethodType) Underlying() types.Type {
	fatal("template recv method type")
	return nil
}

func (p *templateRecvMethodType) String() string {
	return fmt.Sprintf("templateRecvMethodType{fn: %v}", p.fn)
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
	switch t.(type) {
	case *overloadFuncType:
		return false
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
