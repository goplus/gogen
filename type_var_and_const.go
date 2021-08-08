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

func (p *CodeBuilder) EndConst() types.TypeAndValue {
	elem := p.stk.Pop()
	if elem.CVal == nil {
		panic("TODO: expression is not a constant")
	}
	return types.TypeAndValue{Type: elem.Type, Value: elem.CVal}
}

// ----------------------------------------------------------------------------

// TypeDecl type
type TypeDecl struct {
	typ     *types.Named
	typExpr *ast.Expr
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
	p.typ.SetUnderlying(typ)
	*p.typExpr = toType(pkg, typ)
	return p.typ
}

// AliasType gives a specified type with a new name
func (p *Package) AliasType(name string, typ types.Type) *types.Named {
	if debugInstr {
		log.Println("AliasType", name, typ)
	}
	decl := p.newType(name, typ, 1)
	return decl.typ
}

// NewType creates a new type (which need to call InitType later).
func (p *Package) NewType(name string) *TypeDecl {
	if debugInstr {
		log.Println("NewType", name)
	}
	return p.newType(name, nil, 0)
}

func (p *Package) newType(name string, typ types.Type, alias token.Pos) *TypeDecl {
	typName := types.NewTypeName(token.NoPos, p.Types, name, typ)
	scope := p.cb.current.scope
	if scope.Insert(typName) != nil {
		log.Panicln("TODO: type already defined -", name)
	}
	spec := &ast.TypeSpec{Name: ident(name), Assign: alias}
	decl := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
	if scope == p.Types.Scope() {
		idx := p.inTestingFile
		p.files[idx].decls = append(p.files[idx].decls, decl)
	} else {
		p.cb.emitStmt(&ast.DeclStmt{Decl: decl})
	}
	if alias != 0 { // alias don't need to call InitType
		spec.Type = toType(p, typ)
		typ = typ.Underlying() // typ.Underlying() may delay load and can be nil, it's reasonable
	}
	named := types.NewNamed(typName, typ, nil)
	return &TypeDecl{typ: named, typExpr: &spec.Type}
}

func (p *Package) getUnderlying(typ *types.Named) types.Type {
	return typ.Underlying()
}

// ----------------------------------------------------------------------------

// ValueDecl type
type ValueDecl struct {
	names []string
	typ   types.Type
	old   codeBlock
	oldv  *ValueDecl
	vals  *[]ast.Expr
	tok   token.Token
	pos   token.Pos
	at    int
}

func (p *ValueDecl) InitStart(pkg *Package) *CodeBuilder {
	p.oldv, pkg.cb.varDecl = pkg.cb.varDecl, p
	p.old = pkg.cb.startInitExpr(p)
	return &pkg.cb
}

func (p *ValueDecl) End(cb *CodeBuilder) {
	panic("don't call End(), please use EndInit() instead")
}

func (p *ValueDecl) resetInit(cb *CodeBuilder) *ValueDecl {
	cb.endInitExpr(p.old)
	if p.at >= 0 {
		cb.commitStmt(p.at) // to support inline call, we must emitStmt at ResetInit stage
	}
	return p.oldv
}

func (p *ValueDecl) endInit(cb *CodeBuilder, arity int) *ValueDecl {
	var expr *ast.Expr
	var values []ast.Expr
	n := len(p.names)
	rets := cb.stk.GetArgs(arity)
	if arity == 1 && n != 1 {
		t, ok := rets[0].Type.(*types.Tuple)
		if !ok || n != t.Len() {
			panic("TODO: unmatched var/const define")
		}
		*p.vals = []ast.Expr{rets[0].Val}
		rets = make([]*internal.Elem, n)
		for i := 0; i < n; i++ {
			rets[i] = &internal.Elem{Type: t.At(i).Type()}
		}
	} else if n != arity {
		panic("TODO: unmatched var/const define")
	} else {
		values = make([]ast.Expr, arity)
		for i, ret := range rets {
			values[i] = ret.Val
		}
		*p.vals = values
	}
	pkg, scope := cb.pkg, cb.current.scope
	typ := p.typ
	if typ != nil {
		for _, ret := range rets {
			if err := matchType(pkg, ret, typ, "assignment"); err != nil {
				panic(err)
			}
		}
	}
	for i, name := range p.names {
		if name == "_" { // skip underscore
			continue
		}
		if p.tok == token.CONST {
			tv := rets[i]
			if old := scope.Insert(types.NewConst(p.pos, pkg.Types, name, tv.Type, tv.CVal)); old != nil {
				oldpos := cb.position(old.Pos())
				cb.panicCodePosErrorf(
					p.pos, "%s redeclared in this block\n\tprevious declaration at %v", name, oldpos)
			}
		} else if typ == nil {
			if values != nil {
				expr = &values[i]
			}
			retType := DefaultConv(pkg, rets[i].Type, expr)
			if old := scope.Insert(types.NewVar(p.pos, pkg.Types, name, retType)); old != nil {
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

func (p *Package) newValueDecl(pos token.Pos, tok token.Token, typ types.Type, names ...string) *ValueDecl {
	scope := p.cb.current.scope
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
		return &ValueDecl{names: names, tok: tok, pos: pos, vals: &stmt.Rhs, at: at}
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
	at := -1
	decl := &ast.GenDecl{Tok: tok, Specs: []ast.Spec{spec}}
	if scope == p.Types.Scope() {
		idx := p.inTestingFile
		p.files[idx].decls = append(p.files[idx].decls, decl)
	} else {
		at = p.cb.startStmtAt(&ast.DeclStmt{Decl: decl})
	}
	return &ValueDecl{typ: typ, names: names, tok: tok, pos: pos, vals: &spec.Values, at: at}
}

func (p *Package) NewConstStart(pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	return p.newValueDecl(pos, token.CONST, typ, names...).InitStart(p)
}

func (p *Package) NewVar(pos token.Pos, typ types.Type, names ...string) *ValueDecl {
	return p.newValueDecl(pos, token.VAR, typ, names...)
}

func (p *Package) NewVarStart(pos token.Pos, typ types.Type, names ...string) *CodeBuilder {
	return p.newValueDecl(pos, token.VAR, typ, names...).InitStart(p)
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

func (p *refType) Underlying() types.Type {
	panic("ref type")
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
		panic("TODO: type is already bounded")
	}
	p.tBound = arg
	for _, pt := range p.ptypes {
		*pt = toType(pkg, arg)
	}
	p.ptypes = nil
}

func (p *unboundType) Underlying() types.Type {
	panic("unbound type")
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
	panic("unbound map elem type")
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
	panic("overload function type")
}

func (p *overloadFuncType) String() string {
	return fmt.Sprintf("overloadFuncType{funcs: %v}", p.funcs)
}

type instructionType struct {
	instr Instruction
}

func (p *instructionType) Underlying() types.Type {
	panic("instruction type")
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
	panic("type of type")
}

func (p *TypeType) String() string {
	return fmt.Sprintf("TypeType{typ: %v}", p.typ)
}

// ----------------------------------------------------------------------------
