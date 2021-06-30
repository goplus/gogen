package gox

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/goplus/gox/internal"
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

func ident(name string) *ast.Ident {
	return &ast.Ident{Name: name}
}

func newField(name string, typ types.Type) *ast.Field {
	return &ast.Field{
		Names: []*ast.Ident{ident(name)},
		Type:  toType(typ),
	}
}

func toType(typ types.Type) ast.Expr {
	return nil
}

func toFuncType(sig *types.Signature) *ast.FuncType {
	return nil
}

// ----------------------------------------------------------------------------

// Func type
type Func struct {
	*types.Func
	decl *ast.FuncDecl
	old  codeBlockCtx
}

// BodyStart func
func (p *Func) BodyStart(pkg *Package) *CodeBuilder {
	return pkg.cb.startCodeBlock(p, &p.old)
}

// End is for internal use.
func (p *Func) End(cb *CodeBuilder) {
	stmts := cb.endCodeBlock(p.old)
	body := &ast.BlockStmt{List: stmts}
	t := p.Type().(*types.Signature)
	if fn := p.decl; fn == nil { // is closure
		expr := &ast.FuncLit{Type: toFuncType(t), Body: body}
		cb.stk.Push(internal.Elem{Val: expr, Type: t})
	} else {
		fn.Name, fn.Type, fn.Body = ident(p.Name()), toFuncType(t), body
		if recv := t.Recv(); recv != nil {
			params := []*ast.Field{newField(recv.Name(), recv.Type())}
			fn.Recv = &ast.FieldList{Opening: 1, List: params, Closing: 1}
		}
	}
}

// NewFunc func
func (p *Package) NewFunc(recv *Param, name string, params, results *Tuple, variadic bool) *Func {
	if name == "" {
		panic("TODO: no func name")
	}
	p.endImport()

	sig := types.NewSignature(recv, params, results, variadic)
	fn := types.NewFunc(token.NoPos, p.Types, name, sig)
	p.Types.Scope().Insert(fn)

	decl := &ast.FuncDecl{}
	p.decls = append(p.decls, decl)
	return &Func{Func: fn, decl: decl}
}

// ----------------------------------------------------------------------------
