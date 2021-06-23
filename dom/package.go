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
		panic("TODO: exists")
	}
	p.typ = typ
	return p
}

// InitStart func
func (p *Var) InitStart() *CodeBuilder {
	return &CodeBuilder{}
}

// ----------------------------------------------------------------------------

// Func type
type Func struct {
	name string
	in   []*Param
	out  []*Param
}

// SetResults func
func (p *Func) SetResults(out ...*Param) *Func {
	p.out = out
	return p
}

// BodyStart func
func (p *Func) BodyStart(pkg *Package) *CodeBuilder {
	return &CodeBuilder{}
}

// ----------------------------------------------------------------------------

// Scope type
type Scope struct {
	parent *Scope
	syms   map[string]Ref
}

func (p *Scope) initScope(parent *Scope) {
	p.parent = parent
	p.syms = make(map[string]Ref)
}

func (p *Scope) addSymbol(name string, v Ref) (err error) {
	if _, ok := p.syms[name]; ok {
		panic("TODO: exists")
	}
	p.syms[name] = v
	return
}

// Ref func
func (p *Scope) Ref(name string) Ref {
	return nil
}

// ----------------------------------------------------------------------------

// Options type
type Options struct {
	Global *Global
}

// ----------------------------------------------------------------------------

// Package type
type Package struct {
	Scope
	gidx int
	gbl  *Global
}

func (p *Package) allocIdx() int {
	p.gidx++
	return p.gidx
}

// NewPkg func
func NewPkg(name string, opts *Options) *Package {
	pkg := &Package{
		gbl: opts.Global,
	}
	pkg.initScope(nil)
	return pkg
}

// MaxIndex func
func (p *Package) MaxIndex() int {
	return p.gidx + 1
}

// Import func
func (p *Package) Import(pkgPath string, name ...string) (pkgImport *PkgRef, err error) {
	if pkgImport, err = p.gbl.Import(pkgPath); err != nil {
		return
	}
	var pkgName string
	if name != nil {
		pkgName = name[0]
	} else {
		pkgName = pkgImport.Name()
	}
	err = p.addSymbol(pkgName, pkgImport)
	return
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
