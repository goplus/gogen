package dom

// ----------------------------------------------------------------------------

// Token type
type Token interface {
	tokenNode()
}

// VarRefToken type
type VarRefToken struct {
	Var *Var
}

// VarToken type
type VarToken struct {
	Var *Var
}

// ConstToken type
type ConstToken struct {
	Value interface{}
}

// AssignToken type
type AssignToken struct {
	N int
}

// FuncToken type
type FuncToken struct {
	Func Ref
}

// CallToken type
type CallToken struct {
	NArg int
}

// DeferToken type
type DeferToken struct {
}

// GoToken type
type GoToken struct {
}

// EndStmtToken type
type EndStmtToken struct {
}

// ----------------------------------------------------------------------------

func (p *VarRefToken) tokenNode()  {}
func (p *VarToken) tokenNode()     {}
func (p *ConstToken) tokenNode()   {}
func (p *AssignToken) tokenNode()  {}
func (p *FuncToken) tokenNode()    {}
func (p *CallToken) tokenNode()    {}
func (p *DeferToken) tokenNode()   {}
func (p *GoToken) tokenNode()      {}
func (p *EndStmtToken) tokenNode() {}

// ----------------------------------------------------------------------------
