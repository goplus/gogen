//go:build gengo
// +build gengo

/*
Copyright 2026 The XGo Authors (xgo.dev)
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

package gogen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/go/printer"
	"github.com/goplus/gogen/internal/target"
)

// ----------------------------------------------------------------------------

// emitMapStringAnyAssert generates a type assertion statement for empty interface (any) values.
// It emits: `tmpVar, _ := argVal.(map[string]any)` and returns the tmpVar identifier.
// This enables safe member access on any types by converting them to map[string]any.
// See https://github.com/goplus/xgo/issues/2572#issuecomment-3807206716
func (p *CodeBuilder) emitMapStringAnyAssert(argVal ast.Expr) ast.Expr {
	pkg := p.pkg
	e := &ast.TypeAssertExpr{X: argVal, Type: toMapType(pkg, tyMapStringAny)}
	ret := ast.NewIdent(pkg.autoName())
	stmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ret, underscore},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{e},
	}
	p.emitStmt(stmt)
	return ret
}

// TypeAssert func
func (p *CodeBuilder) TypeAssert(typ types.Type, lhs int, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("TypeAssert", typ, lhs)
	}
	arg := p.stk.Get(-1)
	xType, ok := p.checkInterface(arg.Type)
	if !ok {
		text, pos, end := p.loadExpr(getSrc(src))
		p.panicCodeErrorf(
			pos, end, "invalid type assertion: %s (non-interface type %v on left)", text, arg.Type)
	}
	if missing := p.missingMethod(typ, xType); missing != "" {
		pos := getSrcPos(getSrc(src))
		end := getSrcEnd(getSrc(src))
		p.panicCodeErrorf(
			pos, end, "impossible type assertion:\n\t%v does not implement %v (missing %s method)",
			typ, arg.Type, missing)
	}
	pkg := p.pkg
	ret := &ast.TypeAssertExpr{X: arg.Val, Type: toType(pkg, typ)}
	if lhs == 2 {
		tyRet := types.NewTuple(
			pkg.NewParam(token.NoPos, "", typ),
			pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
		p.stk.Ret(1, &internal.Elem{Type: tyRet, Val: ret, Src: getSrc(src)})
	} else {
		p.stk.Ret(1, &internal.Elem{Type: typ, Val: ret, Src: getSrc(src)})
	}
	return p
}

func (p *CodeBuilder) mapIndexExpr(o *types.Map, name string, lhs int, argVal ast.Expr, src ast.Node) MemberKind {
	tyRet := o.Elem()
	if lhs == 2 { // two-value assignment
		pkg := p.pkg.Types
		tyRet = types.NewTuple(
			types.NewParam(token.NoPos, pkg, "", tyRet),
			types.NewParam(token.NoPos, pkg, "", types.Typ[types.Bool]))
	}
	p.stk.Ret(1, &internal.Elem{
		Val: &ast.IndexExpr{X: argVal, Index: stringLit(name)}, Type: tyRet, Src: src,
	})
	return MemberField
}

// MapLitEx func
func (p *CodeBuilder) MapLitEx(typ types.Type, arity int, src ...ast.Node) error {
	if debugInstr {
		log.Println("MapLit", typ, arity)
	}
	var t *types.Map
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		typExpr = toType(pkg, typ)
		var ok bool
	retry:
		switch tt := typ.(type) {
		case *types.Named:
			t, ok = p.getUnderlying(tt).(*types.Map)
		case *types.Map:
			t, ok = tt, true
		case *types.Alias:
			typ = types.Unalias(tt)
			goto retry
		}
		if !ok {
			return p.newCodeErrorf(getPos(src), getEnd(src), "type %v isn't a map", typ)
		}
	}
	if arity == 0 {
		if t == nil {
			t = types.NewMap(types.Typ[types.String], TyEmptyInterface)
			typ = t
			typExpr = toMapType(pkg, t)
		}
		ret := &ast.CompositeLit{Type: typExpr}
		p.stk.Push(&internal.Elem{Type: typ, Val: ret, Src: getSrc(src)})
		return nil
	}
	if (arity & 1) != 0 {
		return p.newCodeErrorf(getPos(src), getEnd(src), "MapLit: invalid arity, can't be odd - %d", arity)
	}
	var key, val types.Type
	var args = p.stk.GetArgs(arity)
	var check = (t != nil)
	if check {
		key, val = t.Key(), t.Elem()
	} else {
		key = boundElementType(pkg, args, 0, arity, 2)
		val = boundElementType(pkg, args, 1, arity, 2)
		t = types.NewMap(Default(pkg, key), Default(pkg, val))
		typ = t
		typExpr = toMapType(pkg, t)
	}
	elts := make([]ast.Expr, arity>>1)
	for i := 0; i < arity; i += 2 {
		elts[i>>1] = &ast.KeyValueExpr{Key: args[i].Val, Value: args[i+1].Val}
		if check {
			if !AssignableTo(pkg, args[i].Type, key) {
				code, pos, end := p.loadExpr(args[i].Src)
				return p.newCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in map key", code, args[i].Type, key)
			} else if !AssignableTo(pkg, args[i+1].Type, val) {
				code, pos, end := p.loadExpr(args[i+1].Src)
				return p.newCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in map value", code, args[i+1].Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return nil
}

// SliceLitEx func
func (p *CodeBuilder) SliceLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	var elts []ast.Expr
	if debugInstr {
		log.Println("SliceLit", typ, arity, keyVal)
	}
	var t *types.Slice
	var typExpr ast.Expr
	var pkg = p.pkg
	if typ != nil {
		typExpr = toType(pkg, typ)
	retry:
		switch tt := typ.(type) {
		case *types.Named:
			t = p.getUnderlying(tt).(*types.Slice)
		case *types.Slice:
			t = tt
		case *types.Alias:
			typ = types.Unalias(typ)
			goto retry
		default:
			p.panicCodeErrorf(getPos(src), getEnd(src), "type %v isn't a slice", typ)
		}
	}
	if keyVal { // in keyVal mode
		if (arity & 1) != 0 {
			log.Panicln("SliceLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		n := arity >> 1
		elts = make([]ast.Expr, n)
		for i := 0; i < arity; i += 2 {
			arg := args[i+1]
			if !AssignableConv(pkg, arg.Type, val, arg) {
				src, pos, end := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in slice literal", src, args[i+1].Type, val)
			}
			elts[i>>1] = p.indexElemExpr(args, i)
		}
	} else {
		if arity == 0 {
			if t == nil {
				t = types.NewSlice(TyEmptyInterface)
				typ = t
				typExpr = toSliceType(pkg, t)
			}
			p.stk.Push(&internal.Elem{
				Type: typ, Val: &ast.CompositeLit{Type: typExpr}, Src: getSrc(src),
			})
			return p
		}
		var val types.Type
		var args = p.stk.GetArgs(arity)
		var check = (t != nil)
		if check {
			val = t.Elem()
		} else {
			val = boundElementType(pkg, args, 0, arity, 1)
			t = types.NewSlice(Default(pkg, val))
			typ = t
			typExpr = toSliceType(pkg, t)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if check {
				if !AssignableConv(pkg, arg.Type, val, arg) {
					src, pos, end := p.loadExpr(arg.Src)
					p.panicCodeErrorf(
						pos, end, "cannot use %s (type %v) as type %v in slice literal", src, arg.Type, val)
				}
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return p
}

func (p *CodeBuilder) indexElemExpr(args []*internal.Elem, i int) ast.Expr {
	key := args[i].Val
	if key == nil { // none
		return args[i+1].Val
	}
	p.toIntVal(args[i], "index which must be non-negative integer constant")
	return &ast.KeyValueExpr{Key: key, Value: args[i+1].Val}
}

// ArrayLitEx func
func (p *CodeBuilder) ArrayLitEx(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	var elts []ast.Expr
	if debugInstr {
		log.Println("ArrayLit", typ, arity, keyVal)
	}
	var t *types.Array
	var pkg = p.pkg
	typExpr := toType(pkg, typ)
retry:
	switch tt := typ.(type) {
	case *types.Named:
		t = p.getUnderlying(tt).(*types.Array)
	case *types.Array:
		t = tt
	case *types.Alias:
		typ = types.Unalias(tt)
		goto retry
	default:
		p.panicCodeErrorf(getPos(src), getEnd(src), "type %v isn't a array", typ)
	}
	if keyVal { // in keyVal mode
		if (arity & 1) != 0 {
			log.Panicln("ArrayLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		n := int(t.Len())
		args := p.stk.GetArgs(arity)
		max := p.toBoundArrayLen(args, arity, n)
		val := t.Elem()
		if n < 0 {
			t = types.NewArray(val, int64(max))
			typ = t
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			if !AssignableTo(pkg, args[i+1].Type, val) {
				src, pos, end := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in array literal", src, args[i+1].Type, val)
			}
			elts[i>>1] = p.indexElemExpr(args, i)
		}
	} else {
		args := p.stk.GetArgs(arity)
		val := t.Elem()
		if n := t.Len(); n < 0 {
			t = types.NewArray(val, int64(arity))
			typ = t
		} else if int(n) < arity {
			pos := getSrcPos(args[n].Src)
			end := getSrcEnd(args[n].Src)
			p.panicCodeErrorf(pos, end, "array index %d out of bounds [0:%d]", n, n)
		}
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			if !AssignableConv(pkg, arg.Type, val, arg) {
				src, pos, end := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in array literal", src, arg.Type, val)
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return p
}

// StructLit func
func (p *CodeBuilder) StructLit(typ types.Type, arity int, keyVal bool, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("StructLit", typ, arity, keyVal)
	}
	var t *types.Struct
	var pkg = p.pkg
	typExpr := toType(pkg, typ)
retry:
	switch tt := typ.(type) {
	case *types.Named:
		t = p.getUnderlying(tt).(*types.Struct)
	case *types.Struct:
		t = tt
	case *types.Alias:
		typ = types.Unalias(tt)
		goto retry
	default:
		p.panicCodeErrorf(getPos(src), getEnd(src), "type %v isn't a struct", typ)
	}
	var elts []ast.Expr
	var n = t.NumFields()
	var args = p.stk.GetArgs(arity)
	if keyVal {
		if (arity & 1) != 0 {
			log.Panicln("StructLit: invalid arity, can't be odd in keyVal mode -", arity)
		}
		elts = make([]ast.Expr, arity>>1)
		for i := 0; i < arity; i += 2 {
			idx := p.toIntVal(args[i], "field which must be non-negative integer constant")
			if idx >= n {
				panic("invalid struct field index")
			}
			elt := t.Field(idx)
			eltTy, eltName := elt.Type(), elt.Name()
			if !AssignableTo(pkg, args[i+1].Type, eltTy) {
				src, pos, end := p.loadExpr(args[i+1].Src)
				p.panicCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in value of field %s",
					src, args[i+1].Type, eltTy, eltName)
			}
			elts[i>>1] = &ast.KeyValueExpr{Key: ident(eltName), Value: args[i+1].Val}
		}
	} else if arity != n {
		if arity != 0 {
			fewOrMany := "few"
			if arity > n {
				fewOrMany = "many"
			}
			pos := getSrcPos(args[arity-1].Src)
			end := getSrcEnd(args[arity-1].Src)
			p.panicCodeErrorf(pos, end, "too %s values in %v{...}", fewOrMany, typ)
		}
	} else {
		elts = make([]ast.Expr, arity)
		for i, arg := range args {
			elts[i] = arg.Val
			eltTy := t.Field(i).Type()
			if !AssignableTo(pkg, arg.Type, eltTy) {
				src, pos, end := p.loadExpr(arg.Src)
				p.panicCodeErrorf(
					pos, end, "cannot use %s (type %v) as type %v in value of field %s",
					src, arg.Type, eltTy, t.Field(i).Name())
			}
		}
	}
	p.stk.Ret(arity, &internal.Elem{
		Type: typ, Val: &ast.CompositeLit{Type: typExpr, Elts: elts}, Src: getSrc(src),
	})
	return p
}

// Slice func
func (p *CodeBuilder) Slice(slice3 bool, src ...ast.Node) *CodeBuilder { // a[i:j:k]
	if debugInstr {
		log.Println("Slice", slice3)
	}
	n := 3
	if slice3 {
		n++
	}
	srcExpr := getSrc(src)
	args := p.stk.GetArgs(n)
	x := args[0]
	typ := x.Type
	switch t := typ.(type) {
	case *types.Slice:
		// nothing to do
	case *types.Basic:
		if t.Kind() == types.String || t.Kind() == types.UntypedString {
			if slice3 {
				code, pos, end := p.loadExpr(srcExpr)
				p.panicCodeErrorf(pos, end, "invalid operation %s (3-index slice of string)", code)
			}
		} else {
			code, pos, end := p.loadExpr(x.Src)
			p.panicCodeErrorf(pos, end, "cannot slice %s (type %v)", code, typ)
		}
	case *types.Array:
		typ = types.NewSlice(t.Elem())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			typ = types.NewSlice(tt.Elem())
		} else {
			code, pos, end := p.loadExpr(x.Src)
			p.panicCodeErrorf(pos, end, "cannot slice %s (type %v)", code, typ)
		}
	}
	var exprMax ast.Expr
	if slice3 {
		exprMax = args[3].Val
	}
	// TODO: check type
	elem := &internal.Elem{
		Val: &ast.SliceExpr{
			X: x.Val, Low: args[1].Val, High: args[2].Val, Max: exprMax, Slice3: slice3,
		},
		Type: typ, Src: srcExpr,
	}
	p.stk.Ret(n, elem)
	return p
}

// Index func:
//   - a[i]
//   - fn[T1, T2, ..., Tn]
//   - G[T1, T2, ..., Tn]
func (p *CodeBuilder) Index(nidx int, lhs int, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Index", nidx, lhs)
	}
	args := p.stk.GetArgs(nidx + 1)
	if nidx > 0 {
		if _, ok := args[1].Type.(*TypeType); ok {
			return p.instantiate(nidx, args, src...)
		}
	}
	if nidx != 1 {
		panic("Index doesn't support a[i, j...] yet")
	}
	srcExpr := getSrc(src)
	typs, ivKind := p.getIdxValTypes(args[0].Type, false, srcExpr)
	var tyRet types.Type
	if lhs == 2 { // elem, ok = a[key]
		if ivKind == ivFalse {
			pos := getSrcPos(srcExpr)
			end := getSrcEnd(srcExpr)
			p.panicCodeErrorf(pos, end, "assignment mismatch: 2 variables but 1 values")
		}
		pkg := p.pkg
		tyRet = types.NewTuple(
			pkg.NewParam(token.NoPos, "", typs[1]),
			pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
	} else { // elem = a[key]
		tyRet = typs[1]
	}
	argVal := args[0].Val
	if ivKind == ivMapStringAny {
		argVal = p.emitMapStringAnyAssert(argVal)
	}
	elem := &internal.Elem{
		Val: &ast.IndexExpr{X: argVal, Index: args[1].Val}, Type: tyRet, Src: srcExpr,
	}
	// TODO(xsw): check index type
	p.stk.Ret(2, elem)
	return p
}

// Star func
func (p *CodeBuilder) Star(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Star")
	}
	arg := p.stk.Get(-1)
	ret := &internal.Elem{Val: &ast.StarExpr{X: arg.Val}, Src: getSrc(src)}
	argType := arg.Type
retry:
	switch t := argType.(type) {
	case *TypeType:
		ret.Type = t.Pointer()
	case *types.Pointer:
		ret.Type = t.Elem()
	case *types.Named:
		argType = p.getUnderlying(t)
		goto retry
	default:
		code, pos, end := p.loadExpr(arg.Src)
		p.panicCodeErrorf(pos, end, "invalid indirect of %s (type %v)", code, t)
	}
	p.stk.Ret(1, ret)
	return p
}

// Elem func
func (p *CodeBuilder) Elem(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Elem")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		code, pos, end := p.loadExpr(arg.Src)
		p.panicCodeErrorf(pos, end, "invalid indirect of %s (type %v)", code, arg.Type)
	}
	p.stk.Ret(1, &internal.Elem{Val: &ast.StarExpr{X: arg.Val}, Type: t.Elem(), Src: getSrc(src)})
	return p
}

// ElemRef func
func (p *CodeBuilder) ElemRef(src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("ElemRef")
	}
	arg := p.stk.Get(-1)
	t, ok := arg.Type.(*types.Pointer)
	if !ok {
		code, pos, end := p.loadExpr(arg.Src)
		p.panicCodeErrorf(pos, end, "invalid indirect of %s (type %v)", code, arg.Type)
	}
	p.stk.Ret(1, &internal.Elem{
		Val: &ast.StarExpr{X: arg.Val}, Type: &refType{typ: t.Elem()}, Src: getSrc(src),
	})
	return p
}

func (p *CodeBuilder) doVarRef(ref any, src ast.Node, allowDebug bool) *CodeBuilder {
	if ref == nil {
		if allowDebug && debugInstr {
			log.Println("VarRef _")
		}
		p.stk.Push(&internal.Elem{
			Val: underscore, // _
		})
	} else {
		if v, ok := ref.(string); ok {
			_, ref = p.Scope().LookupParent(v, token.NoPos)
		}
		if v, ok := ref.(*types.Var); ok {
			if allowDebug && debugInstr {
				log.Println("VarRef", v.Name(), v.Type())
			}
			fn := p.current.fn
			if fn != nil && fn.isInline() { // is in an inline call
				key := closureParamInst{fn, v}
				if arg, ok := p.paramInsts[key]; ok { // replace param with arg
					v = arg
				}
			}
			p.stk.Push(&internal.Elem{
				Val: toObjectExpr(p.pkg, v), Type: &refType{typ: v.Type()}, Src: src,
			})
		} else {
			code, pos, end := p.loadExpr(src)
			p.panicCodeErrorf(pos, end, "%s is not a variable", code)
		}
	}
	return p
}

// NewAutoVar func
func (p *CodeBuilder) NewAutoVar(pos, end token.Pos, name string, pv **types.Var) *CodeBuilder {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ident(name)}}
	decl := &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
	stmt := &ast.DeclStmt{
		Decl: decl,
	}
	if debugInstr {
		log.Println("NewAutoVar", name)
	}
	p.emitStmt(stmt)
	typ := &unboundType{ptypes: []*ast.Expr{&spec.Type}}
	*pv = types.NewVar(pos, p.pkg.Types, name, typ)
	if old := p.current.scope.Insert(*pv); old != nil {
		oldPos := p.fset.Position(old.Pos())
		p.panicCodeErrorf(
			pos, end, "%s redeclared in this block\n\tprevious declaration at %v", name, oldPos)
	}
	return p
}

func methodToFuncSig(pkg *Package, o types.Object, fn *Element) *types.Signature {
	sig := o.Type().(*types.Signature)
	recv := sig.Recv()
	if recv == nil { // special signature
		fn.Val = toObjectExpr(pkg, o)
		return sig
	}

	sel := fn.Val.(*ast.SelectorExpr)
	sel.Sel = ident(o.Name())
	sel.X = &ast.ParenExpr{X: sel.X}
	return toFuncSig(sig, recv)
}

func (p *CodeBuilder) methodSigOf(typ types.Type, flag MemberFlag, arg, ret *Element) (types.Type, bool) {
	if flag != memberFlagMethodToFunc {
		return methodCallSig(typ), true
	}

	sig := typ.(*types.Signature)
	if t, ok := CheckFuncEx(sig); ok {
		switch ext := t.(type) {
		case *TyStaticMethod:
			return p.funcExSigOf(ext.Func, ret)
		case *TyTemplateRecvMethod:
			return p.funcExSigOf(ext.Func, ret)
		}
		// TODO: We should take `methodSigOf` more seriously
		return typ, true
	}

	sel := ret.Val.(*ast.SelectorExpr)
	at := arg.Type.(*TypeType).typ
	recv := sig.Recv().Type()
	_, isPtr := recv.(*types.Pointer) // recv is a pointer
	if t, ok := at.(*types.Pointer); ok {
		if !isPtr {
			if _, ok := recv.Underlying().(*types.Interface); !ok { // and recv isn't a interface
				log.Panicf("recv of method %v.%s isn't a pointer\n", t.Elem(), sel.Sel.Name)
			}
		}
	} else if isPtr { // use *T
		at = types.NewPointer(at)
		sel.X = &ast.StarExpr{X: sel.X}
	}
	sel.X = &ast.ParenExpr{X: sel.X}

	return toFuncSig(sig, types.NewVar(token.NoPos, nil, "", at)), true
}

func boundTypeParams(p *Package, fn *Element, sig *types.Signature, args []*Element, flags InstrFlags) (*Element, *types.Signature, []*Element, error) {
	if debugMatch {
		log.Println("boundTypeParams:", goxdbg.Format(p.Fset, fn.Val), "sig:", sig, "args:", len(args), "flags:", flags)
	}
	params := sig.TypeParams()
	if n := params.Len(); n > 0 {
		from := 0
		if (flags & instrFlagXGotFunc) != 0 {
			from = 1
		}
		targs := make([]types.Type, n)
		for i := 0; i < n; i++ {
			arg := args[from+i]
			t, ok := arg.Type.(*TypeType)
			if !ok {
				src, pos, end := p.cb.loadExpr(arg.Src)
				err := p.cb.newCodeErrorf(pos, end, "%s (type %v) is not a type", src, arg.Type)
				return fn, sig, args, err
			}
			targs[i] = t.typ
		}
		ret, err := types.Instantiate(p.cb.ctxt, sig, targs, true)
		if err != nil {
			return fn, sig, args, err
		}
		indices := make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			indices[i] = args[from+i].Val
		}
		fn = &Element{Val: &ast.IndexListExpr{X: fn.Val, Indices: indices}, Type: ret, Src: fn.Src}
		sig = ret.(*types.Signature)
		if from > 0 {
			args = append([]*Element{args[0]}, args[n+1:]...)
		} else {
			args = args[n:]
		}
	}
	return fn, sig, args, nil
}

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	typ := args[0].Type
	var tt bool
	if t, ok := typ.(*TypeType); ok {
		typ = t.Type()
		tt = true
	}
	p.ensureLoaded(typ)
	srcExpr := getSrc(src)
	if !isGenericType(typ) {
		pos := getSrcPos(srcExpr)
		end := getSrcEnd(srcExpr)
		if tt {
			p.panicCodeErrorf(pos, end, "%v is not a generic type", typ)
		} else {
			p.panicCodeErrorf(pos, end, "invalid operation: cannot index %v (value of type %v)", types.ExprString(args[0].Val), typ)
		}
	}
	targs := make([]types.Type, nidx)
	indices := make([]ast.Expr, nidx)
	for i := 0; i < nidx; i++ {
		targs[i] = args[i+1].Type.(*TypeType).Type()
		p.ensureLoaded(targs[i])
		indices[i] = args[i+1].Val
	}
	var tyRet types.Type
	var err error
	if tt {
		tyRet, err = types.Instantiate(p.ctxt, typ, targs, true)
		if err == nil {
			tyRet = NewTypeType(tyRet)
		}
	} else {
		sig := typ.(*types.Signature)
		if nidx >= sig.TypeParams().Len() {
			tyRet, err = types.Instantiate(p.ctxt, typ, targs, true)
		} else {
			tyRet = newInferFuncType(p.pkg, args[0], sig, targs, srcExpr)
		}
	}
	if err != nil {
		pos := getSrcPos(srcExpr)
		end := getSrcEnd(srcExpr)
		p.panicCodeErrorf(pos, end, "%v", err)
	}
	if debugMatch {
		log.Println("==> InferType", tyRet)
	}
	elem := &internal.Elem{
		Type: tyRet, Src: srcExpr,
	}
	x := args[0].Val
	if nidx == 1 {
		elem.Val = &ast.IndexExpr{X: x, Index: indices[0]}
	} else {
		elem.Val = &ast.IndexListExpr{X: x, Indices: indices}
	}
	p.stk.Ret(nidx+1, elem)
	return p
}

func instanceInferFunc(pkg *Package, arg *internal.Elem, tsig *inferFuncType, sig *types.Signature) error {
	args := paramsToArgs(sig)
	targs, _, err := inferFunc(tsig.pkg, tsig.fn, tsig.typ, tsig.targs, args, 0)
	if err != nil {
		return err
	}
	arg.Type = sig
	index := make([]ast.Expr, len(targs))
	for i, a := range targs {
		index[i] = toType(pkg, a)
	}
	var x ast.Expr
	switch v := arg.Val.(type) {
	case *ast.IndexExpr:
		x = v.X
	case *ast.IndexListExpr:
		x = v.X
	}
	arg.Val = &ast.IndexListExpr{
		Indices: index,
		X:       x,
	}
	return nil
}

func instanceFunc(pkg *Package, arg *internal.Elem, tsig *types.Signature, sig *types.Signature) error {
	args := paramsToArgs(sig)
	targs, _, err := inferFunc(pkg, &internal.Elem{Val: arg.Val}, tsig, nil, args, 0)
	if err != nil {
		return err
	}
	arg.Type = sig
	if len(targs) == 1 {
		arg.Val = &ast.IndexExpr{
			Index: toType(pkg, targs[0]),
			X:     arg.Val,
		}
	} else {
		index := make([]ast.Expr, len(targs))
		for i, a := range targs {
			index[i] = toType(pkg, a)
		}
		arg.Val = &ast.IndexListExpr{
			Indices: index,
			X:       arg.Val,
		}
	}
	return nil
}

func checkInferArgs(pkg *Package, fn *internal.Elem, sig *types.Signature, args []*internal.Elem, flags InstrFlags) ([]*internal.Elem, error) {
	nargs := len(args)
	nreq := sig.Params().Len()
	if sig.Variadic() {
		if nargs < nreq-1 {
			caller := types.ExprString(fn.Val)
			return nil, fmt.Errorf(
				"not enough arguments in call to %s\n\thave (%v)\n\twant (%v)", caller, getTypes(args), getParamsTypes(sig.Params(), true))
		}
		if flags&InstrFlagEllipsis != 0 {
			return args, nil
		}
		var typ types.Type
		if nargs < nreq {
			typ = sig.Params().At(nreq - 1).Type()
			elem := typ.(*types.Slice).Elem()
			if t, ok := elem.(*types.TypeParam); ok {
				return nil, fmt.Errorf("cannot infer %v (%v)", elem, pkg.cb.fset.Position(t.Obj().Pos()))
			}
		} else {
			typ = types.NewSlice(types.Default(args[nreq-1].Type))
		}
		res := make([]*internal.Elem, nreq)
		for i := 0; i < nreq-1; i++ {
			res[i] = args[i]
		}
		res[nreq-1] = &internal.Elem{Type: typ}
		return res, nil
	} else if nreq != nargs {
		fewOrMany := "not enough"
		if nargs > nreq {
			fewOrMany = "too many"
		}
		caller := types.ExprString(fn.Val)
		return nil, fmt.Errorf(
			"%s arguments in call to %s\n\thave (%v)\n\twant (%v)", fewOrMany, caller, getTypes(args), getParamsTypes(sig.Params(), false))
	}
	return args, nil
}

func getFunExpr(fn *internal.Elem) (caller string, pos, end token.Pos) {
	if fn == nil {
		return "the closure call", token.NoPos, token.NoPos
	}
	caller = types.ExprString(fn.Val)
	pos = getSrcPos(fn.Src)
	end = getSrcEnd(fn.Src)
	return
}

func getCaller(expr *internal.Elem) string {
	if ce, ok := expr.Val.(*ast.CallExpr); ok {
		return types.ExprString(ce.Fun)
	}
	return "the function call"
}

// NewAndInit creates variables with specified `typ` (can be nil) and `names`, and
// initializes them by `fn` (can be nil). When `fn` is nil (no initialization),
// `typ` must not be nil. When names is empty, creates an embedded field.
func (p *ClassDefs) NewAndInit(fn F, pos token.Pos, typ types.Type, names ...string) {
	pkg := p.pkg
	pkgTypes := pkg.Types
	embed := len(names) == 0
	if fn == nil { // no initialization
		if embed {
			p.addFld(0, embedName(typ), typ, true)
		} else {
			for i, name := range names {
				p.addFld(i, name, typ, false)
			}
		}
		return
	}
	recv := p.recv
	cb := p.cb
	if cb == nil {
		result := types.NewTuple(types.NewParam(token.NoPos, pkgTypes, "", recv.Type()))
		cb = pkg.NewFunc(recv, "XGo_Init", nil, result, false).BodyStart(pkg)
		p.cb = cb
	}
	scope := cb.current.scope
	recvName := ident(recv.Name())
	if embed {
		names = []string{embedName(typ)}
	}
	if typ == nil {
		cb.DefineVarStart(pos, names...)
		decl := cb.valDecl
		stmt := cb.current.stmts[decl.at].(*ast.AssignStmt)
		callInitExpr(cb, fn)

		for i, name := range names {
			o := scope.Lookup(name)
			p.addFld(i, name, o.Type(), embed)
			stmt.Lhs[i] = &ast.SelectorExpr{
				X:   recvName,
				Sel: stmt.Lhs[i].(*ast.Ident),
			}
		}
		stmt.Tok = token.ASSIGN
	} else {
		cb.NewVarStart(typ, names...)
		decl := cb.valDecl
		pstmt := &cb.current.stmts[decl.at]
		spec := (*pstmt).(*ast.DeclStmt).Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
		callInitExpr(cb, fn)

		lhs := make([]ast.Expr, len(names))
		for i, name := range names {
			p.addFld(i, name, typ, embed)
			lhs[i] = &ast.SelectorExpr{
				X:   recvName,
				Sel: spec.Names[i],
			}
		}
		*pstmt = &ast.AssignStmt{
			Lhs: lhs,
			Tok: token.ASSIGN,
			Rhs: spec.Values,
		}
	}
}

// ----------------------------------------------------------------------------

type fileDecls struct {
	goDecls []ast.Decl
}

type (
	funcDecl  = ast.FuncDecl
	typeDecl  = ast.GenDecl
	valDecl   = ast.GenDecl
	valueSpec = ast.ValueSpec
)

func asValueSpec(spec ast.Spec) *valueSpec {
	return spec.(*valueSpec)
}

func (p *fileDecls) appendFuncDecl(decl *funcDecl) {
	p.goDecls = append(p.goDecls, decl)
}

func (p *fileDecls) appendValDecl(decl *valDecl) {
	p.goDecls = append(p.goDecls, decl)
}

func startValDeclStmtAt(cb *CodeBuilder, decl *valDecl) int {
	return cb.startStmtAt(&ast.DeclStmt{Decl: decl})
}

func (p *fileDecls) appendTypeDecl(decl *typeDecl) {
	p.goDecls = append(p.goDecls, decl)
}

// ----------------------------------------------------------------------------

var (
	identAppend = ident("append")
	identLen    = ident("len")
	identCap    = ident("cap")
	identNew    = ident("new")
	identMake   = ident("make")
	identIota   = ident("iota")
)

func newIotaExpr(_ int) *ast.Ident {
	return identIota
}

func newAppendStringExpr(args []*internal.Elem) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:      identAppend,
		Args:     []ast.Expr{args[0].Val, args[1].Val},
		Ellipsis: 1,
	}
}

func newLenExpr(args []*internal.Elem) *ast.CallExpr {
	return &ast.CallExpr{Fun: identLen, Args: []ast.Expr{args[0].Val}}
}

func newCapExpr(args []*internal.Elem) *ast.CallExpr {
	return &ast.CallExpr{Fun: identCap, Args: []ast.Expr{args[0].Val}}
}

func newNewExpr(args []*internal.Elem) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  identNew,
		Args: []ast.Expr{args[0].Val},
	}
}

func newMakeExpr(args []*internal.Elem) *ast.CallExpr {
	argsExpr := make([]target.Expr, n)
	for i, arg := range args {
		argsExpr[i] = arg.Val
	}
	return &ast.CallExpr{
		Fun:  identMake,
		Args: argsExpr,
	}
}

func newSizeofExpr(pkg *Package, args []*internal.Elem) *ast.CallExpr {
	fn := toObjectExpr(pkg, unsafeRef("Sizeof"))
	return &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}}
}

func newAlignofExpr(pkg *Package, args []*internal.Elem) *ast.CallExpr {
	fn := toObjectExpr(pkg, unsafeRef("Alignof"))
	return &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}}
}

func newOffsetofExpr(pkg *Package, args []*internal.Elem) *ast.CallExpr {
	fn := toObjectExpr(pkg, unsafeRef("Offsetof"))
	return &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}}
}

func newUnsafeAddExpr(pkg *Package, args []*internal.Elem) *ast.CallExpr {
	fn := toObjectExpr(pkg, unsafeRef("Sizeof")).(*ast.SelectorExpr)
	fn.Sel.Name = "Add"
	return &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val, args[1].Val}}
}

func newUnsafeDataExpr(p unsafeDataInstr, pkg *Package, args []*internal.Elem) *ast.CallExpr {
	fn := toObjectExpr(pkg, unsafeRef("Sizeof")).(*ast.SelectorExpr)
	fn.Sel.Name = p.name
	var exprs []ast.Expr
	if p.args == 2 {
		exprs = []ast.Expr{args[0].Val, args[1].Val}
	} else {
		exprs = []ast.Expr{args[0].Val}
	}
	return &ast.CallExpr{Fun: fn, Args: exprs}
}

func newRecvExpr(args []*internal.Elem) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: token.ARROW, X: args[0].Val}
}

func newAddrExpr(args []*internal.Elem) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: token.AND, X: args[0].Val}
}

func zeroCompositeLit(p *Package, typ types.Type) *ast.CompositeLit {
	ret := &ast.CompositeLit{}
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound == nil {
			t.ptypes = append(t.ptypes, &ret.Type)
		} else {
			typ = t.tBound
			typ0 = typ
			ret.Type = toType(p, typ)
		}
	default:
		ret.Type = toType(p, typ)
	}
	return ret
}

func newFuncLit(pkg *Package, t *types.Signature, body *ast.BlockStmt) *ast.FuncLit {
	return &ast.FuncLit{Type: toFuncType(pkg, t), Body: body}
}

func newCommentedNodes(p *Package, f *ast.File) *printer.CommentedNodes {
	return &printer.CommentedNodes{
		Node:           f,
		CommentedStmts: p.commentedStmts,
	}
}

// ----------------------------------------------------------------------------

func newIncDecStmt(x ast.Expr, tok token.Token) *ast.IncDecStmt {
	return &ast.IncDecStmt{X: x, Tok: tok}
}

func newAssignOpStmt(tok token.Token, args []*internal.Elem) *ast.AssignStmt {
	return &ast.AssignStmt{
		Tok: tok,
		Lhs: []ast.Expr{args[0].Val},
		Rhs: []ast.Expr{args[1].Val},
	}
}

func emitAssignStmt(cb *CodeBuilder, stmt *ast.AssignStmt) {
	cb.emitStmt(stmt)
}

func commitAssignStmt(cb *CodeBuilder, p *ValueDecl) {
	cb.commitStmt(p.at)
}

func emitTypeDeclStmt(cb *CodeBuilder, decl *ast.GenDecl) {
	cb.emitStmt(&ast.DeclStmt{Decl: decl})
}

func emitGoStmt(cb *CodeBuilder, call ast.Expr) {
	cb.emitStmt(&ast.GoStmt{Call: call})
}

func emitDeferStmt(cb *CodeBuilder, call ast.Expr) {
	cb.emitStmt(&ast.DeferStmt{Call: call})
}

func emitSendStmt(cb *CodeBuilder, ch, val ast.Expr) {
	cb.emitStmt(&ast.SendStmt{Chan: ch, Value: val})
}

func emitGotoStmt(cb *CodeBuilder, name string) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.GOTO, Label: ident(name)})
}

func emitReturnStmt(cb *CodeBuilder, pos token.Pos, rets ...ast.Expr) {
	cb.emitStmt(&ast.ReturnStmt{Return: pos, Results: rets})
}

func emitIfStmt(cb *CodeBuilder, p *ifStmt, el ast.Stmt) {
	cb.emitStmt(&ast.IfStmt{Init: p.init, Cond: p.cond, Body: p.body, Else: el})
}

func emitSWitchStmt(cb *CodeBuilder, p *switchStmt, stmts []ast.Stmt) {
	body := &ast.BlockStmt{List: stmts}
	cb.emitStmt(&ast.SwitchStmt{Init: p.init, Tag: checkParenExpr(p.tag.Val), Body: body})
}

func emitFullthrough(cb *CodeBuilder) {
	cb.emitStmt(&ast.BranchStmt{Tok: token.FALLTHROUGH})
}

func emitCaseClause(cb *CodeBuilder, p *caseStmt, body []ast.Stmt) {
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

func emitSelectStmt(cb *CodeBuilder, stmts []ast.Stmt) {
	cb.emitStmt(&ast.SelectStmt{Body: &ast.BlockStmt{List: stmts}})
}

func emitCommClause(cb *CodeBuilder, p *commCase, body []ast.Stmt) {
	cb.emitStmt(&ast.CommClause{Comm: p.comm, Body: body})
}

func emitTypeSwitchStmt(cb *CodeBuilder, p *typeSwitchStmt, stmts []ast.Stmt) {
	body := &ast.BlockStmt{List: stmts}
	var assign ast.Stmt
	x := &ast.TypeAssertExpr{X: p.x}
	if p.name != "" {
		assign = &ast.AssignStmt{
			Tok: token.DEFINE,
			Lhs: []ast.Expr{ident(p.name)},
			Rhs: []ast.Expr{x},
		}
	} else {
		assign = &ast.ExprStmt{X: x}
	}
	cb.emitStmt(&ast.TypeSwitchStmt{Init: p.init, Assign: assign, Body: body})
}

func emitTypeCaseClause(cb *CodeBuilder, p *typeCaseStmt, body []ast.Stmt) {
	cb.emitStmt(&ast.CaseClause{List: p.list, Body: body})
}

func emitForRangeStmt(cb *CodeBuilder, p *forRangeStmt, stmts []ast.Stmt) {
	if n := p.udt; n == 0 {
		if p.enumName != "" {
			p.stmt.X = &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: p.stmt.X, Sel: ident(p.enumName)},
			}
		}
		p.stmt.Body = p.handleFor(&ast.BlockStmt{List: stmts}, 1)
		cb.emitStmt(p.stmt)
	} else if n > 0 {
		cb.stk.Push(p.x)
		cb.MemberVal(p.enumName, 0).Call(0)
		callEnum := cb.stk.Pop().Val
		/*
			for _xgo_it := X.XGo_Enum();; {
				var _xgo_ok bool
				k, v, _xgo_ok = _xgo_it.Next()
				if !_xgo_ok {
					break
				}
				...
			}
		*/
		lhs := make([]ast.Expr, n)
		lhs[0] = p.stmt.Key
		lhs[1] = p.stmt.Value
		lhs[n-1] = identXgoOk
		if lhs[0] == nil { // bugfix: for range udt { ... }
			lhs[0] = underscore
			if p.stmt.Tok == token.ILLEGAL {
				p.stmt.Tok = token.ASSIGN
			}
		} else {
			// for _ = range udt { ... }
			if ki, ok := lhs[0].(*ast.Ident); ok && ki.Name == "_" {
				if n == 2 {
					p.stmt.Tok = token.ASSIGN
				} else {
					// for _ , _ = range udt { ... }
					if vi, ok := lhs[1].(*ast.Ident); ok && vi.Name == "_" {
						p.stmt.Tok = token.ASSIGN
					}
				}
			}
		}
		body := make([]ast.Stmt, len(stmts)+3)
		body[0] = stmtXGoOkDecl
		body[1] = &ast.AssignStmt{Lhs: lhs, Tok: p.stmt.Tok, Rhs: exprIterNext}
		body[2] = stmtBreakIfNotXGoOk
		copy(body[3:], stmts)
		stmt := &ast.ForStmt{
			Init: &ast.AssignStmt{
				Lhs: []ast.Expr{identXgoIt},
				Tok: token.DEFINE,
				Rhs: []ast.Expr{callEnum},
			},
			Body: p.handleFor(&ast.BlockStmt{List: body}, 2),
		}
		cb.emitStmt(stmt)
	} else {
		/*
			X.XGo_Enum(func(k K, v V) {
				...
			})
		*/
		if flows != 0 {
			cb.panicCodeError(p.stmt.For, p.stmt.For, cantUseFlows)
		}
		n = -n
		def := p.stmt.Tok == token.DEFINE
		args := make([]*ast.Field, n)
		if def {
			args[0] = &ast.Field{
				Names: []*ast.Ident{p.stmt.Key.(*ast.Ident)},
				Type:  toType(cb.pkg, p.kvt[0]),
			}
			if n > 1 {
				args[1] = &ast.Field{
					Names: []*ast.Ident{p.stmt.Value.(*ast.Ident)},
					Type:  toType(cb.pkg, p.kvt[1]),
				}
			}
		} else {
			panic("TODO: for range udt assign")
		}
		stmt := &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: p.stmt.X, Sel: ident(p.enumName)},
				Args: []ast.Expr{
					&ast.FuncLit{
						Type: &ast.FuncType{Params: &ast.FieldList{List: args}},
						Body: p.handleFor(&ast.BlockStmt{List: stmts}, -1),
					},
				},
			},
		}
		cb.emitStmt(stmt)
	}
}

var (
	identXgoOk = ident("_xgo_ok")
	identXgoIt = ident("_xgo_it")
)

var (
	stmtXGoOkDecl = &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{Names: []*ast.Ident{identXgoOk}, Type: ident("bool")},
			},
		},
	}
	stmtBreakIfNotXGoOk = &ast.IfStmt{
		Cond: &ast.UnaryExpr{Op: token.NOT, X: identXgoOk},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	}
	exprIterNext = []ast.Expr{
		&ast.CallExpr{
			Fun: &ast.SelectorExpr{X: identXgoIt, Sel: ident("Next")},
		},
	}
)

// ----------------------------------------------------------------------------
