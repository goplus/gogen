package dom

// ----------------------------------------------------------------------------

// Code type
type Code struct {
	Tokens []Token
}

// ----------------------------------------------------------------------------

// CodeBuilder type
type CodeBuilder struct {
	code *Code
}

// NewVar func
func (p *CodeBuilder) NewVar(name string, pv **Var) *CodeBuilder {
	return p
}

// VarRef func
func (p *CodeBuilder) VarRef(v *Var) *CodeBuilder {
	return p
}

// Val func
func (p *CodeBuilder) Val(v Ref) *CodeBuilder {
	return p
}

// Const func
func (p *CodeBuilder) Const(v interface{}) *CodeBuilder {
	return p
}

// Assign func
func (p *CodeBuilder) Assign(n int) *CodeBuilder {
	return p
}

// Call func
func (p *CodeBuilder) Call(n int) *CodeBuilder {
	return p
}

// Defer func
func (p *CodeBuilder) Defer(n int) *CodeBuilder {
	return p
}

// Go func
func (p *CodeBuilder) Go() *CodeBuilder {
	return p
}

// EndStmt func
func (p *CodeBuilder) EndStmt() *CodeBuilder {
	return p
}

// End func
func (p *CodeBuilder) End() {
}

// ----------------------------------------------------------------------------
