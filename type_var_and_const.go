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
	"go/token"
	"go/types"
	"log"
)

// ----------------------------------------------------------------------------

// A Variable represents a declared variable (including function parameters and results, and struct fields).
type AutoVar struct {
	name  string
	typ   types.Type
	ptype *ast.Expr
}

func newAutoVar(name string, ptype *ast.Expr) *AutoVar {
	v := &AutoVar{name: name, ptype: ptype}
	v.typ = &unboundType{v: v}
	return v
}

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

// ----------------------------------------------------------------------------

// ValueDecl type
type ValueDecl struct {
	names []string
	typ   types.Type
	old   codeBlock
	vals  *[]ast.Expr
	tok   token.Token
}

func (p *ValueDecl) BodyStart(pkg *Package) *CodeBuilder {
	p.old = pkg.cb.startInitExpr(p)
	return &pkg.cb
}

func (p *ValueDecl) End(cb *CodeBuilder) {
	pkg, scope := cb.pkg, cb.current.scope
	typ := p.typ
	n := len(p.names)
	if n != cb.stk.Len()-cb.current.base {
		panic("TODO: unmatched count of initial expressions")
	}
	rets := cb.stk.GetArgs(n)
	if typ != nil {
		for _, ret := range rets {
			if err := checkMatchType(pkg, ret.Type, typ); err != nil {
				panic(err)
			}
		}
	}
	values := make([]ast.Expr, n)
	for i, name := range p.names {
		val := rets[i].Val
		values[i] = val
		if p.tok == token.CONST {
			tv := evalConstExpr(pkg, val)
			scope.Insert(types.NewConst(token.NoPos, pkg.Types, name, tv.Type, tv.Value))
		} else if typ == nil {
			scope.Insert(types.NewVar(token.NoPos, pkg.Types, name, rets[i].Type))
		}
	}
	cb.stk.PopN(n)
	cb.endInitExpr(p.old)
	*p.vals = values
	return
}

func (p *Package) newValueDecl(tok token.Token, typ types.Type, names ...string) *ValueDecl {
	var typExpr ast.Expr
	if typ != nil {
		typExpr = toType(p, typ)
	}
	scope := p.cb.current.scope
	n := len(names)
	nameIdents := make([]*ast.Ident, n)
	for i, name := range names {
		nameIdents[i] = ident(name)
		if typ != nil && tok == token.VAR {
			scope.Insert(types.NewVar(token.NoPos, p.Types, name, typ))
		}
	}
	spec := &ast.ValueSpec{Names: nameIdents, Type: typExpr}
	decl := &ast.GenDecl{Tok: tok, Specs: []ast.Spec{spec}}
	if scope == p.Types.Scope() {
		p.decls = append(p.decls, decl)
	} else {
		p.cb.current.stmts = append(p.cb.current.stmts, &ast.DeclStmt{Decl: decl})
	}
	return &ValueDecl{typ: typ, names: names, tok: tok, vals: &spec.Values}
}

func (p *Package) NewConst(typ types.Type, names ...string) *ValueDecl {
	return p.newValueDecl(token.CONST, typ, names...)
}

func (p *Package) NewVar(typ types.Type, names ...string) *ValueDecl {
	return p.newValueDecl(token.VAR, typ, names...)
}

// ----------------------------------------------------------------------------

var (
	TyByte = types.Universe.Lookup("byte").Type().(*types.Basic)
	TyRune = types.Universe.Lookup("rune").Type().(*types.Basic)
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
	bound types.Type
	v     *AutoVar
}

func isUnbound(t types.Type) bool {
	ut, ok := t.(*unboundType)
	return ok && ut.bound == nil
}

func (p *unboundType) Underlying() types.Type {
	panic("unbound type")
}

func (p *unboundType) String() string {
	panic("unbound type")
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
