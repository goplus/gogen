package dom

// ----------------------------------------------------------------------------

// CodeBuilder type
type CodeBuilder struct {
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
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
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
func (p *CodeBuilder) Defer() *CodeBuilder {
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
