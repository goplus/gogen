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
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	p.current.stmts = append(p.current.stmts, stmt)
	*pv = newVar(name, &spec.Type)
	return p
}

// VarRef func: p.VarRef(nil) means underscore (_)
func (p *CodeBuilder) VarRef(v *Var) *CodeBuilder {
	if v != nil {
		p.stk.Push(internal.Elem{
			Val:  ident(v.name),
			Type: &refType{typ: v.typ},
		})
	} else {
		p.stk.Push(internal.Elem{
			Val: underscore, // _
		})
	}
	return p
}

// Val func
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
	p.stk.Push(toExpr(p.pkg, v))
	return p
}

// Assign func
func (p *CodeBuilder) Assign(lhs int, v ...int) *CodeBuilder {
	var rhs int
	if v != nil {
		rhs = v[0]
	} else {
		rhs = lhs
	}
	args := p.stk.GetArgs(lhs + rhs)
	stmt := &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: make([]ast.Expr, lhs),
		Rhs: make([]ast.Expr, rhs),
	}
	pkg := p.pkg
	if lhs == rhs {
		for i := 0; i < lhs; i++ {
			assignMatchType(pkg, args[i], args[lhs+i].Type)
			stmt.Lhs[i] = args[i].Val
			stmt.Rhs[i] = args[lhs+i].Val
		}
	} else if rhs == 1 {
		rhsVals, ok := args[lhs].Type.(*types.Tuple)
		if !ok || lhs != rhsVals.Len() {
			panic("TODO: unmatch assignment")
		}
		for i := 0; i < lhs; i++ {
			assignMatchType(pkg, args[i], rhsVals.At(i).Type())
			stmt.Lhs[i] = args[i].Val
		}
		stmt.Rhs[0] = args[lhs].Val
	} else {
		panic("TODO: unmatch assignment")
	}
	p.current.stmts = append(p.current.stmts, stmt)
	p.stk.PopN(lhs + rhs)
	return p
}

const (
	HasEllipsis = token.Pos(1)
)

// Call func
func (p *CodeBuilder) Call(n int, ellipsis ...token.Pos) *CodeBuilder {
	args := p.stk.GetArgs(n)
	n++
	fn := p.stk.Get(-n)
	var hasEllipsis token.Pos
	if ellipsis != nil {
		hasEllipsis = 1
	}
	ret := toFuncCall(p.pkg, fn, args, hasEllipsis)
	p.stk.Ret(n, ret)
	return p
}

// BinaryOp func
func (p *CodeBuilder) BinaryOp(op token.Token) *CodeBuilder {
	pkg := p.pkg
	args := p.stk.GetArgs(2)
	if typ, ok := pkg.checkBuiltin(args[0].Type); ok {
		name := pkg.prefix.Operator + typ + binaryOps[op]
		fn := pkg.builtin.Scope().Lookup(name)
		if fn == nil {
			panic("TODO: operator not matched")
		}
		ret := toFuncCall(pkg, toObject(pkg, fn), args, token.NoPos)
		p.stk.Ret(2, ret)
	} else {
		panic("TODO: BinaryOp")
	}
	return p
}

var (
	binaryOps = [...]string{
		token.ADD: "_Add", // +
		token.SUB: "_Sub", // -
		token.MUL: "_Mul", // *
		token.QUO: "_Quo", // /
		token.REM: "_Rem", // %

		token.AND:     "_And",    // &
		token.OR:      "_Or",     // |
		token.XOR:     "_Xor",    // ^
		token.AND_NOT: "_AndNot", // &^
		token.SHL:     "_Lsh",    // <<
		token.SHR:     "_Rsh",    // >>

		token.LSS: "_LT",
		token.LEQ: "_LE",
		token.GTR: "_GT",
		token.GEQ: "_GE",
		token.EQL: "_EQ",
		token.NEQ: "_NE",
	}
)

// UnaryOp func
func (p *CodeBuilder) UnaryOp(op token.Token) *CodeBuilder {
	pkg := p.pkg
	args := p.stk.GetArgs(1)
	if typ, ok := pkg.checkBuiltin(args[0].Type); ok {
		name := pkg.prefix.Operator + typ + unaryOps[op]
		fn := pkg.builtin.Scope().Lookup(name)
		if fn == nil {
			panic("TODO: operator not matched")
		}
		ret := toFuncCall(pkg, toObject(pkg, fn), args, token.NoPos)
		p.stk.Ret(1, ret)
	} else {
		panic("TODO: UnaryOp")
	}
	return p
}

var (
	unaryOps = [...]string{
		token.SUB: "_Neg",
		token.XOR: "_Not",
	}
)

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
