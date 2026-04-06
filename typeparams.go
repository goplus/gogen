/*
 Copyright 2022 The XGo Authors (xgo.dev)
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
	"strings"
	_ "unsafe"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/target"
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
		end := getSrcEnd(p.src)
		p.pkg.cb.panicCodeErrorf(pos, end, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func (p *inferFuncType) InstanceWithArgs(args []*internal.Elem, flags InstrFlags) *types.Signature {
	tyRet, err := InferFunc(p.pkg, p.fn, p.typ, p.targs, args, flags)
	if err != nil {
		pos := getSrcPos(p.src)
		end := getSrcEnd(p.src)
		p.pkg.cb.panicCodeErrorf(pos, end, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func isGenericType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		return t.Obj() != nil && t.TypeArgs() == nil && t.TypeParams() != nil
	case *types.Signature:
		return t.TypeParams() != nil
	case *types.Alias:
		return t.TypeParams() != nil
	}
	return false
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
	expr := toObjectTypeExpr(pkg, t.Obj())
	if tparams := t.TypeParams(); tparams != nil {
		n := tparams.Len()
		indices := make([]ast.Expr, n)
		for i := 0; i < n; i++ {
			indices[i] = toObjectTypeExpr(pkg, tparams.At(i).Obj())
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

func toTypeArgs(pkg *Package, expr ast.Expr, targs *types.TypeList) ast.Expr {
	if targs != nil {
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

func toNamedType(pkg *Package, t *types.Named) ast.Expr {
	expr := toObjectTypeExpr(pkg, t.Obj())
	return toTypeArgs(pkg, expr, t.TypeArgs())
}

func toAliasType(pkg *Package, t *types.Alias) ast.Expr {
	expr := toObjectTypeExpr(pkg, t.Obj())
	return toTypeArgs(pkg, expr, t.TypeArgs())
}

type operandMode byte

type builtinId int

type operand struct {
	mode operandMode
	expr target.Expr
	typ  types.Type
	val  constant.Value
	id   builtinId
}

type positioner interface {
	Pos() token.Pos
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
		p.cb.handleCodeError(getPos(src), getEnd(src), first.Error())
		return types.Typ[types.Invalid]
	}
	if !isGenericType(orig) {
		p.cb.handleCodeErrorf(getPos(src), getEnd(src), "%v is not a generic type", orig)
		return types.Typ[types.Invalid]
	}
	ret, err := types.Instantiate(ctxt, orig, targs, true)
	if err != nil {
		p.cb.handleCodeError(getPos(src), getEnd(src), err.Error())
	}
	return ret
}

func paramsToArgs(sig *types.Signature) []*internal.Elem {
	n := sig.Params().Len()
	args := make([]*internal.Elem, n)
	for i := 0; i < n; i++ {
		p := sig.Params().At(i)
		args[i] = &internal.Elem{
			Val:  ident(p.Name()),
			Type: p.Type(),
		}
	}
	return args
}

func inferFunc(pkg *Package, fn *internal.Elem, sig *types.Signature, targs []types.Type, args []*Element, flags InstrFlags) ([]types.Type, types.Type, error) {
	args, err := checkInferArgs(pkg, fn, sig, args, flags)
	if err != nil {
		return nil, nil, err
	}
	xlist := make([]*operand, len(args))
	tp := sig.TypeParams()
	n := tp.Len()
	tparams := make([]*types.TypeParam, n)
	for i := 0; i < n; i++ {
		tparams[i] = tp.At(i)
	}
	for i, arg := range args {
		xlist[i] = &operand{
			mode: value,
			expr: arg.Val,
			typ:  arg.Type,
			val:  arg.CVal,
		}
		tt := arg.Type
	retry:
		switch t := tt.(type) {
		case *types.Slice:
			tt = t.Elem()
			goto retry
		case *inferFuncType:
			xlist[i].typ = t.typ
			if tp := t.typ.TypeParams(); tp != nil {
				for i := 0; i < tp.Len(); i++ {
					tparams = append(tparams, tp.At(i))
				}
			}
		case *types.Signature:
			if tp := t.TypeParams(); tp != nil {
				for i := 0; i < tp.Len(); i++ {
					tparams = append(tparams, tp.At(i))
				}
			}
		}
	}
	targs, err = infer(pkg, fn.Val, tparams, targs, sig.Params(), xlist)
	if err != nil {
		return nil, nil, err
	}
	typ, err := types.Instantiate(pkg.cb.ctxt, sig, targs[:n], true)
	return targs, typ, err
}

// ----------------------------------------------------------------------------
