package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"
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
	p.endImport()
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

func matchType(arg, param types.Type) {
	if t, ok := arg.(*unboundType); ok {
		if t.bound == nil {
			param = types.Default(param)
			t.bound = param
			t.v.typ = param
			*t.v.ptype = toType(param)
			return
		}
		arg = t.bound
	}
	if !types.AssignableTo(arg, param) {
		log.Panicf("TODO: can't assign %v to %v", arg, param)
	}
}

// ----------------------------------------------------------------------------
