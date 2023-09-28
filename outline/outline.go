package outline

import (
	"go/ast"
	"go/types"
)

// ----------------------------------------------------------------------------

// A Package describes a Go/Go+ package.
type Package struct {
	*types.Package
}

func From(pkg *types.Package) Package {
	return Package{pkg}
}

// ----------------------------------------------------------------------------

// A Scope maintains a set of objects and links to its containing
// (parent) and contained (children) scopes. Objects may be inserted
// and looked up by name. The zero value for Scope is a ready-to-use
// empty scope.
type Scope struct {
	*types.Scope
}

func ScopeFrom(o *types.Scope) Scope {
	return Scope{o}
}

// Scope returns the (complete or incomplete) package scope
// holding the objects declared at package level (TypeNames,
// Consts, Vars, and Funcs).
// For a nil pkg receiver, Scope returns the Universe scope.
func (p Package) Scope() Scope {
	return Scope{p.Package.Scope()}
}

// ----------------------------------------------------------------------------

type Func interface {
	types.Object

	// FullName returns the package- or receiver-type-qualified name of function or method obj.
	FullName() string

	// Scope returns the scope of the function's body block. The result is nil for imported or
	// instantiated functions and methods (but there is also no mechanism to get to an instantiated
	// function).
	Scope() *types.Scope

	// Comments returns associated documentation.
	Comments() *ast.CommentGroup
}

func (p Scope) InsertFunc(fn Func) types.Object {
	return p.Scope.Insert(fn)
}

// An FuncKind represents a function kind.
type FuncKind int

const (
	KindFunc FuncKind = iota
	KindMethod
)

// Kind returns kind of a function.
func Kind(t *types.Signature) FuncKind {
	recv := t.Recv()
	if recv == nil {
		return KindFunc
	}
	return KindMethod
}

// FuncFrom returns a function outline.
func FuncFrom(o types.Object) Func {
	return o.(Func)
}

var (
	// MethodFrom gets method outline from *types.Func.
	MethodFrom func(fn *types.Func) Func
)

// ----------------------------------------------------------------------------
