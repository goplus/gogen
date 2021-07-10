/*
 Copyright 2021 The GoPlus Authors (goplus.org)
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"reflect"

	"github.com/goplus/gox/internal"
)

var (
	debug bool
)

func SetDebug(d bool) {
	debug = d
}

// ----------------------------------------------------------------------------

type codeBlock interface {
	End(cb *CodeBuilder)
}

type codeBlockCtx struct {
	codeBlock
	scope *types.Scope
	base  int
	stmts []ast.Stmt
}

type funcBodyCtx struct {
	codeBlockCtx
	fn *Func
}

// CodeBuilder type
type CodeBuilder struct {
	stk     internal.Stack
	current funcBodyCtx
	pkg     *Package
	varDecl *ValueDecl
}

func (p *CodeBuilder) init(pkg *Package) {
	p.pkg = pkg
	p.current.scope = pkg.Types.Scope()
	p.stk.Init()
}

func (p *CodeBuilder) Pkg() *Package {
	return p.pkg
}

func (p *CodeBuilder) startFuncBody(fn *Func, old *funcBodyCtx) *CodeBuilder {
	p.current.fn, old.fn = fn, p.current.fn
	return p.startBlockStmt(fn, "func "+fn.FullName(), &old.codeBlockCtx)
}

func (p *CodeBuilder) endFuncBody(old funcBodyCtx) []ast.Stmt {
	p.current.fn = old.fn
	return p.endBlockStmt(old.codeBlockCtx)
}

func (p *CodeBuilder) startBlockStmt(current codeBlock, comment string, old *codeBlockCtx) *CodeBuilder {
	scope := types.NewScope(p.current.scope, token.NoPos, token.NoPos, comment)
	p.current.codeBlockCtx, *old = codeBlockCtx{current, scope, p.stk.Len(), nil}, p.current.codeBlockCtx
	return p
}

func (p *CodeBuilder) endBlockStmt(old codeBlockCtx) []ast.Stmt {
	stmts := p.current.stmts
	p.stk.SetLen(p.current.base)
	p.current.codeBlockCtx = old
	return stmts
}

func (p *CodeBuilder) startInitExpr(current codeBlock) (old codeBlock) {
	p.current.codeBlock, old = current, p.current.codeBlock
	return
}

func (p *CodeBuilder) endInitExpr(old codeBlock) {
	p.current.codeBlock = old
}

// NewClosure func
func (p *CodeBuilder) NewClosure(params, results *Tuple, variadic bool) *Func {
	sig := types.NewSignature(nil, params, results, variadic)
	return p.NewClosureWith(sig)
}

// NewClosureWith func
func (p *CodeBuilder) NewClosureWith(sig *types.Signature) *Func {
	fn := types.NewFunc(token.NoPos, p.pkg.Types, "", sig)
	return &Func{Func: fn}
}

// NewVar func
func (p *CodeBuilder) NewVar(typ types.Type, names ...string) *CodeBuilder {
	p.pkg.newValueDecl(token.VAR, typ, names...)
	return p
}

// NewVarStart func
func (p *CodeBuilder) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	return p.pkg.newValueDecl(token.VAR, typ, names...).InitStart(p.pkg)
}

// DefineVarStart func
func (p *CodeBuilder) DefineVarStart(names ...string) *CodeBuilder {
	return p.pkg.newValueDecl(token.DEFINE, nil, names...).InitStart(p.pkg)
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(name string, pv **AutoVar) *CodeBuilder {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	if debug {
		log.Println("NewAutoVar", name)
	}
	// TODO: scope.Insert this variable
	p.current.stmts = append(p.current.stmts, stmt)
	*pv = newAutoVar(name, &spec.Type)
	return p
}

// VarRef func: p.VarRef(nil) means underscore (_)
func (p *CodeBuilder) VarRef(ref interface{}) *CodeBuilder {
	if ref == nil {
		if debug {
			log.Println("VarRef _")
		}
		p.stk.Push(internal.Elem{
			Val: underscore, // _
		})
	} else {
		switch v := ref.(type) {
		case *AutoVar:
			if debug {
				log.Println("VarRef", v.name)
			}
			p.stk.Push(internal.Elem{
				Val:  ident(v.name),
				Type: &refType{typ: v.typ},
			})
		case *types.Var:
			if debug {
				log.Println("VarRef", v.Name())
			}
			p.stk.Push(internal.Elem{
				Val:  ident(v.Name()),
				Type: &refType{typ: v.Type()},
			})
		default:
			log.Panicln("TODO: VarRef", reflect.TypeOf(ref))
		}
	}
	return p
}

// Val func
func (p *CodeBuilder) Val(v interface{}) *CodeBuilder {
	if debug {
		if o, ok := v.(types.Object); ok {
			log.Println("Val", o.Name())
		} else {
			log.Println("Val", v)
		}
	}
	p.stk.Push(toExpr(p.pkg, v))
	return p
}

// MemberVal func
func (p *CodeBuilder) MemberVal(name string) *CodeBuilder {
	if debug {
		log.Println("MemberVal", name)
	}
	arg := p.stk.Get(-1)
	switch o := indirect(arg.Type).(type) {
	case *types.Named:
		for i, n := 0, o.NumMethods(); i < n; i++ {
			method := o.Method(i)
			if method.Name() == name {
				p.stk.Ret(1, internal.Elem{
					Val:  &ast.SelectorExpr{X: arg.Val, Sel: ident(name)},
					Type: methodTypeOf(method.Type()),
				})
				return p
			}
		}
		if struc, ok := o.Underlying().(*types.Struct); ok {
			p.fieldVal(arg.Val, struc, name)
		} else {
			panic("TODO: member not found - " + name)
		}
	case *types.Struct:
		p.fieldVal(arg.Val, o, name)
	default:
		log.Panicln("TODO: MemberVal - unexpected type:", o)
	}
	return p
}

func (p *CodeBuilder) fieldVal(x ast.Expr, struc *types.Struct, name string) {
	if t := structFieldType(struc, name); t != nil {
		p.stk.Ret(1, internal.Elem{
			Val:  &ast.SelectorExpr{X: x, Sel: ident(name)},
			Type: t,
		})
	} else {
		panic("TODO: member not found - " + name)
	}
}

func structFieldType(o *types.Struct, name string) types.Type {
	for i, n := 0, o.NumFields(); i < n; i++ {
		fld := o.Field(i)
		if fld.Name() == name {
			return fld.Type()
		}
	}
	return nil
}

func methodTypeOf(typ types.Type) types.Type {
	switch sig := typ.(type) {
	case *types.Signature:
		return types.NewSignature(nil, sig.Params(), sig.Results(), sig.Variadic())
	default:
		panic("TODO: methodTypeOf")
	}
}

func indirect(typ types.Type) types.Type {
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
	}
	return typ
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
			assignMatchType(pkg, args[i].Type, args[lhs+i].Type)
			stmt.Lhs[i] = args[i].Val
			stmt.Rhs[i] = args[lhs+i].Val
		}
	} else if rhs == 1 {
		rhsVals, ok := args[lhs].Type.(*types.Tuple)
		if !ok || lhs != rhsVals.Len() {
			panic("TODO: unmatch assignment")
		}
		for i := 0; i < lhs; i++ {
			assignMatchType(pkg, args[i].Type, rhsVals.At(i).Type())
			stmt.Lhs[i] = args[i].Val
		}
		stmt.Rhs[0] = args[lhs].Val
	} else {
		panic("TODO: unmatch assignment")
	}
	if debug {
		log.Println("Assign", lhs, rhs)
	}
	p.current.stmts = append(p.current.stmts, stmt)
	p.stk.PopN(lhs + rhs)
	return p
}

// Call func
func (p *CodeBuilder) Call(n int, ellipsis ...bool) *CodeBuilder {
	args := p.stk.GetArgs(n)
	n++
	fn := p.stk.Get(-n)
	var hasEllipsis token.Pos
	if ellipsis != nil && ellipsis[0] {
		hasEllipsis = 1
	}
	if debug {
		log.Println("Call", n-1, int(hasEllipsis))
	}
	ret := toFuncCall(p.pkg, fn, args, hasEllipsis)
	p.stk.Ret(n, ret)
	return p
}

// Return func
func (p *CodeBuilder) Return(n int) *CodeBuilder {
	if debug {
		log.Println("Return", n)
	}
	results := p.current.fn.Type().(*types.Signature).Results()
	args := p.stk.GetArgs(n)
	if err := checkMatchFuncResults(p.pkg, args, results); err != nil {
		panic(err)
	}
	var rets []ast.Expr
	if n > 0 {
		rets = make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			rets[i] = args[i].Val
		}
		p.stk.PopN(n)
	}
	p.current.stmts = append(p.current.stmts, &ast.ReturnStmt{Results: rets})
	return p
}

// BinaryOp func
func (p *CodeBuilder) BinaryOp(op token.Token) *CodeBuilder {
	pkg := p.pkg
	args := p.stk.GetArgs(2)
	name := pkg.prefix.Operator + binaryOps[op]
	fn := pkg.builtin.Scope().Lookup(name)
	if fn == nil {
		panic("TODO: operator not matched")
	}
	ret := toFuncCall(pkg, toObject(pkg, fn), args, token.NoPos)
	if debug {
		log.Println("BinaryOp", op, "// ret", ret.Type)
	}
	p.stk.Ret(2, ret)
	return p
}

var (
	binaryOps = [...]string{
		token.ADD: "Add", // +
		token.SUB: "Sub", // -
		token.MUL: "Mul", // *
		token.QUO: "Quo", // /
		token.REM: "Rem", // %

		token.AND:     "And",    // &
		token.OR:      "Or",     // |
		token.XOR:     "Xor",    // ^
		token.AND_NOT: "AndNot", // &^
		token.SHL:     "Lsh",    // <<
		token.SHR:     "Rsh",    // >>

		token.LSS: "LT",
		token.LEQ: "LE",
		token.GTR: "GT",
		token.GEQ: "GE",
		token.EQL: "EQ",
		token.NEQ: "NE",
	}
)

// UnaryOp func
func (p *CodeBuilder) UnaryOp(op token.Token) *CodeBuilder {
	pkg := p.pkg
	args := p.stk.GetArgs(1)
	name := pkg.prefix.Operator + unaryOps[op]
	fn := pkg.builtin.Scope().Lookup(name)
	if fn == nil {
		panic("TODO: operator not matched")
	}
	ret := toFuncCall(pkg, toObject(pkg, fn), args, token.NoPos)
	if debug {
		log.Println("UnaryOp", op, "// ret", ret.Type)
	}
	p.stk.Ret(1, ret)
	return p
}

var (
	unaryOps = [...]string{
		token.SUB: "Neg",
		token.XOR: "Not",
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
	if debug {
		log.Println("End")
	}
	p.current.End(p)
	return p
}

// EndInit func
func (p *CodeBuilder) EndInit(n int) *CodeBuilder {
	if debug {
		log.Println("EndInit")
	}
	p.varDecl.EndInit(p, n)
	p.varDecl = nil
	return p
}

// ----------------------------------------------------------------------------
