/*
 Copyright 2021 The XGo Authors (xgo.dev)
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

// ----------------------------------------------------------------------------

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
						if _, ok := typ.(*types.Alias); ok {
							typ = types.Unalias(typ)
						}
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
			ret, err := matchFuncCall(pkg, fn, []*internal.Elem{arg}, 0, 0)
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
	return types.NewMap(key, elem)
}

// NewChan returns a new channel type for the given direction and element type.
func NewChan(dir types.ChanDir, elem types.Type) types.Type {
	return types.NewChan(dir, elem)
}

// NewArray returns a new array type for the given element type and length.
// A negative length indicates an unknown length.
func NewArray(elem types.Type, len int64) types.Type {
	return types.NewArray(elem, len)
}

// NewPointer returns a new pointer type for the given element (base) type.
func NewPointer(elem types.Type) types.Type {
	return types.NewPointer(elem)
}

// ----------------------------------------------------------------------------

const (
	tokUnaryFlag      token.Token = 0x80000
	tokFlagApproxType token.Token = 0x40000 // ~T
	tokFlagAll                    = tokUnaryFlag | tokFlagApproxType
)

// TemplateSignature: type of template function
type TemplateSignature struct {
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

func (p *TemplateSignature) Underlying() types.Type { return p }
func (p *TemplateSignature) String() string {
	return fmt.Sprintf("TemplateSignature{%v}", p.sig)
}

// NewTemplateSignature creates type of a typeparams function.
func NewTemplateSignature(
	tparams []*types.TypeParam, recv *types.Var, params, results *types.Tuple, variadic bool, tok ...token.Token) *TemplateSignature {
	var tokFlag token.Token
	if tok != nil {
		tokFlag = tok[0]
	}
	tsig := &TemplateSignature{
		sig:     types.NewSignatureType(recv, nil, tparams, params, results, variadic),
		tokFlag: tokFlag,
	}
	return tsig
}

func (p *TemplateSignature) instantiate(pkg *Package, fn *internal.Elem, args []*internal.Elem, flags InstrFlags) (*types.Signature, error) {
	nargs := make([]*internal.Elem, len(args))
	copy(nargs, args)
	for i := 0; i < len(nargs); i++ {
		if ref, ok := nargs[i].Type.(*refType); ok {
			nargs[i] = &internal.Elem{
				Val:  args[i].Val,
				Type: types.NewPointer(ref.typ),
				CVal: args[i].CVal,
				Src:  args[i].Src,
			}
		}
	}
	if p.isOp() {
		// fix binary bigint -> rat
		if args[0].Type == pkg.utBigRat && args[1].Type == pkg.utBigInt {
			nargs[1] = &internal.Elem{
				Val:  args[1].Val,
				Type: types.Typ[types.UntypedInt],
				CVal: args[1].CVal,
				Src:  args[1].Src,
			}
		} else if args[0].Type == pkg.utBigInt && args[1].Type == pkg.utBigRat {
			nargs[0] = &internal.Elem{
				Val:  args[0].Val,
				Type: types.Typ[types.UntypedInt],
				CVal: args[0].CVal,
				Src:  args[0].Src,
			}
		}
	}
	sig, err := InferFunc(pkg, fn, p.sig, nil, nargs, flags)
	if err != nil {
		return nil, err
	}
	return sig.(*types.Signature), nil
}

func newTypeParams(pkg *types.Package, conf *Config, params []typeTParam) []*types.TypeParam {
	n := len(params)
	tparams := make([]*types.TypeParam, n)
	for i, tparam := range params {
		tparams[i] = types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, tparam.name, nil),
			makeConstraint(conf, tparam.contract.String()))
	}
	return tparams
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
