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

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

// ConstStart starts a constant expression.
func (p *Package) ConstStart() *CodeBuilder {
	return &p.cb
}

func (p *CodeBuilder) EndConst() types.TypeAndValue {
	elem := p.stk.Pop()
	return evalConstExpr(p.pkg, elem.Val)
}

func evalConstExpr(pkg *Package, expr ast.Expr) types.TypeAndValue {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	if err := types.CheckExpr(pkg.Fset, pkg.Types, token.NoPos, expr, info); err != nil {
		log.Panicln("TODO: eval constant -", err)
	}
	return info.Types[expr]
}

func evalIntExpr(pkg *Package, expr ast.Expr) int {
	tv := evalConstExpr(pkg, expr)
	if v, ok := constant.Int64Val(tv.Value); ok {
		return int(v)
	}
	panic("TODO: require integer constant")
}

// ----------------------------------------------------------------------------

// ValueDecl type
type ValueDecl struct {
	names []string
	typ   types.Type
	old   codeBlock
	vals  *[]ast.Expr
	tok   token.Token
}

func (p *ValueDecl) InitStart(pkg *Package) *CodeBuilder {
	pkg.cb.varDecl = p
	p.old = pkg.cb.startInitExpr(p)
	return &pkg.cb
}

func (p *ValueDecl) End(cb *CodeBuilder) {
	panic("don't call End(), please use EndInit() instead")
}

func (p *ValueDecl) EndInit(cb *CodeBuilder, arity int) {
	n := len(p.names)
	rets := cb.stk.GetArgs(arity)
	if arity == 1 && n != 1 {
		t, ok := rets[0].Type.(*types.Tuple)
		if !ok || n != t.Len() {
			panic("TODO: unmatched var/const define")
		}
		*p.vals = []ast.Expr{rets[0].Val}
		rets = make([]internal.Elem, n)
		for i := 0; i < n; i++ {
			rets[i] = internal.Elem{Type: t.At(i).Type()}
		}
	} else if n != arity {
		panic("TODO: unmatched var/const define")
	} else {
		values := make([]ast.Expr, arity)
		for i, ret := range rets {
			values[i] = ret.Val
		}
		*p.vals = values
	}
	pkg, scope := cb.pkg, cb.current.scope
	typ := p.typ
	if typ != nil {
		for _, ret := range rets {
			if err := checkMatchType(pkg, ret.Type, typ); err != nil {
				panic(err)
			}
		}
	}
	for i, name := range p.names {
		if name == "_" { // skip underscore
			continue
		}
		if p.tok == token.CONST {
			tv := evalConstExpr(pkg, rets[i].Val)
			if scope.Insert(types.NewConst(token.NoPos, pkg.Types, name, tv.Type, tv.Value)) != nil {
				panic("TODO: constant already defined")
			}
		} else if typ == nil {
			if old := scope.Insert(types.NewVar(token.NoPos, pkg.Types, name, rets[i].Type)); old != nil {
				if p.tok != token.DEFINE {
					log.Panicln("TODO: variable already defined -", name)
				}
				if err := checkMatchType(pkg, rets[i].Type, old.Type()); err != nil {
					panic(err)
				}
			}
		}
	}
	cb.stk.PopN(arity)
	cb.endInitExpr(p.old)
	return
}

func (p *Package) newValueDecl(tok token.Token, typ types.Type, names ...string) *ValueDecl {
	n := len(names)
	if tok == token.DEFINE { // a, b := expr
		nameIdents := make([]ast.Expr, n)
		for i, name := range names {
			nameIdents[i] = ident(name)
		}
		stmt := &ast.AssignStmt{Tok: token.DEFINE, Lhs: nameIdents}
		p.cb.emitStmt(stmt)
		return &ValueDecl{names: names, tok: tok, vals: &stmt.Rhs}
	}
	// var a, b = expr
	// const a, b = expr
	var typExpr ast.Expr
	if typ != nil {
		typExpr = toType(p, typ)
	}
	scope := p.cb.current.scope
	nameIdents := make([]*ast.Ident, n)
	for i, name := range names {
		nameIdents[i] = ident(name)
		if name == "_" { // skip underscore
			continue
		}
		if typ != nil && tok == token.VAR {
			scope.Insert(types.NewVar(token.NoPos, p.Types, name, typ))
		}
	}
	spec := &ast.ValueSpec{Names: nameIdents, Type: typExpr}
	decl := &ast.GenDecl{Tok: tok, Specs: []ast.Spec{spec}}
	if scope == p.Types.Scope() {
		p.decls = append(p.decls, decl)
	} else {
		p.cb.emitStmt(&ast.DeclStmt{Decl: decl})
	}
	return &ValueDecl{typ: typ, names: names, tok: tok, vals: &spec.Values}
}

func (p *Package) NewConst(typ types.Type, names ...string) *ValueDecl {
	return p.newValueDecl(token.CONST, typ, names...)
}

func (p *Package) NewConstStart(typ types.Type, names ...string) *CodeBuilder {
	return p.newValueDecl(token.CONST, typ, names...).InitStart(p)
}

func (p *Package) NewVar(typ types.Type, names ...string) *ValueDecl {
	return p.newValueDecl(token.VAR, typ, names...)
}

func (p *Package) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	return p.newValueDecl(token.VAR, typ, names...).InitStart(p)
}

// ----------------------------------------------------------------------------

var (
	TyByte = types.Universe.Lookup("byte").Type().(*types.Basic)
	TyRune = types.Universe.Lookup("rune").Type().(*types.Basic)
)

var (
	TyEmptyInterface = types.NewInterfaceType(nil, nil)
)

// refType: &T
type refType struct {
	typ types.Type
}

func (p *refType) Underlying() types.Type {
	panic("ref type")
}

func (p *refType) String() string {
	panic("ref type")
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
	panic("unbound type")
}

func realType(typ types.Type) types.Type {
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound != nil {
			return t.tBound
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
	panic("unbound map elem type")
}

// overloadFuncType: overload function type
type overloadFuncType struct {
	funcs []types.Object
}

func (p *overloadFuncType) Underlying() types.Type {
	panic("overload function type")
}

func (p *overloadFuncType) String() string {
	panic("overload function type")
}

// delayedLoadType: delay load object type
type delayedLoadType struct {
	obj   func() types.Object
	cache types.Object
}

func (p *delayedLoadType) Underlying() types.Type {
	panic("overload function type")
}

func (p *delayedLoadType) String() string {
	panic("overload function type")
}

func (p *delayedLoadType) Obj() types.Object {
	if p.cache == nil {
		p.cache = p.obj()
	}
	return p.cache
}

func NewDelayedLoad(pos token.Pos, pkg *types.Package, name string, obj func() types.Object) *types.TypeName {
	return types.NewTypeName(pos, pkg, name, &delayedLoadType{obj: obj})
}

// ----------------------------------------------------------------------------
