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
	"go/constant"
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
	label *ast.LabeledStmt
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

// Scope returns current scope.
func (p *CodeBuilder) Scope() *types.Scope {
	return p.current.scope
}

func (p *CodeBuilder) Pkg() *Package {
	return p.pkg
}

func (p *CodeBuilder) startFuncBody(fn *Func, old *funcBodyCtx) *CodeBuilder {
	p.current.fn, old.fn = fn, p.current.fn
	p.startBlockStmt(fn, "func "+fn.Name(), &old.codeBlockCtx)
	scope := p.current.scope
	sig := fn.Type().(*types.Signature)
	insertParams(scope, sig.Params())
	insertParams(scope, sig.Results())
	return p
}

func insertParams(scope *types.Scope, params *types.Tuple) {
	for i, n := 0, params.Len(); i < n; i++ {
		v := params.At(i)
		if name := v.Name(); name != "" {
			scope.Insert(v)
		}
	}
}

func (p *CodeBuilder) endFuncBody(old funcBodyCtx) []ast.Stmt {
	p.current.fn = old.fn
	return p.endBlockStmt(old.codeBlockCtx)
}

func (p *CodeBuilder) startBlockStmt(current codeBlock, comment string, old *codeBlockCtx) *CodeBuilder {
	scope := types.NewScope(p.current.scope, token.NoPos, token.NoPos, comment)
	p.current.codeBlockCtx, *old = codeBlockCtx{current, scope, p.stk.Len(), nil, nil}, p.current.codeBlockCtx
	return p
}

func (p *CodeBuilder) endBlockStmt(old codeBlockCtx) []ast.Stmt {
	if p.current.label != nil {
		p.emitStmt(&ast.EmptyStmt{})
	}
	stmts := p.current.stmts
	p.stk.SetLen(p.current.base)
	p.current.codeBlockCtx = old
	return stmts
}

func (p *CodeBuilder) clearBlockStmt() []ast.Stmt {
	stmts := p.current.stmts
	p.current.stmts = nil
	return stmts
}

func (p *CodeBuilder) emitStmt(stmt ast.Stmt) {
	if p.current.label != nil {
		p.current.label.Stmt = stmt
		stmt, p.current.label = p.current.label, nil
	}
	p.current.stmts = append(p.current.stmts, stmt)
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
	if debug {
		t := sig.Params()
		for i, n := 0, t.Len(); i < n; i++ {
			v := t.At(i)
			if _, ok := v.Type().(*unboundType); ok {
				panic("TODO: can't use auto type in func parameters")
			}
		}
	}
	fn := types.NewFunc(token.NoPos, p.pkg.Types, "", sig)
	return &Func{Func: fn}
}

// NewConstStart func
func (p *CodeBuilder) NewConstStart(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewConstStart", typ, names)
	}
	return p.pkg.newValueDecl(token.CONST, typ, names...).InitStart(p.pkg)
}

// NewVar func
func (p *CodeBuilder) NewVar(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewVar", typ, names)
	}
	p.pkg.newValueDecl(token.VAR, typ, names...)
	return p
}

// NewVarStart func
func (p *CodeBuilder) NewVarStart(typ types.Type, names ...string) *CodeBuilder {
	if debug {
		log.Println("NewVarStart", typ, names)
	}
	return p.pkg.newValueDecl(token.VAR, typ, names...).InitStart(p.pkg)
}

// DefineVarStart func
func (p *CodeBuilder) DefineVarStart(names ...string) *CodeBuilder {
	if debug {
		log.Println("DefineVarStart", names)
	}
	return p.pkg.newValueDecl(token.DEFINE, nil, names...).InitStart(p.pkg)
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(name string, pv **types.Var) *CodeBuilder {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	if debug {
		log.Println("NewAutoVar", name)
	}
	p.emitStmt(stmt)
	typ := &unboundType{ptypes: []*ast.Expr{&spec.Type}}
	*pv = types.NewVar(token.NoPos, p.pkg.Types, name, typ)
	if p.current.scope.Insert(*pv) != nil {
		log.Panicln("TODO: variable already defined -", name)
	}
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

// None func
func (p *CodeBuilder) None() *CodeBuilder {
	if debug {
		log.Println("None")
	}
	p.stk.Push(internal.Elem{})
	return p
}

func (p *CodeBuilder) ZeroLit(typ types.Type) *CodeBuilder {
	if debug {
		log.Println("ZeroLit")
	}
	ret := &ast.CompositeLit{}
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound == nil {
			t.ptypes = append(t.ptypes, &ret.Type)
		} else {
			typ = t.tBound
			ret.Type = toType(p.pkg, typ)
		}
	default:
		ret.Type = toType(p.pkg, typ)
	}
	p.stk.Push(internal.Elem{Type: typ, Val: ret})
	return p
}

// MapLit func
func (p *CodeBuilder) MapLit(t *types.Map, arity int) *CodeBuilder {
	if debug {
		log.Println("MapLit", t, arity)
	}
	pkg := p.pkg
	if arity == 0 {
		if t == nil {
			t = types.NewMap(types.Typ[types.String], TyEmptyInterface)
		}
		ret := &ast.CompositeLit{Type: toMapType(pkg, t)}
		p.stk.Push(internal.Elem{Type: t, Val: ret})
		return p
	}
	if (arity & 1) != 0 {
		panic("TODO: MapLit - invalid arity")
	}
	var key, val types.Type
	var args = p.stk.GetArgs(arity)
	var check = (t != nil)
	if check {
		key, val = t.Key(), t.Elem()
	} else {
		key = boundElementType(args, 0, arity, 2)
		val = boundElementType(args, 1, arity, 2)
		t = types.NewMap(types.Default(key), types.Default(val))
	}
	elts := make([]ast.Expr, arity>>1)
	for i := 0; i < arity; i += 2 {
		elts[i>>1] = &ast.KeyValueExpr{Key: args[i].Val, Value: args[i+1].Val}
		if check {
			if !AssignableTo(args[i].Type, key) {
				log.Panicf("TODO: MapLit - can't assign %v to %v\n", args[i].Type, key)
			} else if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: MapLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
		}
	}
	ret := &ast.CompositeLit{
		Type: toMapType(pkg, t),
		Elts: elts,
	}
	p.stk.Ret(arity, internal.Elem{Type: t, Val: ret})
	return p
}

func toBoundArrayLen(pkg *Package, elts []internal.Elem, arity int) int {
	n := -1
	max := -1
	for i := 0; i < arity; i += 2 {
		if elts[i].Val != nil {
			n = toIntVal(elts[i].CVal)
		} else {
			n++
		}
		if max < n {
			max = n
		}
	}
	return max + 1
}

func toIntVal(cval constant.Value) int {
	if cval != nil {
		if v, ok := constant.Int64Val(cval); ok {
			return int(v)
		}
	}
	panic("TODO: not a constant integer or too large")
}

func toLitElemExpr(args []internal.Elem, i int) ast.Expr {
	key := args[i].Val
	if key == nil {
		return args[i+1].Val
	}
	return &ast.KeyValueExpr{Key: key, Value: args[i+1].Val}
}

// SliceLit func
func (p *CodeBuilder) SliceLit(t *types.Slice, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var pkg = p.pkg
	var keyValMode = (keyVal != nil && keyVal[0])
	if debug {
		log.Println("SliceLit", t, arity, keyValMode)
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			panic("TODO: SliceLit - invalid arity")
		}
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		n := arity >> 1
		elts = make([]ast.Expr, n)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: SliceLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
			elts[i>>1] = toLitElemExpr(args, i)
		}
	} else {
		if arity == 0 {
			if t == nil {
				t = types.NewSlice(TyEmptyInterface)
			}
			ret := &ast.CompositeLit{Type: toSliceType(pkg, t)}
			p.stk.Push(internal.Elem{Type: t, Val: ret})
			return p
		}
		var val types.Type
		var args = p.stk.GetArgs(arity)
		var check = (t != nil)
		if check {
			val = t.Elem()
		} else {
			val = boundElementType(args, 0, arity, 1)
			t = types.NewSlice(types.Default(val))
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if check {
				if !AssignableTo(arg.Type, val) {
					log.Panicf("TODO: SliceLit - can't assign %v to %v\n", arg.Type, val)
				}
			}
		}
	}
	ret := &ast.CompositeLit{
		Type: toSliceType(pkg, t),
		Elts: elts,
	}
	p.stk.Ret(arity, internal.Elem{Type: t, Val: ret})
	return p
}

// ArrayLit func
func (p *CodeBuilder) ArrayLit(t *types.Array, arity int, keyVal ...bool) *CodeBuilder {
	var elts []ast.Expr
	var keyValMode = (keyVal != nil && keyVal[0])
	if debug {
		log.Println("ArrayLit", t, arity, keyValMode)
	}
	if keyValMode { // in keyVal mode
		if (arity & 1) != 0 {
			panic("TODO: ArrayLit - invalid arity")
		}
		args := p.stk.GetArgs(arity)
		max := toBoundArrayLen(p.pkg, args, arity)
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(max))
		} else if int(n) < max {
			log.Panicf("TODO: array index %v out of bounds [0:%v]\n", max, n)
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(args[i+1].Type, val) {
				log.Panicf("TODO: ArrayLit - can't assign %v to %v\n", args[i+1].Type, val)
			}
			elts[i>>1] = toLitElemExpr(args, i)
		}
	} else {
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(arity))
		} else if int(n) < arity {
			log.Panicf("TODO: array index %v out of bounds [0:%v]\n", arity, n)
		}
		args := p.stk.GetArgs(arity)
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if !AssignableTo(arg.Type, val) {
				log.Panicf("TODO: ArrayLit - can't assign %v to %v\n", arg.Type, val)
			}
		}
	}
	ret := &ast.CompositeLit{
		Type: toArrayType(p.pkg, t),
		Elts: elts,
	}
	p.stk.Ret(arity, internal.Elem{Type: t, Val: ret})
	return p
}

func (p *CodeBuilder) SliceGet(slice3 bool) *CodeBuilder { // a[i:j:k]
	if debug {
		log.Println("SliceGet", slice3)
	}
	n := 3
	if slice3 {
		n++
	}
	args := p.stk.GetArgs(n)
	x := args[0]
	typ := x.Type
	if _, ok := typ.(*types.Slice); !ok {
		if t, ok := typ.(*types.Basic); ok && t.Kind() == types.String || t.Kind() == types.UntypedString {
			if slice3 {
				log.Panicln("TODO: invalid operation `???` (3-index slice of string)")
			}
		} else {
			log.Panicln("TODO: slice on non slice object -", x.Type)
		}
	}
	var exprMax ast.Expr
	if slice3 {
		exprMax = args[3].Val
	}
	// TODO: check type
	elem := internal.Elem{
		Val: &ast.SliceExpr{
			X: x.Val, Low: args[1].Val, High: args[2].Val, Max: exprMax, Slice3: slice3,
		},
		Type: typ,
	}
	p.stk.Ret(n, elem)
	return p
}

func (p *CodeBuilder) IndexGet(nidx int, twoValue bool) *CodeBuilder {
	if debug {
		log.Println("IndexGet", nidx, twoValue)
	}
	if nidx != 1 {
		panic("TODO: IndexGet doesn't support a[i, j...] already")
	}
	args := p.stk.GetArgs(2)
	typs, allowTwoValue := getIdxValTypes(args[0].Type)
	var tyRet types.Type
	if twoValue { // elem, ok = a[key]
		if !allowTwoValue {
			panic("TODO: doesn't return twoValue")
		}
		pkg := p.pkg
		tyRet = types.NewTuple(pkg.NewParam("", typs[1]), pkg.NewParam("", types.Typ[types.Bool]))
	} else { // elem = a[key]
		tyRet = typs[1]
	}
	elem := internal.Elem{
		Val:  &ast.IndexExpr{X: args[0].Val, Index: args[1].Val},
		Type: tyRet,
	}
	// TODO: check index type
	p.stk.Ret(2, elem)
	return p
}

func (p *CodeBuilder) IndexRef(nidx int) *CodeBuilder {
	if debug {
		log.Println("IndexRef", nidx)
	}
	if nidx != 1 {
		panic("TODO: IndexRef doesn't support a[i, j...] = val already")
	}
	args := p.stk.GetArgs(2)
	typ := args[0].Type
	elemRef := internal.Elem{
		Val: &ast.IndexExpr{X: args[0].Val, Index: args[1].Val},
	}
	if t, ok := typ.(*unboundType); ok {
		tyMapElem := &unboundMapElemType{key: args[1].Type, typ: t}
		elemRef.Type = &refType{typ: tyMapElem}
	} else {
		typs, _ := getIdxValTypes(typ)
		elemRef.Type = &refType{typ: typs[1]}
		// TODO: check index type
	}
	p.stk.Ret(2, elemRef)
	return p
}

func getIdxValTypes(typ types.Type) ([]types.Type, bool) {
	switch t := typ.(type) {
	case *types.Slice:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Map:
		return []types.Type{t.Key(), t.Elem()}, true
	case *types.Array:
		return []types.Type{tyInt, t.Elem()}, false
	case *types.Pointer:
		if e, ok := t.Elem().(*types.Array); ok {
			return []types.Type{tyInt, e.Elem()}, false
		}
	}
	log.Panicln("TODO: can't index of type", typ)
	return nil, false
}

var (
	tyInt = types.Typ[types.Int]
)

// Typ func
func (p *CodeBuilder) Typ(typ types.Type) *CodeBuilder {
	if debug {
		log.Println("Typ", typ)
	}
	p.stk.Push(internal.Elem{
		Val:  toType(p.pkg, typ),
		Type: NewTypeType(typ),
	})
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
	var ret internal.Elem
	if o, ok := v.(*types.TypeName); ok {
		if typ := o.Type(); isType(typ) {
			ret = internal.Elem{Val: toType(p.pkg, typ), Type: NewTypeType(typ)}
		} else {
			ret = toObject(p.pkg, o)
		}
	} else {
		ret = toExpr(p.pkg, v)
	}
	p.stk.Push(ret)
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
	if debug {
		log.Println("Assign", lhs, rhs)
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
	p.emitStmt(stmt)
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
	p.emitStmt(&ast.ReturnStmt{Results: rets})
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

// Star func
func (p *CodeBuilder) Star() *CodeBuilder {
	if debug {
		log.Println("Star")
	}
	arg := p.stk.Get(-1)
	ret := internal.Elem{Val: &ast.StarExpr{X: arg.Val}}
	switch t := arg.Type.(type) {
	case *TypeType:
		t.typ = types.NewPointer(t.typ)
		ret.Type = arg.Type
	case *types.Pointer:
		ret.Type = t.Elem()
	default:
		log.Panicln("TODO: can't use *X to a non pointer value -", t)
	}
	p.stk.Ret(1, ret)
	return p
}

// IncDec func
func (p *CodeBuilder) IncDec(op token.Token) *CodeBuilder {
	if debug {
		log.Println("IncDec", op)
	}
	pkg := p.pkg
	args := p.stk.GetArgs(1)
	name := pkg.prefix.Operator + incdecOps[op]
	fn := pkg.builtin.Scope().Lookup(name)
	if fn == nil {
		panic("TODO: operator not matched")
	}
	switch t := fn.Type().(type) {
	case *instructionType:
		if _, err := t.instr.Call(pkg, args, token.NoPos); err != nil {
			panic(err)
		}
	default:
		panic("TODO: IncDec not found?")
	}
	p.stk.Pop()
	return p
}

var (
	incdecOps = [...]string{
		token.INC: "Inc",
		token.DEC: "Dec",
	}
)

// Send func
func (p *CodeBuilder) Send() *CodeBuilder {
	if debug {
		log.Println("Send")
	}
	val := p.stk.Pop()
	ch := p.stk.Pop()
	// TODO: check types
	p.emitStmt(&ast.SendStmt{Chan: ch.Val, Value: val.Val})
	return p
}

// Defer func
func (p *CodeBuilder) Defer() *CodeBuilder {
	if debug {
		log.Println("Defer")
	}
	arg := p.stk.Pop()
	call, ok := arg.Val.(*ast.CallExpr)
	if !ok {
		panic("TODO: please use defer callExpr()")
	}
	p.emitStmt(&ast.DeferStmt{Call: call})
	return p
}

// Go func
func (p *CodeBuilder) Go() *CodeBuilder {
	if debug {
		log.Println("Go")
	}
	arg := p.stk.Pop()
	call, ok := arg.Val.(*ast.CallExpr)
	if !ok {
		panic("TODO: please use go callExpr()")
	}
	p.emitStmt(&ast.GoStmt{Call: call})
	return p
}

// If func
func (p *CodeBuilder) If() *CodeBuilder {
	if debug {
		log.Println("If")
	}
	stmt := &ifStmt{}
	p.startBlockStmt(stmt, "if statement", &stmt.old)
	return p
}

// Then func
func (p *CodeBuilder) Then() *CodeBuilder {
	if debug {
		log.Println("Then")
	}
	if p.stk.Len() == p.current.base {
		panic("use None() for empty expr")
	}
	if flow, ok := p.current.codeBlock.(controlFlow); ok {
		flow.Then(p)
		return p
	}
	panic("use if..then or switch..then please")
}

// Else func
func (p *CodeBuilder) Else() *CodeBuilder {
	if debug {
		log.Println("Else")
	}
	if flow, ok := p.current.codeBlock.(*ifStmt); ok {
		flow.Else(p)
		return p
	}
	panic("use if..else please")
}

// Switch func
func (p *CodeBuilder) Switch() *CodeBuilder {
	if debug {
		log.Println("Switch")
	}
	stmt := &switchStmt{}
	p.startBlockStmt(stmt, "switch statement", &stmt.old)
	return p
}

// Case func
func (p *CodeBuilder) Case(n int) *CodeBuilder { // n=0 means default case
	if debug {
		log.Println("Case", n)
	}
	if flow, ok := p.current.codeBlock.(*switchStmt); ok {
		flow.Case(p, n)
		return p
	}
	panic("use switch..case please")
}

// Label func
func (p *CodeBuilder) Label(name string) *CodeBuilder {
	if debug {
		log.Println("Label", name)
	}
	label := types.NewLabel(token.NoPos, p.pkg.Types, name)
	if p.current.scope.Insert(label) != nil {
		log.Panicf("TODO: label name %v exists\n", name)
	}
	p.current.label = &ast.LabeledStmt{Label: ident(name)}
	return p
}

// Goto func
func (p *CodeBuilder) Goto(name string) *CodeBuilder {
	if debug {
		log.Println("Goto", name)
	}
	label := p.getLabel(name, true)
	p.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: label})
	return p
}

func (p *CodeBuilder) getLabel(name string, must bool) *ast.Ident {
	if name == "" {
		if must {
			log.Panicln("TODO: need label name")
		}
		return nil
	}
	_, o := p.current.scope.LookupParent(name, token.NoPos)
	if o == nil {
		log.Panicln("TODO: label not found -", name)
	}
	if o.Type() != types.Typ[types.Invalid] {
		log.Panicf("TODO: %v not a label name\n", name)
	}
	return ident(name)
}

// Break func
func (p *CodeBuilder) Break(name string) *CodeBuilder {
	if debug {
		log.Println("Break", name)
	}
	label := p.getLabel(name, false)
	p.emitStmt(&ast.BranchStmt{Tok: token.BREAK, Label: label})
	return p
}

// Continue func
func (p *CodeBuilder) Continue(name string) *CodeBuilder {
	if debug {
		log.Println("Continue", name)
	}
	label := p.getLabel(name, false)
	p.emitStmt(&ast.BranchStmt{Tok: token.CONTINUE, Label: label})
	return p
}

// Fallthrough func
func (p *CodeBuilder) Fallthrough() *CodeBuilder {
	if debug {
		log.Println("Fallthrough")
	}
	if flow, ok := p.current.codeBlock.(*caseStmt); ok {
		flow.Fallthrough(p)
		return p
	}
	panic("please use fallthrough in case statement")
}

// For func
func (p *CodeBuilder) For() *CodeBuilder {
	if debug {
		log.Println("For")
	}
	stmt := &forStmt{}
	p.startBlockStmt(stmt, "for statement", &stmt.old)
	return p
}

// Post func
func (p *CodeBuilder) Post() *CodeBuilder {
	if debug {
		log.Println("Post")
	}
	if flow, ok := p.current.codeBlock.(*forStmt); ok {
		flow.Post(p)
		return p
	}
	panic("please use Post() in for statement")
}

func (p *CodeBuilder) ForRange(names ...string) *CodeBuilder {
	if debug {
		log.Println("ForRange", names)
	}
	stmt := &forRangeStmt{names: names}
	p.startBlockStmt(stmt, "for range statement", &stmt.old)
	return p
}

// RangeAssignThen func
func (p *CodeBuilder) RangeAssignThen() *CodeBuilder {
	if debug {
		log.Println("RangeAssignThen")
	}
	if flow, ok := p.current.codeBlock.(*forRangeStmt); ok {
		flow.RangeAssignThen(p)
		return p
	}
	panic("please use RangeAssignThen() in for range statement")
}

// EndStmt func
func (p *CodeBuilder) EndStmt() *CodeBuilder {
	n := p.stk.Len() - p.current.base
	if n > 0 {
		if n != 1 {
			panic("syntax error: unexpected newline, expecting := or = or comma")
		}
		stmt := &ast.ExprStmt{X: p.stk.Pop().Val}
		p.emitStmt(stmt)
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
		log.Println("EndInit", n)
	}
	p.varDecl.EndInit(p, n)
	p.varDecl = nil
	return p
}

// ----------------------------------------------------------------------------
