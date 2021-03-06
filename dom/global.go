package dom

import (
	"sync"
)

// ----------------------------------------------------------------------------

// GlobalOpts type
type GlobalOpts struct {
	Import func(pkgPath string, global *Global) (*PkgRef, error)
}

// ----------------------------------------------------------------------------

// Global type
type Global struct {
	pkgImports map[string]*PkgRef
	mutex      sync.RWMutex

	fnImport func(pkgPath string, global *Global) (*PkgRef, error)
}

// NewGlobal func
func NewGlobal(opts *GlobalOpts) *Global {
	return &Global{
		pkgImports: make(map[string]*PkgRef),
		fnImport:   opts.Import,
	}
}

func (p *Global) findPkg(pkgPath string) (pkgImport *PkgRef, ok bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	pkgImport, ok = p.pkgImports[pkgPath]
	return
}

// Import func
func (p *Global) Import(pkgPath string) (pkgImport *PkgRef, err error) {
	pkgImport, ok := p.findPkg(pkgPath)
	if ok {
		return
	}
	pkg, err := p.fnImport(pkgPath, p)
	if err != nil {
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	pkgImport, ok = p.pkgImports[pkgPath]
	if ok {
		return
	}
	pkgImport = pkg
	p.pkgImports[pkgPath] = pkg
	return
}

// ----------------------------------------------------------------------------
