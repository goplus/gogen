package dom

import (
	"sync"
)

// ----------------------------------------------------------------------------

// GlobalOpts type
type GlobalOpts struct {
	Import func(pi PkgImporter, pkgPath string, global *Global) error
}

// ----------------------------------------------------------------------------

// Global type
type Global struct {
	pkgImports map[string]*PkgRef
	mutex      sync.Mutex

	fnImport func(pi PkgImporter, pkgPath string, global *Global) error
}

// NewGlobal func
func NewGlobal(opts *GlobalOpts) *Global {
	return &Global{
		pkgImports: make(map[string]*PkgRef),
		fnImport:   opts.Import,
	}
}

func (p *Global) requirePkg(pkgPath string) (pkgImport *PkgRef, ok bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if pkgImport, ok = p.pkgImports[pkgPath]; !ok {
		pkgImport = newPkgRef()
		p.pkgImports[pkgPath] = pkgImport
	}
	return
}

// Import func
func (p *Global) Import(pkgPath string) (pkgImport *PkgRef, err error) {
	pkgImport, ok := p.requirePkg(pkgPath)
	if !ok {
		err = p.fnImport(pkgImport, pkgPath, p)
	}
	return
}

// ----------------------------------------------------------------------------
