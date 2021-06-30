package gox

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

type codeBlock interface {
	End(cb *CodeBuilder)
}

type codeBlockCtx struct {
	codeBlock
	base  int
	stmts []ast.Stmt
}

// CodeBuilder type
type CodeBuilder struct {
	stk     internal.Stack
	current codeBlockCtx
	pkg     *Package
}

func (p *CodeBuilder) init(pkg *Package) {
	p.pkg = pkg
	p.stk.Init()
}

func (p *CodeBuilder) startCodeBlock(current codeBlock, old *codeBlockCtx) *CodeBuilder {
	p.current, *old = codeBlockCtx{current, p.stk.Len(), nil}, p.current
	return p
}

func (p *CodeBuilder) endCodeBlock(old codeBlockCtx) []ast.Stmt {
	stmts := p.current.stmts
	p.current = old
	p.stk.SetLen(old.base)
	return stmts
}

// NewClosure func
func (p *CodeBuilder) NewClosure(params, results *Tuple, variadic bool) *Func {
	sig := types.NewSignature(nil, params, results, variadic)
	fn := types.NewFunc(token.NoPos, p.pkg.Types, "", sig)
	return &Func{Func: fn}
}

// NewVar func
func (p *CodeBuilder) NewVar(name string, pv **Var) *CodeBuilder {
	return p
}

// VarRef func
func (p *CodeBuilder) VarRef(v *Var) *CodeBuilder {
	return p
}

// Val func
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
	return p
}

// Assign func
func (p *CodeBuilder) Assign(n int) *CodeBuilder {
	return p
}

// Call func
func (p *CodeBuilder) Call(n int) *CodeBuilder {
	return p
}

// Defer func
func (p *CodeBuilder) Defer() *CodeBuilder {
	return p
}

// Go func
func (p *CodeBuilder) Go() *CodeBuilder {
	return p
}

// EndStmt func
func (p *CodeBuilder) EndStmt() *CodeBuilder {
	return p
}

// End func
func (p *CodeBuilder) End() *CodeBuilder {
	p.current.End(p)
	return p
}

// ----------------------------------------------------------------------------
