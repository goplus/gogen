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

// ConstDecl type
type ConstDecl struct {
	name string
	typ  types.Type
	old  codeBlock
}

func (p *ConstDecl) InitType(typ types.Type) *ConstDecl {
	if p.typ != nil {
		panic("const type is already initialized")
	}
	p.typ = typ
	return p
}

func (p *ConstDecl) BodyStart(pkg *Package) *CodeBuilder {
	cb := pkg.ConstStart()
	p.old = cb.startInitExpr(p)
	return cb
}

func (p *ConstDecl) End(cb *CodeBuilder) {
	pkg := cb.pkg
	elem := cb.stk.Pop()
	if p.typ != nil {
		if err := checkMatchType(pkg, elem.Type, p.typ); err != nil {
			panic(err)
		}
	} else {
		p.typ = elem.Type
	}
	cb.endInitExpr(p.old)
	tv := evalConstExpr(pkg, elem.Val)
	cb.current.scope.Insert(types.NewConst(token.NoPos, pkg.Types, p.name, p.typ, tv.Value))
	var typExpr ast.Expr
	if !isUntyped(p.typ) {
		typExpr = toType(pkg, p.typ)
	}
	pkg.decls = append(pkg.decls, &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{ident(p.name)},
				Type:   typExpr,
				Values: []ast.Expr{elem.Val},
			},
		},
	})
}

func (p *Package) NewConst(name string) *ConstDecl {
	return &ConstDecl{name: name}
}

// ----------------------------------------------------------------------------

// A Variable represents a declared variable (including function parameters and results, and struct fields).
type Var struct {
	name  string
	typ   types.Type
	ptype *ast.Expr
}

func newVar(name string, ptype *ast.Expr) *Var {
	v := &Var{name: name, ptype: ptype}
	v.typ = &unboundType{v: v}
	return v
}

// VarDecl type
type VarDecl struct {
	name string
	typ  types.Type
	pkg  *Package
	pv   **Var
}

// MatchType func
func (p *VarDecl) MatchType(typ types.Type) *VarDecl {
	if p.typ != nil {
		if p.typ == typ {
			return p
		}
		panic("TODO: unmatched type")
	}
	p.typ = typ
	//*p.pv = TODO:
	types.NewVar(token.NoPos, p.pkg.Types, p.name, typ)
	return p
}

// InitStart func
func (p *VarDecl) InitStart() *CodeBuilder {
	panic("TODO: VarDecl.InitStart")
}

// NewVar func
func (p *Package) NewVar(name string, pv **Var) *VarDecl {
	return &VarDecl{name, nil, p, pv}
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
	v     *Var
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
