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
	"go/token"
	"go/types"
	"log"
)

// ----------------------------------------------------------------------------

type Contract interface {
	Match(t types.Type) bool
	String() string
}

type TemplateParamType struct {
	name     string
	contract Contract
	index    int
}

func NewTemplateParamType(idx int, name string, contract Contract) *TemplateParamType {
	return &TemplateParamType{index: idx, name: name, contract: contract}
}

func (p *TemplateParamType) Underlying() types.Type {
	panic("don't call me")
}

func (p *TemplateParamType) String() string {
	panic("don't call me")
}

// ----------------------------------------------------------------------------

type unboundFuncParam struct {
	bound types.Type
	typ   *TemplateParamType
}

func (p *unboundFuncParam) Bound(t types.Type) {
	if p.typ.contract.Match(t) {
		if p.bound == t {
			// nothing to do
		} else if p.bound == nil || types.AssignableTo(t, p.bound) {
			p.bound = t
		} else {
			panic("TODO: unmatched template param type")
		}
	} else {
		panic("TODO: unmatched template contract")
	}
}

func (p *unboundFuncParam) Underlying() types.Type {
	panic("don't call me")
}

func (p *unboundFuncParam) String() string {
	panic("don't call me")
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
	switch t := typ.(type) {
	case *types.Basic:
	case *types.Named:
	case *types.Interface:
		// nothing to do: interface doesn't support template
	case *types.Pointer:
		if elem, ok := toNormalize(tparams, t.Elem()); ok {
			return types.NewPointer(elem), true
		}
	case *types.Slice:
		if elem, ok := toNormalize(tparams, t.Elem()); ok {
			return types.NewSlice(elem), true
		}
	case *types.Array:
		if elem, ok := toNormalize(tparams, t.Elem()); ok {
			return types.NewArray(elem, t.Len()), true
		}
	case *types.Map:
		key, ok1 := toNormalize(tparams, t.Key())
		elem, ok2 := toNormalize(tparams, t.Elem())
		if ok1 || ok2 {
			return types.NewMap(key, elem), true
		}
	case *types.Chan:
		if elem, ok := toNormalize(tparams, t.Elem()); ok {
			return types.NewChan(t.Dir(), elem), true
		}
	case *types.Signature:
		return toNormalizeSignature(tparams, t)
	case *types.Struct:
		panic("TODO: instantiate struct")
	case *unboundFuncParam:
		if t.bound == nil {
			log.Panicln("TODO: unbound type -", t.typ.name)
		}
		return t.bound, true
	default:
		log.Panicln("TODO: normalize unknown type -", typ)
	}
	return typ, false
}

func toNormalizeVar(tparams []*unboundFuncParam, param *types.Var) (*types.Var, bool) {
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

// TemplateSignature: type of template function
type TemplateSignature struct {
	params []*TemplateParamType
	sig    *types.Signature
}

func assertValidTemplateSignature(tsig *TemplateSignature) {
	for i, param := range tsig.params {
		if param.index != i {
			panic("TODO: invalid TemplateSignature - incorrect index")
		}
	}
}

// NewTemplateSignature creates type of a template function.
func NewTemplateSignature(
	templateParams []*TemplateParamType,
	recv *types.Var, params, results *types.Tuple, variadic bool) *TemplateSignature {

	tsig := &TemplateSignature{params: templateParams, sig: types.NewSignature(recv, params, results, variadic)}
	assertValidTemplateSignature(tsig)
	return tsig
}

func (p *TemplateSignature) Underlying() types.Type {
	panic("don't call me")
}

func (p *TemplateSignature) String() string {
	panic("don't call me")
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
	switch t := typ.(type) {
	case *types.Basic:
	case *types.Named:
	case *types.Interface:
		// nothing to do: interface doesn't support template
	case *types.Pointer:
		if elem, ok := toInstantiate(tparams, t.Elem()); ok {
			return types.NewPointer(elem), true
		}
	case *types.Slice:
		if elem, ok := toInstantiate(tparams, t.Elem()); ok {
			return types.NewSlice(elem), true
		}
	case *types.Array:
		if elem, ok := toInstantiate(tparams, t.Elem()); ok {
			return types.NewArray(elem, t.Len()), true
		}
	case *types.Map:
		key, ok1 := toInstantiate(tparams, t.Key())
		elem, ok2 := toInstantiate(tparams, t.Elem())
		if ok1 || ok2 {
			return types.NewMap(key, elem), true
		}
	case *types.Chan:
		if elem, ok := toInstantiate(tparams, t.Elem()); ok {
			return types.NewChan(t.Dir(), elem), true
		}
	case *types.Signature:
		sig, ok, _ := toInstantiateSignature(tparams, t)
		return sig, ok
	case *types.Struct:
		panic("TODO: instantiate struct")
	case *TemplateParamType:
		return tparams[t.index], true
	default:
		log.Panicln("TODO: instantiate unknown type -", typ)
	}
	return typ, false
}

func toInstantiateVar(tparams []*unboundFuncParam, param *types.Var) (*types.Var, bool) {
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
