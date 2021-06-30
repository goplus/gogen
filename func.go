package gox

import (
	"go/token"
	"go/types"
)

// ----------------------------------------------------------------------------

// A Variable represents a function parameters and results.
type Param = types.Var

// NewParam returns a new variable representing a function parameter.
func (p *Package) NewParam(name string, typ types.Type) *Param {
	return types.NewParam(token.NoPos, p.Types, name, typ)
}

// ----------------------------------------------------------------------------

// A Tuple represents an ordered list of variables; a nil *Tuple is a valid (empty) tuple.
// Tuples are used as components of signatures and to represent the type of multiple
// assignments; they are not first class types of Go.
type Tuple = types.Tuple

// NewTuple returns a new tuple for the given parameters.
func NewTuple(x ...*Param) *Tuple {
	return types.NewTuple(x...)
}

// ----------------------------------------------------------------------------

// Func type
type Func struct {
	*types.Func
	old     codeBlockCtx
	closure bool
}

// BodyStart func
func (p *Func) BodyStart(pkg *Package) *CodeBuilder {
	return pkg.cb.startCodeBlock(p, &p.old)
}

func (p *Func) End(cb *CodeBuilder) {
	cb.endCodeBlock(p.old)
	if p.closure {
		//cb.stk.Push()
	}
}

// NewFunc func
func (p *Package) NewFunc(recv *Param, name string, params, results *Tuple, variadic bool) *Func {
	p.endImport()
	sig := types.NewSignature(recv, params, results, variadic)
	fn := types.NewFunc(token.NoPos, p.Types, name, sig)
	return &Func{Func: fn}
}

// ----------------------------------------------------------------------------
