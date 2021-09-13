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
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

type Contract interface {
	Match(pkg *Package, t types.Type) bool
	String() string
}

type TemplateParamType struct {
	name     string
	contract Contract
	idxFlag  int
}

func NewTemplateParamType(idx int, name string, contract Contract) *TemplateParamType {
	return &TemplateParamType{idxFlag: idx, name: name, contract: contract}
}

func (p *TemplateParamType) Underlying() types.Type {
	panic("TemplateParamType")
}

func (p *TemplateParamType) String() string {
	return fmt.Sprintf("TemplateParamType{name: %v}", p.name)
}

func (p *TemplateParamType) idx() int {
	return p.idxFlag &^ paramAllowUntyped
}

func (p *TemplateParamType) allowUntyped() bool {
	return (p.idxFlag & paramAllowUntyped) != 0
}

const (
	paramAllowUntyped = 0x10000
)

// ----------------------------------------------------------------------------

type unboundFuncParam struct {
	tBound types.Type
	typ    *TemplateParamType
	parg   *internal.Elem
}

func (p *unboundFuncParam) boundTo(pkg *Package, t types.Type, parg *internal.Elem) {
	if !p.typ.allowUntyped() {
		var expr *ast.Expr
		if parg != nil {
			expr = &parg.Val
		}
		t = DefaultConv(pkg, t, expr)
	}
	p.tBound, p.parg = t, parg
}

func (p *unboundFuncParam) Underlying() types.Type {
	panic("unboundFuncParam")
}

func (p *unboundFuncParam) String() string {
	return fmt.Sprintf("unboundFuncParam{typ: %v}", p.tBound)
}

type unboundProxyParam struct {
	real types.Type
}

func (p *unboundProxyParam) Underlying() types.Type {
	panic("unboundProxyParam")
}

func (p *unboundProxyParam) String() string {
	return fmt.Sprintf("unboundProxyParam{typ: %v}", p.real)
}

func getElemTypeIf(t types.Type, parg *internal.Elem) types.Type {
	if parg != nil && parg.CVal != nil {
		if tb, ok := t.(*types.Basic); ok && (tb.Info()&types.IsFloat) != 0 {
			if constant.ToInt(parg.CVal).Kind() == constant.Int {
				return types.Typ[types.UntypedInt]
			}
		}
	}
	return t
}

func boundType(pkg *Package, arg, param types.Type, parg *internal.Elem) error {
	switch p := param.(type) {
	case *unboundFuncParam: // template function param
		if p.typ.contract.Match(pkg, arg) {
			if p.tBound == nil {
				p.boundTo(pkg, arg, parg)
			} else if !AssignableTo(pkg, getElemTypeIf(arg, parg), p.tBound) {
				if isUntyped(pkg, p.tBound) && AssignableConv(pkg, p.tBound, arg, p.parg) {
					p.tBound = arg
					return nil
				}
				return fmt.Errorf("TODO: boundType %v => %v failed", arg, p.tBound)
			}
			return nil
		}
		return fmt.Errorf("TODO: contract.Match %v => %v failed", arg, p.typ.contract)
	case *unboundProxyParam:
		switch param := p.real.(type) {
		case *types.Pointer:
			switch t := arg.(type) {
			case *types.Pointer:
				return boundType(pkg, t.Elem(), param.Elem(), nil) // TODO: expr = nil
			case *refType:
				return boundType(pkg, t.typ, param.Elem(), nil)
			}
		case *types.Array:
			if t, ok := arg.(*types.Array); ok && param.Len() == t.Len() {
				return boundType(pkg, t.Elem(), param.Elem(), nil) // TODO: expr = nil
			}
		case *types.Map:
			if t, ok := arg.(*types.Map); ok {
				if err1 := boundType(pkg, t.Key(), param.Key(), nil); err1 != nil { // TODO: expr = nil
					return fmt.Errorf("TODO: bound map keyType %v => %v failed", t.Key(), param.Key())
				}
				return boundType(pkg, t.Elem(), param.Elem(), nil) // TODO: expr = nil
			}
		case *types.Chan:
			if t, ok := arg.(*types.Chan); ok {
				if dir := t.Dir(); dir == param.Dir() || dir == types.SendRecv {
					return boundType(pkg, t.Elem(), param.Elem(), nil) // TODO: expr = nil
				}
			}
		case *types.Struct:
			panic("TODO: boundType struct")
		default:
			log.Panicln("TODO: boundType - unknown type:", param)
		}
		return fmt.Errorf("TODO: bound %v => unboundProxyParam", arg)
	case *types.Slice:
		typ := arg
	retry:
		switch t := typ.(type) {
		case *types.Slice:
			return boundType(pkg, t.Elem(), p.Elem(), nil) // TODO: expr = nil
		case *types.Named:
			typ = pkg.cb.getUnderlying(t)
			goto retry
		}
		return fmt.Errorf("TODO: bound slice failed - %v not a slice", arg)
	case *types.Signature:
		panic("TODO: boundType function signature")
	default:
		if AssignableTo(pkg, arg, param) {
			return nil
		}
	}
	return fmt.Errorf("TODO: bound %v => %v", arg, param)
}

// Default returns the default "typed" type for an "untyped" type;
// it returns the incoming type for all other types. The default type
// for untyped nil is untyped nil.
//
func Default(pkg *Package, t types.Type) types.Type {
	return DefaultConv(pkg, t, nil)
}

func DefaultConv(pkg *Package, t types.Type, expr *ast.Expr) types.Type {
	named, ok := t.(*types.Named)
	if !ok {
		return types.Default(t)
	}
	o := named.Obj()
	if at := o.Pkg(); at != nil {
		name := o.Name() + "_Default"
		if typName := at.Scope().Lookup(name); typName != nil {
			if tn, ok := typName.(*types.TypeName); ok && tn.IsAlias() {
				typ := tn.Type()
				if expr != nil {
					if ok = assignable(pkg, t, typ.(*types.Named), expr); !ok {
						log.Panicln("==> DefaultConv failed:", t, typ)
					}
					if debugMatch {
						log.Println("==> DefaultConv", t, typ)
					}
				}
				return typ
			}
		}
	}
	return t
}

// AssignableTo reports whether a value of type V is assignable to a variable of type T.
func AssignableTo(pkg *Package, V, T types.Type) bool {
	return AssignableConv(pkg, V, T, nil)
}

func AssignableConv(pkg *Package, V, T types.Type, pv *internal.Elem) bool {
	pkg.cb.ensureLoaded(V)
	pkg.cb.ensureLoaded(T)
	V, T = realType(V), realType(T)
	if v, ok := V.(*refType); ok { // ref type
		if t, ok := T.(*types.Pointer); ok {
			V, T = v.typ, t.Elem()
		} else {
			V = v.typ
		}
	} else {
		V = getElemTypeIf(V, pv)
	}
	if types.AssignableTo(V, T) {
		if t, ok := T.(*types.Basic); ok { // untyped type
			vkind := V.(*types.Basic).Kind()
			tkind := t.Kind()
			switch {
			case vkind >= types.UntypedInt && vkind <= types.UntypedComplex:
				if tkind >= types.UntypedInt && tkind <= types.UntypedComplex {
					if vkind == tkind || vkind == types.UntypedRune {
						return true
					}
					return tkind != types.UntypedRune && tkind > vkind
				}
				if vkind == types.UntypedFloat {
					return tkind >= types.Float32
				}
				if vkind == types.UntypedComplex {
					return tkind >= types.Complex64
				}
			}
		}
		return true
	}
	if t, ok := T.(*types.Named); ok {
		var expr *ast.Expr
		if pv != nil {
			expr = &pv.Val
		}
		ok = assignable(pkg, V, t, expr)
		if debugMatch && pv != nil {
			log.Println("==> AssignableConv", V, T, ok)
		}
		return ok
	}
	return false
}

func assignable(pkg *Package, v types.Type, t *types.Named, expr *ast.Expr) bool {
	o := t.Obj()
	if at := o.Pkg(); at != nil {
		name := o.Name() + "_Init"
		if ini := at.Scope().Lookup(name); ini != nil {
			fn := &internal.Elem{Val: toObjectExpr(pkg, ini), Type: ini.Type()}
			arg := &internal.Elem{Type: v}
			if expr != nil {
				arg.Val = *expr
			}
			ret, err := matchFuncCall(pkg, fn, []*internal.Elem{arg}, 0)
			if err == nil {
				if expr != nil {
					*expr = ret.Val
				}
				return true
			}
		}
	}
	return false
}

func ComparableTo(pkg *Package, varg, targ *Element) bool {
	V, T := varg.Type, targ.Type
	if V == T {
		return true
	}

	switch v := V.(type) {
	case *types.Basic:
		if (v.Info() & types.IsUntyped) != 0 {
			return untypedComparable(pkg, v, varg, T)
		}
	case *types.Interface:
		return interfaceComparable(pkg, v, T)
	case *types.Named:
		if u, ok := v.Underlying().(*types.Interface); ok {
			return interfaceComparable(pkg, u, T)
		}
	}

	switch t := T.(type) {
	case *types.Basic:
		if (t.Info() & types.IsUntyped) != 0 {
			return untypedComparable(pkg, t, targ, V)
		}
	case *types.Interface:
		return interfaceComparable(pkg, t, V)
	case *types.Named:
		if u, ok := t.Underlying().(*types.Interface); ok {
			return interfaceComparable(pkg, u, V)
		}
	}

	if getUnderlying(pkg, V) != getUnderlying(pkg, T) {
		return false
	}
	return types.Comparable(V)
}

func interfaceComparable(pkg *Package, v *types.Interface, t types.Type) bool {
	if types.AssignableTo(t, v) {
		return true
	}
	if tt, ok := t.(*types.Named); ok {
		t = pkg.cb.getUnderlying(tt)
	}
	if tt, ok := t.(*types.Interface); ok {
		return types.AssignableTo(v, tt)
	}
	return false
}

func untypedComparable(pkg *Package, v *types.Basic, varg *Element, t types.Type) bool {
	kind := v.Kind()
	if kind == types.UntypedNil {
	retry:
		switch tt := t.(type) {
		case *types.Interface, *types.Slice, *types.Pointer, *types.Map, *types.Chan:
			return true
		case *types.Named:
			t = pkg.cb.getUnderlying(tt)
			goto retry
		case *types.Basic:
			return tt.Kind() == types.UnsafePointer || tt.Kind() == types.UntypedNil
		}
	} else if u, ok := getUnderlying(pkg, t).(*types.Basic); ok {
		switch v.Kind() {
		case types.UntypedBool:
			return (u.Info() & types.IsBoolean) != 0
		case types.UntypedFloat:
			if constant.ToInt(varg.CVal).Kind() != constant.Int {
				return (u.Info() & (types.IsFloat | types.IsComplex)) != 0
			}
			fallthrough
		case types.UntypedInt, types.UntypedRune:
			return (u.Info() & types.IsNumeric) != 0
		case types.UntypedComplex:
			return (u.Info() & types.IsComplex) != 0
		case types.UntypedString:
			return (u.Info() & types.IsString) != 0
		}
	}
	return false
}

// NewSignature returns a new function type for the given receiver, parameters,
// and results, either of which may be nil. If variadic is set, the function
// is variadic, it must have at least one parameter, and the last parameter
// must be of unnamed slice type.
func NewSignature(recv *types.Var, params, results *types.Tuple, variadic bool) *types.Signature {
	return types.NewSignature(recv, params, results, variadic)
}

// NewSlice returns a new slice type for the given element type.
func NewSlice(elem types.Type) types.Type {
	return types.NewSlice(elem)
}

// NewMap returns a new map for the given key and element types.
func NewMap(key, elem types.Type) types.Type {
	var t types.Type = types.NewMap(key, elem)
	if isUnboundParam(key) || isUnboundParam(elem) {
		t = &unboundProxyParam{real: t}
	}
	return t
}

// NewChan returns a new channel type for the given direction and element type.
func NewChan(dir types.ChanDir, elem types.Type) types.Type {
	var t types.Type = types.NewChan(dir, elem)
	if isUnboundParam(elem) {
		t = &unboundProxyParam{real: t}
	}
	return t
}

// NewArray returns a new array type for the given element type and length.
// A negative length indicates an unknown length.
func NewArray(elem types.Type, len int64) types.Type {
	var t types.Type = types.NewArray(elem, len)
	if isUnboundParam(elem) {
		t = &unboundProxyParam{real: t}
	}
	return t
}

// NewPointer returns a new pointer type for the given element (base) type.
func NewPointer(elem types.Type) types.Type {
	var t types.Type = types.NewPointer(elem)
	if isUnboundParam(elem) {
		t = &unboundProxyParam{real: t}
	}
	return t
}

func isUnboundParam(typ types.Type) bool {
	switch t := typ.(type) {
	case *unboundFuncParam:
		return true
	case *TemplateParamType:
		return true
	case *unboundProxyParam:
		return true
	case *types.Slice:
		return isUnboundParam(t.Elem())
	case *types.Signature:
		return isUnboundSignature(t)
	}
	return false
}

func isUnboundVar(v *types.Var) bool {
	if v == nil {
		return false
	}
	return isUnboundParam(v.Type())
}

func isUnboundTuple(t *types.Tuple) bool {
	for i, n := 0, t.Len(); i < n; i++ {
		if isUnboundVar(t.At(i)) {
			return true
		}
	}
	return false
}

func isUnboundSignature(sig *types.Signature) bool {
	return isUnboundVar(sig.Recv()) ||
		isUnboundTuple(sig.Params()) ||
		isUnboundTuple(sig.Results())
}

// ----------------------------------------------------------------------------

type instantiated struct {
	tparams []*unboundFuncParam
	results bool
}

func (p *instantiated) normalize(t types.Type) types.Type {
	if p != nil && p.results {
		t, _ = toNormalize(p.tparams, t)
	}
	return t
}

func (p *instantiated) normalizeTuple(t *types.Tuple) *types.Tuple {
	if p != nil && p.results {
		t, _ = toNormalizeTuple(p.tparams, t)
	}
	return t
}

func toNormalize(tparams []*unboundFuncParam, typ types.Type) (types.Type, bool) {
	switch tt := typ.(type) {
	case *unboundFuncParam:
		if tt.tBound == nil {
			log.Panicln("TODO: unbound type -", tt.typ.name)
		}
		return tt.tBound, true
	case *unboundProxyParam:
		switch t := tt.real.(type) {
		case *types.Pointer:
			elem, _ := toNormalize(tparams, t.Elem())
			return types.NewPointer(elem), true
		case *types.Array:
			elem, _ := toNormalize(tparams, t.Elem())
			return types.NewArray(elem, t.Len()), true
		case *types.Map:
			key, _ := toNormalize(tparams, t.Key())
			elem, _ := toNormalize(tparams, t.Elem())
			return types.NewMap(key, elem), true
		case *types.Chan:
			elem, _ := toNormalize(tparams, t.Elem())
			return types.NewChan(t.Dir(), elem), true
		case *types.Struct:
			panic("TODO: toNormalize struct")
		default:
			log.Panicln("TODO: toNormalize - unknown type:", t)
		}
	case *unboundType:
		if tt.tBound == nil {
			log.Panicln("TODO: unbound type")
		}
		return tt.tBound, true
	case *types.Slice:
		if elem, ok := toNormalize(tparams, tt.Elem()); ok {
			return types.NewSlice(elem), true
		}
	case *types.Signature:
		return toNormalizeSignature(tparams, tt)
	}
	return typ, false
}

func toNormalizeVar(tparams []*unboundFuncParam, param *types.Var) (*types.Var, bool) {
	if param == nil {
		return nil, false
	}
	if t, changed := toNormalize(tparams, param.Type()); changed {
		return types.NewParam(param.Pos(), param.Pkg(), param.Name(), t), true
	}
	return param, false
}

func toNormalizeTuple(tparams []*unboundFuncParam, params *types.Tuple) (*types.Tuple, bool) {
	n := params.Len()
	vars := make([]*types.Var, n)
	var ok, changed bool
	for i := 0; i < n; i++ {
		if vars[i], ok = toNormalizeVar(tparams, params.At(i)); ok {
			changed = true
		}
	}
	if changed {
		return types.NewTuple(vars...), true
	}
	return params, false
}

func toNormalizeSignature(
	tparams []*unboundFuncParam, sig *types.Signature) (*types.Signature, bool) {
	recv, ok1 := toNormalizeVar(tparams, sig.Recv())
	params, ok2 := toNormalizeTuple(tparams, sig.Params())
	results, ok3 := toNormalizeTuple(tparams, sig.Results())
	if ok1 || ok2 || ok3 {
		return types.NewSignature(recv, params, results, sig.Variadic()), true
	}
	return sig, false
}

// ----------------------------------------------------------------------------

const (
	tokUnaryFlag      token.Token = 0x80000
	tokFlagApproxType token.Token = 0x40000
	tokFlagAll                    = tokUnaryFlag | tokFlagApproxType
)

// TemplateSignature: type of template function
type TemplateSignature struct {
	params  []*TemplateParamType
	sig     *types.Signature
	tokFlag token.Token // tok + unary flag, only for builtin operator
}

func (p *TemplateSignature) tok() token.Token {
	return p.tokFlag &^ tokFlagAll
}

func (p *TemplateSignature) hasApproxType() bool {
	return (p.tokFlag & tokFlagApproxType) != 0
}

func (p *TemplateSignature) isOp() bool {
	return (p.tokFlag &^ tokFlagApproxType) != 0
}

func (p *TemplateSignature) isUnaryOp() bool {
	return (p.tokFlag & tokUnaryFlag) != 0
}

func assertValidTemplateSignature(tsig *TemplateSignature) {
	for i, param := range tsig.params {
		if param.idx() != i {
			panic("TODO: invalid TemplateSignature - incorrect index")
		}
	}
}

// NewTemplateSignature creates type of a template function.
func NewTemplateSignature(
	templateParams []*TemplateParamType,
	recv *types.Var, params, results *types.Tuple, variadic bool, tok ...token.Token) *TemplateSignature {

	var tokFlag token.Token
	if tok != nil {
		tokFlag = tok[0]
	}
	tsig := &TemplateSignature{
		params:  templateParams,
		sig:     types.NewSignature(recv, params, results, variadic),
		tokFlag: tokFlag,
	}
	if tsig.isOp() {
		for _, tparam := range templateParams {
			tparam.idxFlag |= paramAllowUntyped
		}
	}
	assertValidTemplateSignature(tsig)
	return tsig
}

func (p *TemplateSignature) Underlying() types.Type {
	panic("TemplateSignature")
}

func (p *TemplateSignature) String() string {
	return p.sig.String()
}

// TODO: check name
func (p *TemplateSignature) instantiate() (*types.Signature, *instantiated) {
	tparams := make([]*unboundFuncParam, len(p.params))
	for i, param := range p.params {
		tparams[i] = &unboundFuncParam{typ: param}
	}
	sig, _, instantiatedResults := toInstantiateSignature(tparams, p.sig)
	return sig, &instantiated{tparams: tparams, results: instantiatedResults}
}

func toInstantiate(tparams []*unboundFuncParam, typ types.Type) (types.Type, bool) {
	switch tt := typ.(type) {
	case *TemplateParamType:
		return tparams[tt.idx()], true
	case *unboundProxyParam:
		switch t := tt.real.(type) {
		case *types.Pointer:
			elem, _ := toInstantiate(tparams, t.Elem())
			return &unboundProxyParam{types.NewPointer(elem)}, true
		case *types.Array:
			elem, _ := toInstantiate(tparams, t.Elem())
			return &unboundProxyParam{types.NewArray(elem, t.Len())}, true
		case *types.Map:
			key, _ := toInstantiate(tparams, t.Key())
			elem, _ := toInstantiate(tparams, t.Elem())
			return &unboundProxyParam{types.NewMap(key, elem)}, true
		case *types.Chan:
			elem, _ := toInstantiate(tparams, t.Elem())
			return &unboundProxyParam{types.NewChan(t.Dir(), elem)}, true
		case *types.Struct:
			panic("TODO: instantiate struct")
		default:
			log.Panicln("TODO: toInstantiate - unknown type:", t)
		}
	case *types.Slice:
		if elem, ok := toInstantiate(tparams, tt.Elem()); ok {
			return types.NewSlice(elem), true
		}
	case *types.Signature:
		t, ok, _ := toInstantiateSignature(tparams, tt)
		return t, ok
	}
	return typ, false
}

func toInstantiateVar(tparams []*unboundFuncParam, param *types.Var) (*types.Var, bool) {
	if param == nil {
		return nil, false
	}
	if t, changed := toInstantiate(tparams, param.Type()); changed {
		return types.NewParam(param.Pos(), param.Pkg(), param.Name(), t), true
	}
	return param, false
}

func toInstantiateTuple(tparams []*unboundFuncParam, params *types.Tuple) (*types.Tuple, bool) {
	n := params.Len()
	vars := make([]*types.Var, n)
	var ok, changed bool
	for i := 0; i < n; i++ {
		if vars[i], ok = toInstantiateVar(tparams, params.At(i)); ok {
			changed = true
		}
	}
	if changed {
		return types.NewTuple(vars...), true
	}
	return params, false
}

func toInstantiateSignature(
	tparams []*unboundFuncParam, sig *types.Signature) (*types.Signature, bool, bool) {
	recv, ok1 := toInstantiateVar(tparams, sig.Recv())
	params, ok2 := toInstantiateTuple(tparams, sig.Params())
	results, ok3 := toInstantiateTuple(tparams, sig.Results())
	if ok1 || ok2 || ok3 {
		return types.NewSignature(recv, params, results, sig.Variadic()), true, ok3
	}
	return sig, false, ok3
}

// ----------------------------------------------------------------------------

// TemplateFunc: template function
type TemplateFunc struct {
	*types.Func
	sig *TemplateSignature
}

// NewTemplateFunc creates a template function.
func NewTemplateFunc(pos token.Pos, pkg *types.Package, name string, tsig *TemplateSignature) *TemplateFunc {
	return &TemplateFunc{sig: tsig, Func: types.NewFunc(pos, pkg, name, tsig.sig)}
}

// NewTemplateFunc return the type of specified template function.
func (p *TemplateFunc) Type() types.Type {
	return p.sig
}

// ----------------------------------------------------------------------------
