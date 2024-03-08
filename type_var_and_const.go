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
	"syscall"

	"github.com/goplus/gogen/internal"
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

// MethodToFunc:
//
//	(T).method
//	(*T).method
func (pkg *Package) MethodToFunc(typ types.Type, name string, src ...ast.Node) (ret *Element, err error) {
	_, err = pkg.cb.Typ(typ, src...).Member(name, MemberFlagVal, src...)
	ret = pkg.cb.stk.Pop()
	return
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
	typ  *types.Named
	spec *ast.TypeSpec
}

// SetComments sets associated documentation.
func (p *TypeDecl) SetComments(pkg *Package, doc *ast.CommentGroup) *TypeDecl {
	p.spec.Doc = doc
	pkg.setDoc(p.typ.Obj(), doc)
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
	spec := p.spec
	if spec.Name != nil {
		if spec.Type != nil {
			return TyStateInited
		}
		return TyStateUninited
	}
	return TyStateDeleted
}

// Delete deletes this type.
// NOTE: It panics if you call InitType after Delete.
func (p *TypeDecl) Delete() {
	p.spec.Name = nil
}

// Inited checkes if InitType is called or not.
// Will panic if this type is deleted (please use State to check).
func (p *TypeDecl) Inited() bool {
	return p.spec.Type != nil
}

// InitType initializes a uncompleted type.
func (p *TypeDecl) InitType(pkg *Package, typ types.Type, tparams ...*TypeParam) *types.Named {
	if debugInstr {
		log.Println("InitType", p.typ.Obj().Name(), typ)
	}
	spec := p.spec
	if spec.Type != nil {
		log.Panicln("TODO: type already defined -", typ)
	}
	if named, ok := typ.(*types.Named); ok {
		p.typ.SetUnderlying(pkg.cb.getUnderlying(named))
	} else {
		p.typ.SetUnderlying(typ)
	}
	if tparams != nil {
		setTypeParams(pkg, p.typ, spec, tparams)
	}
	spec.Type = toType(pkg, typ)
	return p.typ
}

// ----------------------------------------------------------------------------

// TypeDefs represents a type declaration block.
type TypeDefs struct {
	decl  *ast.GenDecl
	scope *types.Scope
	pkg   *Package
}

// Pkg returns the package instance.
func (p *TypeDefs) Pkg() *Package {
	return p.pkg
}

// SetComments sets associated documentation.
func (p *TypeDefs) SetComments(doc *ast.CommentGroup) *TypeDefs {
	p.decl.Doc = doc
	return p
}

// NewType creates a new type (which need to call InitType later).
func (p *TypeDefs) NewType(name string, src ...ast.Node) *TypeDecl {
	if debugInstr {
		log.Println("NewType", name)
	}
	return p.pkg.doNewType(p, getPos(src), name, nil, 0)
}

// AliasType gives a specified type with a new name.
func (p *TypeDefs) AliasType(name string, typ types.Type, src ...ast.Node) *TypeDecl {
	if debugInstr {
		log.Println("AliasType", name, typ)
	}
	return p.pkg.doNewType(p, getPos(src), name, typ, 1)
}

// Complete checks type declarations & marks completed.
func (p *TypeDefs) Complete() {
	decl := p.decl
	specs := decl.Specs
	if len(specs) == 1 && decl.Doc == nil {
		if spec := specs[0].(*ast.TypeSpec); spec.Doc != nil {
			decl.Doc, spec.Doc = spec.Doc, nil
		}
	}
	for i, spec := range specs {
		v := spec.(*ast.TypeSpec)
		if v.Name == nil || v.Type == nil {
			for j := i + 1; j < len(specs); j++ {
				v = specs[j].(*ast.TypeSpec)
				if v.Name != nil && v.Type != nil {
					specs[i] = v
					i++
				}
			}
			decl.Specs = specs[:i]
			return
		}
	}
}

// ----------------------------------------------------------------------------

// AliasType gives a specified type with a new name.
//
// Deprecated: use NewTypeDefs instead.
func (p *Package) AliasType(name string, typ types.Type, src ...ast.Node) *types.Named {
	decl := p.NewTypeDefs().AliasType(name, typ, src...)
	return decl.typ
}

// NewType creates a new type (which need to call InitType later).
//
// Deprecated: use NewTypeDefs instead.
func (p *Package) NewType(name string, src ...ast.Node) *TypeDecl {
	return p.NewTypeDefs().NewType(name, src...)
}

// NewTypeDefs starts a type declaration block.
func (p *Package) NewTypeDefs() *TypeDefs {
	decl := &ast.GenDecl{Tok: token.TYPE}
	p.file.decls = append(p.file.decls, decl)
	return &TypeDefs{decl: decl, scope: p.Types.Scope(), pkg: p}
}

// NewTypeDefs starts a type declaration block.
func (p *CodeBuilder) NewTypeDefs() *TypeDefs {
	ret, defineHere := p.NewTypeDecls()
	defineHere()
	return ret
}

// NewTypeDecls starts a type declaration block but delay to define it.
func (p *CodeBuilder) NewTypeDecls() (ret *TypeDefs, defineHere func()) {
	pkg, scope := p.pkg, p.current.scope
	decl := &ast.GenDecl{Tok: token.TYPE}
	return &TypeDefs{decl: decl, scope: scope, pkg: pkg}, func() {
		if scope == pkg.Types.Scope() {
			pkg.file.decls = append(pkg.file.decls, decl)
		} else {
			p.emitStmt(&ast.DeclStmt{Decl: decl})
		}
	}
}

func (p *Package) doNewType(tdecl *TypeDefs, pos token.Pos, name string, typ types.Type, alias token.Pos) *TypeDecl {
	scope := tdecl.scope
	typName := types.NewTypeName(pos, p.Types, name, typ)
	if old := scope.Insert(typName); old != nil {
		oldPos := p.cb.fset.Position(old.Pos())
		p.cb.panicCodeErrorf(
			pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldPos)
	}
	decl := tdecl.decl
	spec := &ast.TypeSpec{Name: ident(name), Assign: alias}
	decl.Specs = append(decl.Specs, spec)
	if alias != 0 { // alias don't need to call InitType
		spec.Type = toType(p, typ)
		typ = typ.Underlying() // typ.Underlying() may delay load and can be nil, it's reasonable
	}
	named := types.NewNamed(typName, typ, nil)
	p.useName(name)
	return &TypeDecl{typ: named, spec: spec}
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
			caller := getCaller(rets[0])
			cb.panicCodeErrorf(
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
				cb.panicCodeError(p.pos, "missing value in const declaration")
			}
			cb.panicCodeError(p.pos, "extra expression in const declaration")
		}
		cb.panicCodeErrorf(p.pos, "assignment mismatch: %d variables but %d values", n, arity)
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
		pkg.useName(name)
		if p.tok == token.CONST {
			tv := rets[i]
			if tv.CVal == nil {
				src, _ := cb.loadExpr(tv.Src)
				cb.panicCodeErrorf(
					p.pos, "const initializer %s is not a constant", src)
			}
			tvType := typ
			if tvType == nil {
				tvType = tv.Type
			}
			if old := p.scope.Insert(types.NewConst(p.pos, pkg.Types, name, tvType, tv.CVal)); old != nil {
				oldpos := cb.fset.Position(old.Pos())
				cb.panicCodeErrorf(
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
					oldpos := cb.fset.Position(old.Pos())
					cb.panicCodeErrorf(
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
	spec ValueAt, scope *types.Scope, pos token.Pos, tok token.Token, typ types.Type, names ...string) *ValueDecl {
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
			p.cb.handleCodeError(pos, "no new variables on left side of :=")
		}
		stmt := &ast.AssignStmt{Tok: token.DEFINE, Lhs: nameIdents}
		at := p.cb.startStmtAt(stmt)
		return &ValueDecl{names: names, tok: tok, pos: pos, scope: scope, vals: &stmt.Rhs, at: at}
	} else if tok == token.CONST && len(names) == 1 && isGopoConst(names[0]) { // Gopo_XXX
		p.isGopPkg = true
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
					oldpos := p.cb.fset.Position(old.Pos())
					p.cb.panicCodeErrorf(
						pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
				}
			}
		}
	}
	spec.Names = nameIdents
	if typ != nil {
		if ut, ok := typ.(*unboundType); ok && ut.tBound == nil {
			ut.ptypes = append(ut.ptypes, &spec.Type)
		} else {
			spec.Type = toType(p, typ)
		}
	}
	return &ValueDecl{
		typ: typ, names: names, tok: tok, pos: pos, scope: scope, vals: &spec.Values, at: spec.at}
}

func (p *Package) newValueDefs(scope *types.Scope, tok token.Token) *valueDefs {
	at := -1
	decl := &ast.GenDecl{Tok: tok}
	if scope == p.Types.Scope() {
		p.file.decls = append(p.file.decls, decl)
	} else {
		at = p.cb.startStmtAt(&ast.DeclStmt{Decl: decl})
	}
	return &valueDefs{pkg: p, scope: scope, decl: decl, at: at}
}

func (p *CodeBuilder) valueDefs(tok token.Token) *valueDefs {
	at := -1
	decl := &ast.GenDecl{Tok: tok}
	pkg, scope := p.pkg, p.current.scope
	if scope == pkg.Types.Scope() {
		pkg.file.decls = append(pkg.file.decls, decl)
	} else {
		at = p.startStmtAt(&ast.DeclStmt{Decl: decl})
	}
	return &valueDefs{pkg: pkg, scope: scope, decl: decl, at: at}
}

// NewConstStart creates constants with names.
//
// Deprecated: Use NewConstDefs instead.
func (p *Package) NewConstStart(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewConst", names)
	}
	at := p.newValueDefs(scope, token.CONST).NewPos()
	return p.newValueDecl(at, scope, pos, token.CONST, typ, names...).InitStart(p)
}

// NewConstDefs starts a constant declaration block.
func (p *Package) NewConstDefs(scope *types.Scope) *ConstDefs {
	if debugInstr {
		log.Println("NewConstDefs")
	}
	return &ConstDefs{valueDefs: *p.newValueDefs(scope, token.CONST)}
}

// NewVar starts a var declaration block and creates uninitialized variables with
// specified `typ` (can be nil) and `names`.
//
// Deprecated: This is a shortcut for creating variables. `NewVarDefs` is more powerful and
// more recommended.
func (p *Package) NewVar(pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	scope := p.Types.Scope()
	at := p.newValueDefs(scope, token.VAR).NewPos()
	return p.newValueDecl(at, scope, pos, token.VAR, typ, names...)
}

// NewVarEx starts a var declaration block and creates uninitialized variables with
// specified `typ` (can be nil) and `names`.
//
// Deprecated: This is a shortcut for creating variables. `NewVarDefs` is more powerful and
// more recommended.
func (p *Package) NewVarEx(scope *types.Scope, pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	at := p.newValueDefs(scope, token.VAR).NewPos()
	return p.newValueDecl(at, scope, pos, token.VAR, typ, names...)
}

// NewVarStart creates variables with specified `typ` (can be nil) and `names` and starts
// to initialize them. You should call `CodeBuilder.EndInit` to end initialization.
//
// Deprecated: This is a shortcut for creating variables. `NewVarDefs` is more powerful and more
// recommended.
func (p *Package) NewVarStart(pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	if debugInstr {
		log.Println("NewVar", names)
	}
	scope := p.Types.Scope()
	at := p.newValueDefs(scope, token.VAR).NewPos()
	return p.newValueDecl(at, scope, pos, token.VAR, typ, names...).InitStart(p)
}

// NewVarDefs starts a var declaration block.
func (p *Package) NewVarDefs(scope *types.Scope) *VarDefs {
	if debugInstr {
		log.Println("NewVarDefs")
	}
	return &VarDefs{*p.newValueDefs(scope, token.VAR)}
}

// ----------------------------------------------------------------------------

type ValueAt struct {
	*ast.ValueSpec
	at int
}

type valueDefs struct {
	decl  *ast.GenDecl
	scope *types.Scope
	pkg   *Package
	at    int
}

func (p *valueDefs) NewPos() ValueAt {
	decl := p.decl
	spec := &ast.ValueSpec{}
	decl.Specs = append(decl.Specs, spec)
	return ValueAt{spec, p.at}
}

// VarDefs represents a var declaration block.
type VarDefs struct {
	valueDefs
}

// SetComments sets associated documentation.
func (p *VarDefs) SetComments(doc *ast.CommentGroup) *VarDefs {
	p.decl.Doc = doc
	return p
}

// New creates uninitialized variables with specified `typ` (can be nil) and `names`.
func (p *VarDefs) New(pos token.Pos, typ types.Type, names ...string) *VarDecl {
	return p.NewAt(p.NewPos(), pos, typ, names...)
}

// NewAt creates uninitialized variables with specified `typ` (can be nil) and `names`.
func (p *VarDefs) NewAt(at ValueAt, pos token.Pos, typ types.Type, names ...string) *VarDecl {
	if debugInstr {
		log.Println("NewVar", names)
	}
	return p.pkg.newValueDecl(at, p.scope, pos, token.VAR, typ, names...)
}

// NewAndInit creates variables with specified `typ` (can be nil) and `names`, and initializes them by `fn`.
func (p *VarDefs) NewAndInit(fn F, pos token.Pos, typ types.Type, names ...string) *VarDefs {
	if debugInstr {
		log.Println("NewAndInit", names)
	}
	decl := p.pkg.newValueDecl(p.NewPos(), p.scope, pos, token.VAR, typ, names...)
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

// F represents an initialization callback for constants/variables.
type F = func(cb *CodeBuilder) int

// ConstDefs represents a const declaration block.
type ConstDefs struct {
	valueDefs
	typ types.Type
	F   F
}

func constInitFn(cb *CodeBuilder, iotav int, fn F) int {
	oldv := cb.iotav
	cb.iotav = iotav
	defer func() {
		cb.iotav = oldv
	}()
	return fn(cb)
}

// SetComments sets associated documentation.
func (p *ConstDefs) SetComments(doc *ast.CommentGroup) *ConstDefs {
	p.decl.Doc = doc
	return p
}

// New creates constants with specified `typ` (can be nil) and `names`.
// The values of the constants are given by the callback `fn`.
func (p *ConstDefs) New(fn F, iotav int, pos token.Pos, typ types.Type, names ...string) *ConstDefs {
	return p.NewAt(p.NewPos(), fn, iotav, pos, typ, names...)
}

// NewAt creates constants with specified `typ` (can be nil) and `names`.
// The values of the constants are given by the callback `fn`.
func (p *ConstDefs) NewAt(at ValueAt, fn F, iotav int, pos token.Pos, typ types.Type, names ...string) *ConstDefs {
	if debugInstr {
		log.Println("NewConst", names, iotav)
	}
	pkg := p.pkg
	cb := pkg.newValueDecl(at, p.scope, pos, token.CONST, typ, names...).InitStart(pkg)
	n := constInitFn(cb, iotav, fn)
	cb.EndInit(n)
	p.F, p.typ = fn, typ
	return p
}

// Next creates constants with specified `names`.
// The values of the constants are given by the callback `fn` which is
// specified by the last call to `New`.
func (p *ConstDefs) Next(iotav int, pos token.Pos, names ...string) *ConstDefs {
	return p.NextAt(p.NewPos(), p.F, iotav, pos, names...)
}

// NextAt creates constants with specified `names`.
// The values of the constants are given by the callback `fn`.
func (p *ConstDefs) NextAt(at ValueAt, fn F, iotav int, pos token.Pos, names ...string) *ConstDefs {
	pkg := p.pkg
	cb := pkg.CB()
	n := constInitFn(cb, iotav, fn)
	if len(names) != n {
		if len(names) < n {
			cb.panicCodeError(pos, "extra expression in const declaration")
		}
		cb.panicCodeError(pos, "missing value in const declaration")
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
				oldpos := cb.fset.Position(old.Pos())
				cb.panicCodeErrorf(
					pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
			}
		}
		idents[i] = ident(name)
	}
	at.Names = idents
	return p
}

// ----------------------------------------------------------------------------
