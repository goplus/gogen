/*
 Copyright 2022 The GoPlus Authors (goplus.org)
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
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"strings"
	_ "unsafe"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/goxdbg"
)

// ----------------------------------------------------------------------------

type TypeParam = types.TypeParam
type Union = types.Union
type Term = types.Term
type TypeParamList = types.TypeParamList

type inferFuncType struct {
	pkg   *Package
	fn    *internal.Elem
	typ   *types.Signature
	targs []types.Type
	src   ast.Node
}

func newInferFuncType(pkg *Package, fn *internal.Elem, typ *types.Signature, targs []types.Type, src ast.Node) *inferFuncType {
	return &inferFuncType{pkg: pkg, fn: fn, typ: typ, targs: targs, src: src}
}

func (p *inferFuncType) Type() types.Type {
	fatal("infer of type")
	return p.typ
}

func (p *inferFuncType) Underlying() types.Type {
	fatal("infer of type")
	return nil
}

func (p *inferFuncType) String() string {
	return fmt.Sprintf("inferFuncType{typ: %v, targs: %v}", p.typ, p.targs)
}

func (p *inferFuncType) Instance() *types.Signature {
	tyRet, err := inferFuncTargs(p.pkg, p.fn, p.typ, p.targs)
	if err != nil {
		pos := getSrcPos(p.src)
		p.pkg.cb.panicCodeErrorf(pos, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func (p *inferFuncType) InstanceWithArgs(args []*internal.Elem, flags InstrFlags) *types.Signature {
	tyRet, err := InferFunc(p.pkg, p.fn, p.typ, p.targs, args, flags)
	if err != nil {
		pos := getSrcPos(p.src)
		p.pkg.cb.panicCodeErrorf(pos, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func isGenericType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		return t.Obj() != nil && t.TypeArgs() == nil && t.TypeParams() != nil
	case *types.Signature:
		return t.TypeParams() != nil
	}
	return false
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
		if tt {
			p.panicCodeErrorf(pos, "%v is not a generic type", typ)
		} else {
			p.panicCodeErrorf(pos, "invalid operation: cannot index %v (value of type %v)", types.ExprString(args[0].Val), typ)
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
		p.panicCodeErrorf(pos, "%v", err)
	}
	if debugMatch {
		log.Println("==> InferType", tyRet)
	}
	elem := &internal.Elem{
		Type: tyRet, Src: srcExpr,
	}
	if nidx == 1 {
		elem.Val = &ast.IndexExpr{X: args[0].Val, Index: indices[0]}
	} else {
		elem.Val = &ast.IndexListExpr{X: args[0].Val, Indices: indices}
	}
	p.stk.Ret(nidx+1, elem)
	return p
}

type typesContext = types.Context

func newTypesContext() *typesContext {
	return types.NewContext()
}

func toRecvType(pkg *Package, typ types.Type) ast.Expr {
	var star bool
	if t, ok := typ.(*types.Pointer); ok {
		typ = t.Elem()
		star = true
	}
	t, ok := typ.(*types.Named)
	if !ok {
		panic("unexpected: recv type must types.Named")
	}
	expr := toObjectExpr(pkg, t.Obj())
	if tparams := t.TypeParams(); tparams != nil {
		n := tparams.Len()
		indices := make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			indices[i] = toObjectExpr(pkg, tparams.At(i).Obj())
		}
		if n == 1 {
			expr = &ast.IndexExpr{
				X:     expr,
				Index: indices[0],
			}
		} else {
			expr = &ast.IndexListExpr{
				X:       expr,
				Indices: indices,
			}
		}
	}
	if star {
		expr = &ast.StarExpr{X: expr}
	}
	return expr
}

func toNamedType(pkg *Package, t *types.Named) ast.Expr {
	expr := toObjectExpr(pkg, t.Obj())
	if targs := t.TypeArgs(); targs != nil {
		n := targs.Len()
		indices := make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			indices[i] = toType(pkg, targs.At(i))
		}
		if n == 1 {
			expr = &ast.IndexExpr{
				X:     expr,
				Index: indices[0],
			}
		} else {
			expr = &ast.IndexListExpr{
				X:       expr,
				Indices: indices,
			}
		}
	}
	return expr
}

type operandMode byte

//type builtinId int

type operand struct {
	mode operandMode
	expr ast.Expr
	typ  types.Type
	val  constant.Value
	//id   builtinId
}

const (
	invalid   operandMode = iota // operand is invalid
	novalue                      // operand represents no value (result of a function call w/o result)
	builtin                      // operand is a built-in function
	typexpr                      // operand is a type
	constant_                    // operand is a constant; the operand's typ is a Basic type
	variable                     // operand is an addressable variable
	mapindex                     // operand is a map index expression (acts like a variable on lhs, commaok on rhs of an assignment)
	value                        // operand is a computed value
	commaok                      // like value, but operand may be used in a comma,ok expression
	commaerr                     // like commaok, but second value is error, not boolean
	cgofunc                      // operand is a cgo function
)

type positioner interface {
	Pos() token.Pos
}

//go:linkname checker_infer go/types.(*Checker).infer
func checker_infer(check *types.Checker, posn positioner, tparams []*types.TypeParam, targs []types.Type, params *types.Tuple, args []*operand) (result []types.Type)

func infer(pkg *Package, posn positioner, tparams []*types.TypeParam, targs []types.Type, params *types.Tuple, args []*operand) (result []types.Type, err error) {
	conf := &types.Config{
		Error: func(e error) {
			err = e
			if terr, ok := e.(types.Error); ok {
				err = fmt.Errorf("%s", terr.Msg)
			}
		},
	}
	checker := types.NewChecker(conf, pkg.Fset, pkg.Types, nil)
	result = checker_infer(checker, posn, tparams, targs, params, args)
	return
}

func getParamsTypes(tuple *types.Tuple, variadic bool) string {
	n := tuple.Len()
	if n == 0 {
		return ""
	}
	typs := make([]string, n)
	for i := 0; i < n; i++ {
		typs[i] = tuple.At(i).Type().String()
	}
	if variadic {
		typs[n-1] = "..." + tuple.At(n-1).Type().(*types.Slice).Elem().String()
	}
	return strings.Join(typs, ", ")
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

// InferFunc attempts to infer and instantiates the generic function instantiation with given func and args.
func InferFunc(pkg *Package, fn *Element, sig *types.Signature, targs []types.Type, args []*Element, flags InstrFlags) (types.Type, error) {
	_, typ, err := inferFunc(pkg, fn, sig, targs, args, flags)
	return typ, err
}

func inferFuncTargs(pkg *Package, fn *internal.Elem, sig *types.Signature, targs []types.Type) (types.Type, error) {
	tp := sig.TypeParams()
	n := tp.Len()
	tparams := make([]*types.TypeParam, n)
	for i := 0; i < n; i++ {
		tparams[i] = tp.At(i)
	}
	targs, err := infer(pkg, fn.Val, tparams, targs, nil, nil)
	if err != nil {
		return nil, err
	}
	return types.Instantiate(pkg.cb.ctxt, sig, targs, true)
}

func toFieldListX(pkg *Package, t *types.TypeParamList) *ast.FieldList {
	if t == nil {
		return nil
	}
	n := t.Len()
	flds := make([]*ast.Field, n)
	for i := 0; i < n; i++ {
		item := t.At(i)
		names := []*ast.Ident{ast.NewIdent(item.Obj().Name())}
		typ := toType(pkg, item.Constraint())
		flds[i] = &ast.Field{Names: names, Type: typ}
	}
	return &ast.FieldList{
		List: flds,
	}
}

func toFuncType(pkg *Package, sig *types.Signature) *ast.FuncType {
	params := toFieldList(pkg, sig.Params())
	results := toFieldList(pkg, sig.Results())
	if sig.Variadic() {
		n := len(params)
		if n == 0 {
			panic("TODO: toFuncType error")
		}
		toVariadic(params[n-1])
	}
	return &ast.FuncType{
		TypeParams: toFieldListX(pkg, sig.TypeParams()),
		Params:     &ast.FieldList{List: params},
		Results:    &ast.FieldList{List: results},
	}
}

func toUnionType(pkg *Package, t *types.Union) ast.Expr {
	var v ast.Expr
	n := t.Len()
	for i := 0; i < n; i++ {
		term := t.Term(i)
		typ := toType(pkg, term.Type())
		if term.Tilde() {
			typ = &ast.UnaryExpr{
				Op: token.TILDE,
				X:  typ,
			}
		}
		if n == 1 {
			return typ
		}
		if i == 0 {
			v = typ
		} else {
			v = &ast.BinaryExpr{
				X:  v,
				Op: token.OR,
				Y:  typ,
			}
		}
	}
	return v
}

func setTypeParams(pkg *Package, typ *types.Named, spec *ast.TypeSpec, tparams []*TypeParam) {
	typ.SetTypeParams(tparams)
	n := len(tparams)
	if n == 0 {
		spec.TypeParams = nil
		return
	}
	flds := make([]*ast.Field, n)
	for i := 0; i < n; i++ {
		item := tparams[i]
		names := []*ast.Ident{ast.NewIdent(item.Obj().Name())}
		typ := toType(pkg, item.Constraint())
		flds[i] = &ast.Field{Names: names, Type: typ}
	}
	spec.TypeParams = &ast.FieldList{List: flds}
}

func interfaceIsImplicit(t *types.Interface) bool {
	return t.IsImplicit()
}

// ----------------------------------------------------------------------------

func boundTypeParams(p *Package, fn *Element, sig *types.Signature, args []*Element, flags InstrFlags) (*Element, *types.Signature, []*Element, error) {
	if debugMatch {
		log.Println("boundTypeParams:", goxdbg.Format(p.Fset, fn.Val), "sig:", sig, "args:", len(args), "flags:", flags)
	}
	params := sig.TypeParams()
	if n := params.Len(); n > 0 {
		from := 0
		if (flags & instrFlagGoptFunc) != 0 {
			from = 1
		}
		targs := make([]types.Type, n)
		for i := 0; i < n; i++ {
			arg := args[from+i]
			t, ok := arg.Type.(*TypeType)
			if !ok {
				src, pos := p.cb.loadExpr(arg.Src)
				err := p.cb.newCodeErrorf(pos, "%s (type %v) is not a type", src, arg.Type)
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

func (p *Package) Instantiate(orig types.Type, targs []types.Type, src ...ast.Node) types.Type {
	p.cb.ensureLoaded(orig)
	for _, targ := range targs {
		p.cb.ensureLoaded(targ)
	}
	ctxt := p.cb.ctxt
	if on, ok := CheckOverloadNamed(orig); ok {
		var first error
		for _, t := range on.Types {
			ret, err := types.Instantiate(ctxt, t, targs, true)
			if err == nil {
				return ret
			}
			if first == nil {
				first = err
			}
		}
		p.cb.handleCodeError(getPos(src), first.Error())
		return types.Typ[types.Invalid]
	}
	if !isGenericType(orig) {
		p.cb.handleCodeErrorf(getPos(src), "%v is not a generic type", orig)
		return types.Typ[types.Invalid]
	}
	ret, err := types.Instantiate(ctxt, orig, targs, true)
	if err != nil {
		p.cb.handleCodeError(getPos(src), err.Error())
	}
	return ret
}

func paramsToArgs(sig *types.Signature) []*internal.Elem {
	n := sig.Params().Len()
	args := make([]*internal.Elem, n)
	for i := 0; i < n; i++ {
		p := sig.Params().At(i)
		args[i] = &internal.Elem{
			Val:  ast.NewIdent(p.Name()),
			Type: p.Type(),
		}
	}
	return args
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

// ----------------------------------------------------------------------------
