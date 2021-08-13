/*
 Copyright 2021 The GoPlus Authors (goplus.org)
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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

	file *file // to import packages anywhere
	pkg  *Package

	// IllTyped indicates whether the package or any dependency contains errors.
	// It is set only when Types is set.
	IllTyped bool

	inTestingFile bool // this package is refered in a testing file.

	isUsed   bool
	nameRefs []*ast.Ident // for internal use
}

func (p *PkgRef) markUsed(v *ast.Ident) {
	if p.isUsed {
		return
	}
	for _, ref := range p.nameRefs {
		if ref == v {
			p.isUsed = true
			return
		}
	}
}

// Ref returns the object in scope s with the given name if such an
// object exists; otherwise the result is nil.
func (p *PkgRef) Ref(name string) Ref {
	p.EnsureImported()
	return p.Types.Scope().Lookup(name)
}

// EnsureImported ensures this package is imported.
func (p *PkgRef) EnsureImported() {
	if p.Types == nil {
		p.file.endImport(p.pkg, p.inTestingFile)
	}
}

// ----------------------------------------------------------------------------

// LoadGoPkgs loads and returns the Go packages named by the given pkgPaths.
func LoadGoPkgs(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int {
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
		LoadGoPkg(at, importPkgs, loadPkg)
	}
	return 0
}

func LoadGoPkg(at *Package, imports map[string]*PkgRef, loadPkg *packages.Package) {
	for _, impPkg := range loadPkg.Imports {
		if _, ok := imports[impPkg.PkgPath]; ok {
			continue
		}
		LoadGoPkg(at, imports, impPkg)
	}
	if debugImport {
		log.Println("==> Import", loadPkg.PkgPath)
	}
	pkg, ok := imports[loadPkg.PkgPath]
	if ok {
		if pkg.ID == "" {
			pkg.ID = loadPkg.ID
			pkg.Errors = loadPkg.Errors
			pkg.Types = loadPkg.Types
			pkg.Fset = loadPkg.Fset
			pkg.Module = loadPkg.Module
			pkg.IllTyped = loadPkg.IllTyped
		}
	} else {
		imports[loadPkg.PkgPath] = &PkgRef{
			pkg:      at,
			ID:       loadPkg.ID,
			Errors:   loadPkg.Errors,
			Types:    loadPkg.Types,
			Fset:     loadPkg.Fset,
			Module:   loadPkg.Module,
			IllTyped: loadPkg.IllTyped,
		}
	}
}

type loadPkgsCached struct {
	imports  map[string]*PkgRef
	pkgsLoad func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error)
}

func (p *loadPkgsCached) load(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int {
	var unimportedPaths []string
retry:
	for _, pkgPath := range pkgPaths {
		if loadPkg, ok := p.imports[pkgPath]; ok {
			if pkg, ok := importPkgs[pkgPath]; ok {
				typs := *loadPkg.Types
				pkg.ID = loadPkg.ID
				pkg.Errors = loadPkg.Errors
				pkg.Types = &typs // clone *types.Package instance
				pkg.Fset = loadPkg.Fset
				pkg.Module = loadPkg.Module
				pkg.IllTyped = loadPkg.IllTyped
			}
		} else {
			unimportedPaths = append(unimportedPaths, pkgPath)
		}
	}
	if len(unimportedPaths) > 0 {
		conf := at.InternalGetLoadConfig()
		loadPkgs, err := p.pkgsLoad(conf, unimportedPaths...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if n := packages.PrintErrors(loadPkgs); n > 0 {
			return n
		}
		for _, loadPkg := range loadPkgs {
			LoadGoPkg(at, p.imports, loadPkg)
		}
		pkgPaths, unimportedPaths = unimportedPaths, nil
		goto retry
	}
	return 0
}

// NewLoadPkgsCached returns a cached
func NewLoadPkgsCached(
	load func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error)) LoadPkgsFunc {
	imports := make(map[string]*PkgRef)
	if load == nil {
		load = packages.Load
	}
	return (&loadPkgsCached{imports: imports, pkgsLoad: load}).load
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
	return p.files[p.inTestingFile].importPkg(p, pkgPath, p.inTestingFile != 0)
}

func (p *Package) big() *PkgRef {
	return p.files[p.inTestingFile].big(p, p.inTestingFile != 0)
}

// ----------------------------------------------------------------------------

type null struct{}
type autoNames struct {
	gbl     *types.Scope
	builtin *types.Scope
	names   map[string]null
	idx     int
}

func (p *Package) autoName() string {
	p.autoIdx++
	return p.autoPrefix + strconv.Itoa(p.autoIdx)
}

func (p *Package) newAutoNames() *autoNames {
	return &autoNames{
		gbl:     p.Types.Scope(),
		builtin: p.builtin.Scope(),
		names:   make(map[string]null),
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
	return scopeHasName(p.gbl, name) || p.importHasName(name) ||
		p.builtin.Lookup(name) != nil || types.Universe.Lookup(name) != nil
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
