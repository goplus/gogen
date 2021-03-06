package dom

// ----------------------------------------------------------------------------

// Ref type
type Ref interface {
}

// ----------------------------------------------------------------------------

// PkgRef type
type PkgRef struct {
	name string
}

// Name func
func (p *PkgRef) Name() string {
	return p.name
}

// Ref func
func (p *PkgRef) Ref(name string) Ref {
	return nil
}

// ----------------------------------------------------------------------------
