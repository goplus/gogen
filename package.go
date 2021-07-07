package gox

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
)

// ----------------------------------------------------------------------------

// NamePrefix config
type NamePrefix struct {
	BuiltinType string // builtin type
	TypeExtend  string // type extend
	Operator    string // operator
	TypeConv    string // type convert
}

type BuiltinContracts struct {
	NInteger, Integer, Float, Complex, Number, Addable, Orderable, Comparable Contract
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
	LoadPkgs func(at *Package, importPkgs map[string]*PkgRef, pkgPaths ...string) int

	// Prefix is name prefix.
	Prefix *NamePrefix

	// Contracts are the builtin contracts.
	Contracts *BuiltinContracts

	// Builtin is the builin package.
	Builtin *types.Package
}

// Package type
type Package struct {
	PkgRef
	decls      []ast.Decl
	cb         CodeBuilder
	importPkgs map[string]*PkgRef
	pkgPaths   []string
	conf       *Config
	prefix     *NamePrefix
	builtin    *types.Package
}

// NewPackage func
func NewPackage(pkgPath, name string, conf *Config) *Package {
	if conf == nil {
		conf = &Config{}
	}
	prefix := conf.Prefix
	if prefix == nil {
		prefix = defaultNamePrefix
	}
	contracts := conf.Contracts
	if contracts == nil {
		contracts = defaultContracts
	}
	builtin := conf.Builtin
	if builtin == nil {
		builtin = newBuiltinDefault(prefix, contracts)
	}
	pkg := &Package{
		importPkgs: make(map[string]*PkgRef),
		conf:       conf,
		prefix:     prefix,
		builtin:    builtin,
	}
	pkg.Types = types.NewPackage(pkgPath, name)
	pkg.cb.init(pkg)
	return pkg
}

// ----------------------------------------------------------------------------
