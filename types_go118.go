//go:build go1.18
// +build go1.18

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

package gox

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	_ "unsafe"

	"github.com/goplus/gox/internal"
)

const enableTypeParams = true

type inferFuncType struct {
	pkg   *Package
	typ   *types.Signature
	targs []types.Type
	src   ast.Node
}

func newInferFuncType(pkg *Package, typ *types.Signature, targs []types.Type, src ast.Node) *inferFuncType {
	return &inferFuncType{pkg: pkg, typ: typ, targs: targs, src: src}
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
	tyRet, err := inferType(p.pkg, p.src, p.typ, p.targs)
	if err != nil {
		_, pos := p.pkg.cb.loadExpr(p.src)
		p.pkg.cb.panicCodeErrorf(&pos, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func (p *inferFuncType) InstanceWithArgs(args []*internal.Elem) *types.Signature {
	tyRet, err := inferFunc(p.pkg, p.src, p.typ, p.targs, args)
	if err != nil {
		_, pos := p.pkg.cb.loadExpr(p.src)
		p.pkg.cb.panicCodeErrorf(&pos, "%v", err)
	}
	return tyRet.(*types.Signature)
}

func isGenericType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		return t.Origin() == t && t.TypeParams() != nil
	case *types.Signature:
		return t.TypeParams() != nil
	}
	return false
}

func (p *CodeBuilder) inferType(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	typ := args[0].Type
	var tt bool
	if t, ok := typ.(*TypeType); ok {
		typ = t.Type()
		tt = true
	}
	p.ensureLoaded(typ)
	srcExpr := getSrc(src)
	if !isGenericType(typ) {
		_, pos := p.loadExpr(srcExpr)
		if tt {
			p.panicCodeErrorf(&pos, "%v is not a generic type", typ)
		} else {
			p.panicCodeErrorf(&pos, "invalid operation: cannot index %v (value of type %v)", types.ExprString(args[0].Val), typ)
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
	if !tt {
		tyRet = newInferFuncType(p.pkg, typ.(*types.Signature), targs, srcExpr)
	} else {
		var err error
		tyRet, err = inferType(p.pkg, srcExpr, typ, targs)
		if err != nil {
			_, pos := p.loadExpr(srcExpr)
			p.panicCodeErrorf(&pos, "%v", err)
		}
		tyRet = NewTypeType(tyRet)
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
type builtinId int

type operand struct {
	mode operandMode
	expr ast.Expr
	typ  types.Type
	val  constant.Value
	id   builtinId
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

func inferFunc(pkg *Package, posn positioner, sig *types.Signature, targs []types.Type, args []*internal.Elem) (types.Type, error) {
	xlist := make([]*operand, len(args))
	for i, arg := range args {
		xlist[i] = &operand{
			mode: value,
			expr: arg.Val,
			typ:  arg.Type,
			val:  arg.CVal,
		}
	}
	tp := sig.TypeParams()
	n := tp.Len()
	tparams := make([]*types.TypeParam, n)
	for i := 0; i < n; i++ {
		tparams[i] = tp.At(i)
	}
	targs, err := infer(pkg, posn, tparams, targs, sig.Params(), xlist)
	if err != nil {
		return nil, err
	}
	return types.Instantiate(pkg.cb.ctxt, sig, targs, true)
}

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

func inferType(pkg *Package, posn positioner, typ types.Type, targs []types.Type) (types.Type, error) {
	if sig, ok := typ.(*types.Signature); ok {
		tp := sig.TypeParams()
		n := tp.Len()
		if len(targs) < n {
			tparams := make([]*types.TypeParam, n)
			for i := 0; i < n; i++ {
				tparams[i] = tp.At(i)
			}
			var err error
			targs, err = infer(pkg, posn, tparams, targs, nil, nil)
			if err != nil {
				return nil, err
			}
		}
	}
	return types.Instantiate(pkg.cb.ctxt, typ, targs, true)
}

func funcHasTypeParams(t *types.Signature) bool {
	return t.TypeParams() != nil
}
