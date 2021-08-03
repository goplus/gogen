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
}

func (p *unboundFuncParam) boundTo(t types.Type) {
	if !p.typ.allowUntyped() {
		t = types.Default(t)
	}
	p.tBound = t
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

func boundType(pkg *Package, arg, param types.Type) error {
	switch p := param.(type) {
	case *unboundFuncParam: // template function param
		if p.typ.contract.Match(pkg, arg) {
			if p.tBound == nil {
				p.boundTo(arg)
			} else if !AssignableTo(pkg, arg, p.tBound) {
				if isUntyped(p.tBound) && AssignableTo(pkg, p.tBound, arg) {
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
			if t, ok := arg.(*types.Pointer); ok {
				return boundType(pkg, t.Elem(), param.Elem())
			}
		case *types.Array:
			if t, ok := arg.(*types.Array); ok && param.Len() == t.Len() {
				return boundType(pkg, t.Elem(), param.Elem())
			}
		case *types.Map:
			if t, ok := arg.(*types.Map); ok {
				if err1 := boundType(pkg, t.Key(), param.Key()); err1 != nil {
					return fmt.Errorf("TODO: bound map keyType %v => %v failed", t.Key(), param.Key())
				}
				return boundType(pkg, t.Elem(), param.Elem())
			}
		case *types.Chan:
			if t, ok := arg.(*types.Chan); ok {
				if dir := t.Dir(); dir == param.Dir() || dir == types.SendRecv {
					return boundType(pkg, t.Elem(), param.Elem())
				}
			}
		case *types.Struct:
			panic("TODO: boundType struct")
		default:
			log.Panicln("TODO: boundType - unknown type:", param)
		}
		return fmt.Errorf("TODO: bound %v => unboundProxyParam", arg)
	case *types.Slice:
		if t, ok := arg.(*types.Slice); ok {
			return boundType(pkg, t.Elem(), p.Elem())
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
	if named, ok := t.(*types.Named); ok {
		o := named.Obj()
		if at := o.Pkg(); at != nil {
			name := o.Name() + "_Default"
			if typ := at.Scope().Lookup(name); typ != nil {
				if tn, ok := typ.(*types.TypeName); ok {
					return tn.Type()
				}
			}
		}
	}
	return types.Default(t)
}

// AssignableTo reports whether a value of type V is assignable to a variable of type T.
func AssignableTo(pkg *Package, V, T types.Type) bool {
	V, T = realType(V), realType(T)
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
		o := t.Obj()
		if at := o.Pkg(); at != nil {
			name := o.Name() + "_Init"
			if ini := at.Scope().Lookup(name); ini != nil {
				fn := internal.Elem{Type: ini.Type()}
				args := []internal.Elem{{Type: V}}
				_, err := matchFuncCall(pkg, fn, args, 0)
				return err == nil
			}
		}
	}
	return false
}

func ComparableTo(pkg *Package, V, T types.Type) bool {
	V, T = types.Default(V), types.Default(T)
	if V != T && V.Underlying() != T.Underlying() {
		return false
	}
	return types.Comparable(V)
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
	return isUnboundVar(sig.Recv()) &&
		isUnboundTuple(sig.Params()) &&
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
	tokUnaryFlag token.Token = 0x80000
)

// TemplateSignature: type of template function
type TemplateSignature struct {
	params  []*TemplateParamType
	sig     *types.Signature
	tokFlag token.Token // only for builtin operator
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
	if tokFlag != 0 {
		for _, tparam := range templateParams {
			tparam.idxFlag |= paramAllowUntyped
		}
	}
	tsig := &TemplateSignature{
		params:  templateParams,
		sig:     types.NewSignature(recv, params, results, variadic),
		tokFlag: tokFlag,
	}
	assertValidTemplateSignature(tsig)
	return tsig
}

func (p *TemplateSignature) Underlying() types.Type {
	panic("TemplateSignature")
}

func (p *TemplateSignature) String() string {
	panic("TemplateSignature")
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
