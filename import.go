package gox

import (
	"fmt"
	"go/token"
	"go/types"
	"os"

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

	// Name is the package name as it appears in the package source code.
	Name string

	// PkgPath is the package path as used by the go/types package.
	PkgPath string

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
}

// Ref func
func (p *PkgRef) Ref(name string) Ref {
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
		if pkg, ok := importPkgs[loadPkg.PkgPath]; ok && loadPkg.ID != "" {
			pkg.ID = loadPkg.ID
			pkg.Name = loadPkg.Name
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
