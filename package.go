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

	// Builtin is the builin package.
	Builtin *types.Package

	// CheckBuiltinType checks a type is builtin type or not.
	CheckBuiltinType func(typ types.Type) (name string, is bool)
}

// Package type
type Package struct {
	PkgRef
	decls        []ast.Decl
	cb           CodeBuilder
	importPkgs   map[string]*PkgRef
	pkgPaths     []string
	conf         *Config
	prefix       *NamePrefix
	builtin      *types.Package
	checkBuiltin func(typ types.Type) (name string, ok bool)
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
	builtin := conf.Builtin
	if builtin == nil {
		builtin = newBuiltinDefault(prefix)
	}
	checkBuiltin := conf.CheckBuiltinType
	if checkBuiltin == nil {
		checkBuiltin = defaultCheckBuiltinType
	}
	pkg := &Package{
		importPkgs:   make(map[string]*PkgRef),
		conf:         conf,
		prefix:       prefix,
		builtin:      builtin,
		checkBuiltin: checkBuiltin,
	}
	pkg.Types = types.NewPackage(pkgPath, name)
	pkg.cb.init(pkg)
	return pkg
}

func Init_untyped_uint(builtin *types.Package, prefix *NamePrefix) types.Type {
	name := types.NewTypeName(token.NoPos, builtin, prefix.BuiltinType+"untyped_uint", nil)
	typ := types.NewNamed(name, types.Typ[types.Uint], nil)
	builtin.Scope().Insert(name)
	return typ
}

// ----------------------------------------------------------------------------

var (
	defaultNamePrefix = &NamePrefix{
		BuiltinType: "Gopb_",
		TypeExtend:  "Gope_",
		Operator:    "Gopo_",
		TypeConv:    "Gopc_",
	}
)

var (
	intBinaryOps = []string{
		"_Add", "_Sub", "_Mul", "_Quo", "_Rem", "_Or", "_Xor", "_And", "_AndNot"}
	numStringBooleanOps = []string{
		"_LT", "_LE", "_GT", "_GE", "_EQ", "_NE"}
	intTypes = []types.BasicKind{
		types.Int, types.Int64, types.Int32, types.Int16, types.Int8,
		types.Uint, types.Uintptr, types.Uint64, types.Uint32, types.Uint16, types.Uint8,
	}
)

func addIntType(builtin *types.Package, typ, untypedUint types.Type, prefix *NamePrefix) {
	gbl := builtin.Scope()
	opPrefix := prefix.Operator + typ.String()

	a := types.NewVar(token.NoPos, builtin, "a", typ)
	b := types.NewVar(token.NoPos, builtin, "b", typ)
	args := types.NewTuple(a, b)
	ret := types.NewTuple(types.NewVar(token.NoPos, builtin, "", typ))
	sig := types.NewSignature(nil, args, ret, false)

	// func opPrefix_type_op(a, b type) type
	for _, op := range intBinaryOps {
		gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+op, sig))
	}

	// func opPrefix_type_op(a type, n untyped_uint) type
	n := types.NewVar(token.NoPos, builtin, "n", untypedUint)
	args2 := types.NewTuple(a, n)
	sig2 := types.NewSignature(nil, args2, ret, false)
	gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+"_Lsh", sig2))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+"_Rsh", sig2))

	// func opPrefix_type_op(a, b type) bool
	ret3 := types.NewTuple(types.NewVar(token.NoPos, builtin, "", types.Typ[types.Bool]))
	sig3 := types.NewSignature(nil, args, ret3, false)
	for _, op := range numStringBooleanOps {
		gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+op, sig3))
	}

	// func opPrefix_type_op(a type) type
	args4 := types.NewTuple(a)
	sig4 := types.NewSignature(nil, args4, ret, false)
	gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+"_Neg", sig4))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+"_Not", sig4))
}

func addStringType(builtin *types.Package, prefix *NamePrefix) {
	gbl := builtin.Scope()
	typ := types.Typ[types.String]
	opPrefix := prefix.Operator + "string"

	a := types.NewVar(token.NoPos, builtin, "a", typ)
	b := types.NewVar(token.NoPos, builtin, "b", typ)
	args := types.NewTuple(a, b)
	ret := types.NewTuple(types.NewVar(token.NoPos, builtin, "", typ))
	sig := types.NewSignature(nil, args, ret, false)

	// func opPrefix_type_op(a, b type) type
	gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+"_Add", sig))

	// func opPrefix_type_op(a, b type) bool
	ret3 := types.NewTuple(types.NewVar(token.NoPos, builtin, "", types.Typ[types.Bool]))
	sig3 := types.NewSignature(nil, args, ret3, false)
	for _, op := range numStringBooleanOps {
		gbl.Insert(types.NewFunc(token.NoPos, builtin, opPrefix+op, sig3))
	}
}

func newBuiltinDefault(prefix *NamePrefix) *types.Package {
	builtin := types.NewPackage("", "")
	untypedUint := Init_untyped_uint(builtin, prefix)
	for _, intTy := range intTypes {
		addIntType(builtin, types.Typ[intTy], untypedUint, prefix)
	}
	addStringType(builtin, prefix)
	return builtin
}

func defaultCheckBuiltinType(typ types.Type) (name string, is bool) {
	if t, ok := typ.(*types.Basic); ok {
		if t.Kind() == types.UnsafePointer {
			return
		}
		return t.Name(), true
	}
	return
}

func isBuiltinOp(v types.Object) bool {
	return v.Pos() == token.NoPos
}

// ----------------------------------------------------------------------------
