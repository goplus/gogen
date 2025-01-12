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

package gogen

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"math/big"

	"github.com/goplus/gogen/internal"
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

func (p *TemplateParamType) Underlying() types.Type { return p }
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
		t = DefaultConv(pkg, t, parg)
	}
	p.tBound, p.parg = t, parg
}

func (p *unboundFuncParam) Underlying() types.Type { return p }
func (p *unboundFuncParam) String() string {
	return fmt.Sprintf("unboundFuncParam{typ: %v}", p.tBound)
}

type unboundProxyParam struct {
	real types.Type
}

func (p *unboundProxyParam) Underlying() types.Type { return p }
func (p *unboundProxyParam) String() string {
	return fmt.Sprintf("unboundProxyParam{typ: %v}", p.real)
}

func getElemTypeIf(t types.Type, parg *internal.Elem) types.Type {
	if parg != nil && parg.CVal != nil {
		if tb, ok := t.(*types.Basic); ok && tb.Kind() == types.UntypedFloat {
			if constant.ToInt(parg.CVal).Kind() == constant.Int {
				return types.Typ[types.UntypedInt]
			}
		}
	}
	return t
}

type boundTypeError struct {
	a, b types.Type
}

func (p *boundTypeError) Error() string {
	return fmt.Sprintf("boundType %v => %v failed", p.a, p.b)
}

func boundType(pkg *Package, arg, param types.Type, parg *internal.Elem) error {
	switch p := param.(type) {
	case *unboundFuncParam: // template function param
		if p.tBound == nil {
			if !p.typ.contract.Match(pkg, arg) {
				return fmt.Errorf("TODO: contract.Match %v => %v failed", arg, p.typ.contract)
			}
			p.boundTo(pkg, arg, parg)
		} else if !AssignableConv(pkg, getElemTypeIf(arg, parg), p.tBound, parg) {
			if !(isUntyped(pkg, p.tBound) && AssignableConv(pkg, p.tBound, arg, p.parg)) {
				return &boundTypeError{a: arg, b: p.tBound}
			}
			p.tBound = arg
		}
		return nil
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
					return &boundTypeError{a: t.Key(), b: param.Key()}
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
		if AssignableConv(pkg, arg, param, parg) {
			return nil
		}
	}
	return fmt.Errorf("TODO: bound %v => %v", arg, param)
}

// Default returns the default "typed" type for an "untyped" type;
// it returns the incoming type for all other types. The default type
// for untyped nil is untyped nil.
func Default(pkg *Package, t types.Type) types.Type {
	return DefaultConv(pkg, t, nil)
}

func DefaultConv(pkg *Package, t types.Type, pv *Element) types.Type {
	switch typ := t.(type) {
	case *types.Named:
		o := typ.Obj()
		if at := o.Pkg(); at != nil {
			name := o.Name() + "_Default"
			if typName := at.Scope().Lookup(name); typName != nil {
				if tn, ok := typName.(*types.TypeName); ok && tn.IsAlias() {
					typ := tn.Type()
					if pv != nil {
						if ok = assignable(pkg, t, typ.(*types.Named), pv); !ok {
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
	case *inferFuncType:
		return typ.Instance()
	case *types.Signature:
		if funcs, ok := CheckOverloadFunc(typ); ok {
			if len(funcs) == 1 {
				o := funcs[0]
				if pv != nil {
					pv.Val = toObjectExpr(pkg, o)
				}
				return o.Type()
			}
			log.Panicln("==> DefaultConv failed: overload functions have no default type")
		}
	default:
		return types.Default(t)
	}
	return t
}

func ConvertibleTo(pkg *Package, V, T types.Type) bool {
	pkg.cb.ensureLoaded(V)
	pkg.cb.ensureLoaded(T)
	if V == types.Typ[types.UnsafePointer] {
		if _, ok := T.(*types.Pointer); ok {
			return true
		}
	}
	return types.ConvertibleTo(V, T)
}

// AssignableTo reports whether a value of type V is assignable to a variable of type T.
func AssignableTo(pkg *Package, V, T types.Type) bool {
	return AssignableConv(pkg, V, T, nil)
}

func AssignableConv(pkg *Package, V, T types.Type, pv *Element) bool {
	pkg.cb.ensureLoaded(V)
	pkg.cb.ensureLoaded(T)
	V, T = realType(V), realType(T)
	switch v := V.(type) {
	case *refType: // ref type
		if t, ok := T.(*types.Pointer); ok {
			V, T = v.typ, t.Elem()
		} else {
			V = v.typ
		}
	case *inferFuncType:
		V = v.Instance()
	case *types.Signature:
		if funcs, ok := CheckOverloadFunc(v); ok {
			if len(funcs) == 1 {
				o := funcs[0]
				V = o.Type()
				if pv != nil {
					pv.Val = toObjectExpr(pkg, o)
					pv.Type = V
				}
			}
		}
	default:
		V = getElemTypeIf(V, pv)
	}
	if types.AssignableTo(V, T) {
		return assignableTo(V, T, pv)
	}
	if t, ok := T.(*types.Named); ok {
		ok = assignable(pkg, V, t, pv)
		if debugMatch && pv != nil {
			log.Println("==> AssignableConv", V, T, ok)
		}
		return ok
	}
	if pkg.implicitCast != nil {
		return pkg.implicitCast(pkg, V, T, pv)
	}
	return false
}

func assignableTo(V, T types.Type, pv *Element) bool {
	if t, ok := T.Underlying().(*types.Basic); ok { // untyped type
		if v, ok := V.Underlying().(*types.Basic); ok {
			tkind, vkind := t.Kind(), v.Kind()
			if vkind >= types.UntypedInt && vkind <= types.UntypedComplex {
				if tkind <= types.Uintptr && pv != nil && outOfRange(tkind, pv.CVal) {
					if debugMatch {
						log.Printf("==> AssignableConv %v (%v): value is out of %v range", V, pv.CVal, T)
					}
					return false
				}
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
	}
	return true
}

func outOfRange(tkind types.BasicKind, cval constant.Value) bool {
	// untyped int may not a constant. For an example:
	//    func GetValue(shift uint) uint {
	//       return 1 << shift
	//    }
	if cval == nil {
		return false
	}
	rg := tkindRanges[tkind]
	return constant.Compare(cval, token.LSS, rg[0]) || constant.Compare(cval, token.GTR, rg[1])
}

const (
	intSize    = 32 << (^uint(0) >> 63)
	intptrSize = 32 << (^uintptr(0) >> 63)
	maxUint    = (1 << intSize) - 1
	maxUintptr = (1 << intptrSize) - 1
	maxUint8   = (1 << 8) - 1
	maxUint16  = (1 << 16) - 1
	maxUint32  = (1 << 32) - 1
	maxUint64  = (1 << 64) - 1
	minInt     = -(1 << (intSize - 1))
	maxInt     = (1 << (intSize - 1)) - 1
	minInt8    = -(1 << (8 - 1))
	maxInt8    = (1 << (8 - 1)) - 1
	minInt16   = -(1 << (16 - 1))
	maxInt16   = (1 << (16 - 1)) - 1
	minInt32   = -(1 << (32 - 1))
	maxInt32   = (1 << (32 - 1)) - 1
	minInt64   = -(1 << (64 - 1))
	maxInt64   = (1 << (64 - 1)) - 1
)

var (
	tkindRanges = [...][2]constant.Value{
		types.Int:     {constant.MakeInt64(minInt), constant.MakeInt64(maxInt)},
		types.Int8:    {constant.MakeInt64(minInt8), constant.MakeInt64(maxInt8)},
		types.Int16:   {constant.MakeInt64(minInt16), constant.MakeInt64(maxInt16)},
		types.Int32:   {constant.MakeInt64(minInt32), constant.MakeInt64(maxInt32)},
		types.Int64:   {constant.MakeInt64(minInt64), constant.MakeInt64(maxInt64)},
		types.Uint:    {constant.MakeInt64(0), constant.MakeUint64(maxUint)},
		types.Uint8:   {constant.MakeInt64(0), constant.MakeUint64(maxUint8)},
		types.Uint16:  {constant.MakeInt64(0), constant.MakeUint64(maxUint16)},
		types.Uint32:  {constant.MakeInt64(0), constant.MakeUint64(maxUint32)},
		types.Uint64:  {constant.MakeInt64(0), constant.MakeUint64(maxUint64)},
		types.Uintptr: {constant.MakeInt64(0), constant.MakeUint64(maxUintptr)},
	}
)

func assignable(pkg *Package, v types.Type, t *types.Named, pv *internal.Elem) bool {
	o := t.Obj()
	if at := o.Pkg(); at != nil {
		tname := o.Name()
		scope := at.Scope()
		name := tname + "_Init"
		if ini := scope.Lookup(name); ini != nil {
			if v == types.Typ[types.UntypedInt] {
				switch t {
				case pkg.utBigInt, pkg.utBigRat:
					if pv != nil {
						switch cv := constant.Val(pv.CVal).(type) {
						case *big.Int:
							nv := pkg.cb.UntypedBigInt(cv).stk.Pop()
							pv.Type, pv.Val = nv.Type, nv.Val
						}
					}
					return true
				}
			}
			if pv.CVal != nil {
				if checkUntypedOverflows(scope, tname, pv) {
					return false
				}
			}
			fn := &internal.Elem{Val: toObjectExpr(pkg, ini), Type: ini.Type()}
			arg := &internal.Elem{Type: v}
			if pv != nil {
				arg.Val, arg.CVal, arg.Src = pv.Val, pv.CVal, pv.Src
			}
			ret, err := matchFuncCall(pkg, fn, []*internal.Elem{arg}, 0)
			if err == nil {
				if pv != nil {
					pv.Val = ret.Val
				}
				return true
			}
		}
	}
	return false
}

func ComparableTo(pkg *Package, varg, targ *Element) bool {
	V, T := varg.Type, targ.Type
	if v, ok := V.(*types.Basic); ok {
		if (v.Info() & types.IsUntyped) != 0 {
			return untypedComparable(pkg, v, varg, T)
		}
	}
	if t, ok := T.(*types.Basic); ok {
		if (t.Info() & types.IsUntyped) != 0 {
			return untypedComparable(pkg, t, targ, V)
		}
	}
	if getUnderlying(pkg, V) == getUnderlying(pkg, T) {
		return true
	}
	return AssignableConv(pkg, V, T, varg) || AssignableConv(pkg, T, V, targ)
}

func untypedComparable(pkg *Package, v *types.Basic, varg *Element, t types.Type) bool {
	kind := v.Kind()
	if kind == types.UntypedNil {
	retry:
		switch tt := t.(type) {
		case *types.Interface, *types.Slice, *types.Pointer, *types.Map, *types.Signature, *types.Chan:
			return true
		case *types.Basic:
			return tt.Kind() == types.UnsafePointer // invalid: nil == nil
		case *types.Named:
			t = pkg.cb.getUnderlying(tt)
			goto retry
		}
	} else {
		switch u := getUnderlying(pkg, t).(type) {
		case *types.Basic:
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
		case *types.Interface:
			return u.Empty()
		}
	}
	return false
}

// NewSignature returns a new function type for the given receiver, parameters,
// and results, either of which may be nil. If variadic is set, the function
// is variadic, it must have at least one parameter, and the last parameter
// must be of unnamed slice type.
func NewSignature(recv *types.Var, params, results *types.Tuple, variadic bool) *types.Signature {
	return types.NewSignatureType(recv, nil, nil, params, results, variadic)
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
		return types.NewSignatureType(recv, nil, nil, params, results, sig.Variadic()), true
	}
	return sig, false
}

// ----------------------------------------------------------------------------

const (
	tokUnaryFlag      token.Token = 0x80000
	tokFlagApproxType token.Token = 0x40000 // ~T
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
		sig:     types.NewSignatureType(recv, nil, nil, params, results, variadic),
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

func (p *TemplateSignature) Underlying() types.Type { return p }
func (p *TemplateSignature) String() string {
	return fmt.Sprintf("TemplateSignature{%v}", p.sig)
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
		return types.NewSignatureType(recv, nil, nil, params, results, sig.Variadic()), true, ok3
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
