package gox

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/packages"
)

// ----------------------------------------------------------------------------

// Config type
type Config struct {
	// Context specifies the context for the load operation.
	// If the context is cancelled, the loader may stop early
	// and return an ErrCancelled error.
	// If Context is nil, the load cannot be cancelled.
	Context context.Context

	// Logf is the logger for the config.
	// If the user provides a logger, debug logging is enabled.
	// If the GOPACKAGESDEBUG environment variable is set to true,
	// but the logger is nil, default to log.Printf.
	Logf func(format string, args ...interface{})

	// Dir is the directory in which to run the build system's query tool
	// that provides information about the packages.
	// If Dir is empty, the tool is run in the current directory.
	Dir string

	// Env is the environment to use when invoking the build system's query tool.
	// If Env is nil, the current environment is used.
	// As in os/exec's Cmd, only the last value in the slice for
	// each environment key is used. To specify the setting of only
	// a few variables, append to the current environment, as in:
	//
	//	opt.Env = append(os.Environ(), "GOOS=plan9", "GOARCH=386")
	//
	Env []string

	// BuildFlags is a list of command-line flags to be passed through to
	// the build system's query tool.
	BuildFlags []string

	// Fset provides source position information for syntax trees and types.
	// If Fset is nil, Load will use a new fileset, but preserve Fset's value.
	Fset *token.FileSet

	// ParseFile is called to read and parse each file
	// when preparing a package's type-checked syntax tree.
	// It must be safe to call ParseFile simultaneously from multiple goroutines.
	// If ParseFile is nil, the loader will uses parser.ParseFile.
	//
	// ParseFile should parse the source from src and use filename only for
	// recording position information.
	//
	// An application may supply a custom implementation of ParseFile
	// to change the effective file contents or the behavior of the parser,
	// or to modify the syntax tree. For example, selectively eliminating
	// unwanted function bodies can significantly accelerate type checking.
	ParseFile func(fset *token.FileSet, filename string, src []byte) (*ast.File, error)

	// LoadPkgs is called to load all import packages.
	LoadPkgs func(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int
}

// Package type
type Package struct {
	PkgRef
	cb         CodeBuilder
	importPkgs map[string]*PkgRef
	pkgPaths   []string
	conf       *Config
}

// NewPackage func
func NewPackage(pkgPath, name string, conf *Config) *Package {
	if conf == nil {
		conf = &Config{}
	}
	pkg := &Package{
		importPkgs: make(map[string]*PkgRef),
		conf:       conf,
	}
	pkg.PkgPath, pkg.Name = pkgPath, name
	pkg.Types = types.NewPackage(pkgPath, name)
	pkg.cb.init(pkg)
	return pkg
}

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
		pkgImport = &PkgRef{PkgPath: pkgPath}
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
	p.pkgPaths = p.pkgPaths[:0]
}

// ----------------------------------------------------------------------------
