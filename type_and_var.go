package gox

import (
	"go/token"
	"go/types"
)

// A Variable represents a declared variable (including function parameters and results, and struct fields).
type Var = types.Var

// ----------------------------------------------------------------------------

// VarStmt type
type VarStmt struct {
	name string
	typ  types.Type
	pkg  *Package
	pv   **Var
}

// MatchType func
func (p *VarStmt) MatchType(typ types.Type) *VarStmt {
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
func (p *VarStmt) InitStart() *CodeBuilder {
	return &CodeBuilder{}
}

// NewVar func
func (p *Package) NewVar(name string, pv **Var) *VarStmt {
	p.endImport()
	return &VarStmt{name, nil, p, pv}
}

// ----------------------------------------------------------------------------
