package gox

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"strconv"

	"golang.org/x/tools/go/packages"
)

// ----------------------------------------------------------------------------

// Ref type
type Ref = types.Object

// ----------------------------------------------------------------------------

// An Error describes a problem with a package's metadata, syntax, or types.
type Error = packages.Error

// Module provides module information for a package.
type Module = packages.Module

// PkgRef type is a subset of golang.org/x/tools/go/packages.Package
type PkgRef struct {
	// ID is a unique identifier for a package,
	// in a syntax provided by the underlying build system.
	ID string

	// Errors contains any errors encountered querying the metadata
	// of the package, or while parsing or type-checking its files.
	Errors []Error

	// Types provides type information for the package.
	// The NeedTypes LoadMode bit sets this field for packages matching the
	// patterns; type information for dependencies may be missing or incomplete,
	// unless NeedDeps and NeedImports are also set.
	Types *types.Package

	// Fset provides position information for Types, TypesInfo, and Syntax.
	// It is set only when Types is set.
	Fset *token.FileSet

	// module is the module information for the package if it exists.
	Module *Module

	// IllTyped indicates whether the package or any dependency contains errors.
	// It is set only when Types is set.
	IllTyped bool

	pkg *Package // to import packages anywhere

	nameRefs []*ast.Ident // for internal use
}

// Ref returns the object in scope s with the given name if such an
// object exists; otherwise the result is nil.
func (p *PkgRef) Ref(name string) Ref {
	if p.Types == nil {
		p.pkg.endImport()
	}
	return p.Types.Scope().Lookup(name)
}

// ----------------------------------------------------------------------------

func loadGoPkgs(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int {
	conf := at.InternalGetLoadConfig()
	loadPkgs, err := packages.Load(conf, pkgPaths...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if n := packages.PrintErrors(loadPkgs); n > 0 {
		return n
	}
	for _, loadPkg := range loadPkgs {
		if pkg, ok := importPkgs[loadPkg.PkgPath]; ok && pkg.ID == "" {
			pkg.ID = loadPkg.ID
			pkg.Errors = loadPkg.Errors
			pkg.Types = loadPkg.Types
			pkg.Fset = loadPkg.Fset
			pkg.Module = loadPkg.Module
			pkg.IllTyped = loadPkg.IllTyped
		}
	}
	return 0
}

// ----------------------------------------------------------------------------

const (
	loadTypes = packages.NeedImports | packages.NeedDeps | packages.NeedTypes
	loadModes = loadTypes | packages.NeedName | packages.NeedModule
)

// InternalGetLoadConfig is a internal function. don't use it.
func (p *Package) InternalGetLoadConfig() *packages.Config {
	conf := p.conf
	return &packages.Config{
		Mode:       loadModes,
		Context:    conf.Context,
		Logf:       conf.Logf,
		Dir:        conf.Dir,
		Env:        conf.Env,
		BuildFlags: conf.BuildFlags,
		Fset:       conf.Fset,
		ParseFile:  conf.ParseFile,
	}
}

// Import func
func (p *Package) Import(pkgPath string) *PkgRef {
	// TODO: canonical pkgPath
	pkgImport, ok := p.importPkgs[pkgPath]
	if !ok {
		pkgImport = &PkgRef{pkg: p}
		p.importPkgs[pkgPath] = pkgImport
		p.pkgPaths = append(p.pkgPaths, pkgPath)
	}
	return pkgImport
}

func (p *Package) endImport() {
	if len(p.pkgPaths) == 0 {
		return
	}
	loadPkgs := p.conf.LoadPkgs
	if loadPkgs == nil {
		loadPkgs = loadGoPkgs
	}
	if n := loadPkgs(p, p.importPkgs, p.pkgPaths...); n > 0 {
		log.Panicf("total %d errors\n", n) // TODO: error message
	}
}

func (p *Package) getDecls() (decls []ast.Decl) {
	n := len(p.pkgPaths)
	if n == 0 {
		return p.decls
	}
	specs := make([]ast.Spec, 0, n)
	names := p.newAutoNames()
	for _, pkgPath := range p.pkgPaths {
		pkg := p.importPkgs[pkgPath]
		if pkg.Types == nil { // unused
			continue
		}
		pkgName, renamed := names.RequireName(pkg.Types.Name())
		if renamed {
			pkg.Types.SetName(pkgName)
			for _, nameRef := range pkg.nameRefs {
				nameRef.Name = pkgName
			}
		}
		specs = append(specs, &ast.ImportSpec{
			Name: ident(pkgName),
			Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(pkgPath)},
		})
	}
	if len(specs) == 0 {
		return p.decls
	}
	decls = make([]ast.Decl, 0, len(p.decls)+1)
	decls = append(decls, &ast.GenDecl{Tok: token.IMPORT, Specs: specs})
	decls = append(decls, p.decls...)
	return
}

// ----------------------------------------------------------------------------

type null struct{}
type autoNames struct {
	gbl   *types.Scope
	names map[string]null
	idx   int
}

func (p *Package) newAutoNames() *autoNames {
	return &autoNames{
		gbl:   p.Types.Scope(),
		names: make(map[string]null),
	}
}

func scopeHasName(at *types.Scope, name string) bool {
	if at.Lookup(name) != nil {
		return true
	}
	for i := at.NumChildren(); i > 0; {
		i--
		if scopeHasName(at.Child(i), name) {
			return true
		}
	}
	return false
}

func (p *autoNames) importHasName(name string) bool {
	_, ok := p.names[name]
	return ok
}

func (p *autoNames) hasName(name string) bool {
	return scopeHasName(p.gbl, name) || p.importHasName(name) || types.Universe.Lookup(name) != nil
}

func (p *autoNames) RequireName(name string) (ret string, renamed bool) {
	ret = name
	for p.hasName(ret) {
		p.idx++
		ret = name + strconv.Itoa(p.idx)
		renamed = true
	}
	p.names[name] = null{}
	return
}

// ----------------------------------------------------------------------------
