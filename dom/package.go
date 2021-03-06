package dom

// ----------------------------------------------------------------------------

// Type type
type Type interface {
}

// ----------------------------------------------------------------------------

// Param type
type Param struct {
	name string
	typ  Type
	idx  int
}

// Name func
func (p *Param) Name() string {
	return p.name
}

// Type func
func (p *Param) Type() Type {
	return p.typ
}

// Index func
func (p *Param) Index() int {
	return p.idx
}

// ----------------------------------------------------------------------------

// Var type
type Var struct {
	Param
}

// SetType func
func (p *Var) SetType(typ Type) *Var {
	if p.typ != nil {
		if p.typ != typ {
			panic("TODO")
		}
	} else {
		p.typ = typ
	}
	return p
}

// ----------------------------------------------------------------------------

// Func type
type Func struct {
	name string
	in   []*Param
	out  []*Param
	body *Code
}

// SetResults func
func (p *Func) SetResults(out ...*Param) *Func {
	return p
}

// BodyStart func
func (p *Func) BodyStart(pkg *Package) *CodeBuilder {
	return &CodeBuilder{}
}

// ----------------------------------------------------------------------------

// Scope type
type Scope struct {
	Parent *Scope
}

// ----------------------------------------------------------------------------

// Options type
type Options struct {
}

// ----------------------------------------------------------------------------

// Package type
type Package struct {
	Scope
	gidx int
}

func (p *Package) allocIdx() int {
	p.gidx++
	return p.gidx
}

// NewPkg func
func NewPkg(name string, opts ...*Options) *Package {
	pkg := &Package{}
	return pkg
}

// MaxIndex func
func (p *Package) MaxIndex() int {
	return p.gidx + 1
}

// Import func
func (p *Package) Import(path string, name ...string) *PkgRef {
	return &PkgRef{}
}

// NewVar func
func (p *Package) NewVar(name string, typ Type) *Var {
	return &Var{}
}

// NewParam func
func (p *Package) NewParam(name string, typ Type) *Param {
	return &Param{name: name, typ: typ, idx: p.allocIdx()}
}

// NewFunc func
func (p *Package) NewFunc(name string, params ...*Param) *Func {
	return &Func{}
}

// ----------------------------------------------------------------------------
