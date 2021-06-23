package dom

// ----------------------------------------------------------------------------

// PkgImporter type
type PkgImporter interface {
	InitName(name string)
}

// ----------------------------------------------------------------------------

// Ref type
type Ref interface {
}

// ----------------------------------------------------------------------------

// PkgRef type
type PkgRef struct {
	name string
}

func newPkgRef() *PkgRef {
	return &PkgRef{}
}

func (p *PkgRef) InitName(name string) {
	if p.name != "" {
		panic("TODO")
	}
	p.name = name
}

// ----------------------------------------------------------------------------

// Name func
func (p *PkgRef) Name() string {
	return p.name
}

// Ref func
func (p *PkgRef) Ref(name string) Ref {
	return nil
}

// ----------------------------------------------------------------------------
