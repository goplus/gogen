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
	p.stk.SetLen(p.current.base)
	p.current = old
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
	panic("CodeBuilder.NewVar")
}

// VarRef func
func (p *CodeBuilder) VarRef(v *Var) *CodeBuilder {
	panic("CodeBuilder.VarRef")
}

// Val func
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
	p.stk.Push(toExpr(v))
	return p
}

// Assign func
func (p *CodeBuilder) Assign(n int) *CodeBuilder {
	panic("CodeBuilder.Assign")
}

// Call func
func (p *CodeBuilder) Call(n int) *CodeBuilder {
	args := p.stk.GetArgs(n)
	n++
	fn := p.stk.Get(-n)
	ret := toFuncCall(fn, args)
	p.stk.Ret(n, ret)
	return p
}

// Defer func
func (p *CodeBuilder) Defer() *CodeBuilder {
	panic("CodeBuilder.Defer")
}

// Go func
func (p *CodeBuilder) Go() *CodeBuilder {
	panic("CodeBuilder.Go")
}

// EndStmt func
func (p *CodeBuilder) EndStmt() *CodeBuilder {
	n := p.stk.Len() - p.current.base
	if n > 0 {
		if n != 1 {
			panic("syntax error: unexpected newline, expecting := or = or comma")
		}
		stmt := &ast.ExprStmt{X: p.stk.Pop().Val}
		p.current.stmts = append(p.current.stmts, stmt)
	}
	return p
}

// End func
func (p *CodeBuilder) End() *CodeBuilder {
	p.current.End(p)
	return p
}

// ----------------------------------------------------------------------------
