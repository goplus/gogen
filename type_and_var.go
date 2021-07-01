package gox

import (
	"go/token"
	"go/types"
)

// ----------------------------------------------------------------------------

// A Variable represents a declared variable (including function parameters and results, and struct fields).
type Var = types.Var

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
	*p.pv = types.NewVar(token.NoPos, p.pkg.Types, p.name, typ)
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
	t types.Type
}

func (p *refType) Underlying() types.Type {
	return p
}

func (p *refType) String() string {
	return "&" + p.t.String()
}

// ----------------------------------------------------------------------------
