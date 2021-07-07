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
)

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
	panic("VarStmt.InitStart")
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

// ----------------------------------------------------------------------------
