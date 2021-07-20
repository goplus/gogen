package gox

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
)

type LoadPkgsFunc = func(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int
type LoadUnderlyingFunc = func(at *Package, typ *types.Named) types.Type

// TypeExtend is to extend Go builtin types.
type TypeExtend interface {
	// Default returns the default "typed" type for an "untyped" type;
	// it returns the incoming type for all other types. The default type
	// for untyped nil is untyped nil.
	Default(t types.Type) types.Type

	// AssignableTo reports whether a value of type V is assignable to a variable of type T.
	AssignableTo(V, T types.Type) bool
}

// ----------------------------------------------------------------------------

type BuiltinContracts struct {
	NInteger, Integer, Float, Complex, Number, Addable, Orderable, Comparable Contract
}

type PkgImporter interface {
	Import(pkgPath string) *PkgRef
}

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
	LoadPkgs LoadPkgsFunc

	// LoadUnderlying is called to load a delay load type.
	LoadUnderlying LoadUnderlyingFunc

	// TypeExtend is to extend Go builtin types.
	TypeExtend TypeExtend

	// Prefix is name prefix.
	Prefix string

	// Contracts are the builtin contracts.
	Contracts *BuiltinContracts

	// NewBuiltin is to create the builin package.
	NewBuiltin func(pkg PkgImporter, prefix string, contracts *BuiltinContracts) *types.Package
}

// Package type
type Package struct {
	PkgRef
	decls          []ast.Decl
	cb             CodeBuilder
	importPkgs     map[string]*PkgRef
	allPkgPaths    []string // all import pkgPaths
	delayPkgPaths  []string // all delay-load pkgPaths
	conf           *Config
	prefix         string
	builtin        *types.Package
	loadPkgs       LoadPkgsFunc
	loadUnderlying LoadUnderlyingFunc
	typExt         TypeExtend
	autoPrefix     string
	autoIdx        int
}

// NewPackage creates a new package.
func NewPackage(pkgPath, name string, conf *Config) *Package {
	if conf == nil {
		conf = &Config{}
	}
	prefix := conf.Prefix
	if prefix == "" {
		prefix = defaultNamePrefix
	}
	contracts := conf.Contracts
	if contracts == nil {
		contracts = defaultContracts
	}
	newBuiltin := conf.NewBuiltin
	if newBuiltin == nil {
		newBuiltin = newBuiltinDefault
	}
	loadPkgs := conf.LoadPkgs
	if loadPkgs == nil {
		loadPkgs = LoadGoPkgs
	}
	loadUnderlying := conf.LoadUnderlying
	if loadUnderlying == nil {
		loadUnderlying = noLoadUnderlying
	}
	typExt := conf.TypeExtend
	if typExt == nil {
		typExt = &goTypes{}
	}
	pkg := &Package{
		importPkgs:     make(map[string]*PkgRef),
		conf:           conf,
		prefix:         prefix,
		loadPkgs:       loadPkgs,
		loadUnderlying: loadUnderlying,
		typExt:         typExt,
		autoPrefix:     "_auto" + prefix,
	}
	pkg.Types = types.NewPackage(pkgPath, name)
	pkg.cb.init(pkg)
	pkg.builtin = newBuiltin(pkg, prefix, contracts)
	return pkg
}

// Builtin returns the buitlin package.
func (p *Package) Builtin() *PkgRef {
	return &PkgRef{Types: p.builtin, Fset: p.Fset, pkg: p}
}

// CB returns the code builder.
func (p *Package) CB() *CodeBuilder {
	return &p.cb
}

// ----------------------------------------------------------------------------
