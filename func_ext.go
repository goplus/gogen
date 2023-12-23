/*
 Copyright 2023 The GoPlus Authors (goplus.org)
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
)

// ----------------------------------------------------------------------------

// TyFuncEx is a FuncEx type.
type TyFuncEx interface {
	types.Type
	funcEx()
}

// IsFunc returns if specified type is a function or not.
func IsFunc(t types.Type) bool {
	if _, ok := t.(*types.Signature); ok {
		return true
	}
	_, ok := t.(*inferFuncType)
	return ok
}

// CheckFuncEx returns if specified function is a FuncEx or not.
func CheckFuncEx(sig *types.Signature) (TyFuncEx, bool) {
	if sig.Params().Len() == 1 && sig.Variadic() {
		if typ, ok := sig.Params().At(0).Type().(*types.Slice); ok {
			if typ, ok := typ.Elem().(*types.Interface); ok && typ.NumMethods() == 1 {
				if sig, ok := typ.Method(0).Type().(*types.Signature); ok {
					if recv := sig.Recv(); recv != nil {
						t, ok := recv.Type().(TyFuncEx)
						return t, ok
					}
				}
			}
		}
	}
	return nil, false
}

// sigFuncEx return func type (args ...interface{__gop_overload__()})
func sigFuncEx(pkg *types.Package, recv *types.Var, t TyFuncEx) *types.Signature {
	sig := types.NewSignature(types.NewVar(token.NoPos, nil, "recv", t), nil, nil, false)
	typ := types.NewInterfaceType([]*types.Func{
		types.NewFunc(token.NoPos, nil, "__gop_overload__", sig),
	}, nil)
	param := types.NewVar(token.NoPos, pkg, "args", types.NewSlice(typ))
	return types.NewSignature(recv, types.NewTuple(param), nil, true)
}

func newFuncEx(pos token.Pos, pkg *types.Package, recv *types.Var, name string, t TyFuncEx) *types.Func {
	sig := sigFuncEx(pkg, recv, t)
	return types.NewFunc(pos, pkg, name, sig)
}

func newMethodEx(typ *types.Named, pos token.Pos, pkg *types.Package, name string, t TyFuncEx) *types.Func {
	recv := types.NewVar(token.NoPos, pkg, "recv", typ)
	ofn := newFuncEx(pos, pkg, recv, name, t)
	typ.AddMethod(ofn)
	return ofn
}

// ----------------------------------------------------------------------------

// TyOverloadFunc: overload function type
type TyOverloadFunc struct {
	Funcs []types.Object
}

func (p *TyOverloadFunc) Underlying() types.Type { return p }
func (p *TyOverloadFunc) String() string         { return "TyOverloadFunc" }
func (p *TyOverloadFunc) funcEx()                {}

func NewOverloadFunc(pos token.Pos, pkg *types.Package, name string, funcs ...types.Object) *types.Func {
	return newFuncEx(pos, pkg, nil, name, &TyOverloadFunc{funcs})
}

func CheckOverloadFunc(sig *types.Signature) (funcs []types.Object, ok bool) {
	if t, ok := CheckFuncEx(sig); ok {
		if oft, ok := t.(*TyOverloadFunc); ok {
			return oft.Funcs, true
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------

// TyOverloadMethod: overload function type
type TyOverloadMethod struct {
	Methods []types.Object
}

func (p *TyOverloadMethod) Underlying() types.Type { return p }
func (p *TyOverloadMethod) String() string         { return "TyOverloadMethod" }
func (p *TyOverloadMethod) funcEx()                {}

func NewOverloadMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, methods ...types.Object) *types.Func {
	return newMethodEx(typ, pos, pkg, name, &TyOverloadMethod{methods})
}

func CheckOverloadMethod(sig *types.Signature) (methods []types.Object, ok bool) {
	if t, ok := CheckFuncEx(sig); ok {
		if oft, ok := t.(*TyOverloadMethod); ok {
			return oft.Methods, true
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------

type TyTemplateRecvMethod struct {
	Func types.Object
}

func (p *TyTemplateRecvMethod) Underlying() types.Type { return p }
func (p *TyTemplateRecvMethod) String() string         { return "TyTemplateRecvMethod" }
func (p *TyTemplateRecvMethod) funcEx()                {}

// NewTemplateRecvMethod - https://github.com/goplus/gop/issues/811
func NewTemplateRecvMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, fn types.Object) *types.Func {
	return newMethodEx(typ, pos, pkg, name, &TyTemplateRecvMethod{fn})
}

// ----------------------------------------------------------------------------

func overloadFnHasAutoProperty(fns []types.Object, n int) bool {
	for _, fn := range fns {
		if methodHasAutoProperty(fn.Type(), n) {
			return true
		}
	}
	return false
}

func methodHasAutoProperty(typ types.Type, n int) bool {
	if sig, ok := typ.(*types.Signature); ok {
		if t, ok := CheckFuncEx(sig); ok {
			switch t := t.(type) {
			case *TyOverloadMethod:
				// is overload method
				return overloadFnHasAutoProperty(t.Methods, n)
			case *TyTemplateRecvMethod:
				// is template recv method
				return methodHasAutoProperty(t.Func.Type(), 1)
			case *TyOverloadFunc:
				// is overload func
				return overloadFnHasAutoProperty(t.Funcs, n)
			}
		}
		return sig.Params().Len() == n
	}
	return false
}

// HasAutoProperty checks if specified type is a function without parameters or not.
func HasAutoProperty(typ types.Type) bool {
	if sig, ok := typ.(*types.Signature); ok {
		if t, ok := CheckFuncEx(sig); ok {
			switch t := t.(type) {
			case *TyOverloadFunc:
				// is overload func
				for _, fn := range t.Funcs {
					if HasAutoProperty(fn.Type()) {
						return true
					}
				}
			}
		} else {
			return sig.Params().Len() == 0
		}
	}
	return false
}

// ----------------------------------------------------------------------------

// CheckSignature checks param idx of typ signature.
// If nin >= 0, it means param idx is a function, and length of its params == nin;
// If nin == -1, it means param idx is a CompositeLit;
// If nin == -2, it means param idx is a SliceLit.
func CheckSignature(typ types.Type, idx, nin int) *types.Signature {
	if sig, ok := typ.(*types.Signature); ok {
		if t, ok := CheckFuncEx(sig); ok {
			switch t := t.(type) {
			case *TyOverloadFunc:
				return selOverloadFunc(t.Funcs, idx, nin)
			case *TyOverloadMethod:
				return selOverloadFunc(t.Methods, idx, nin)
			case *TyTemplateRecvMethod:
				if tsig, ok := t.Func.Type().(*types.Signature); ok {
					if tf, ok := CheckFuncEx(tsig); ok {
						if t, ok := tf.(*TyOverloadFunc); ok {
							return selOverloadFunc(t.Funcs, idx, nin)
						}
					}
					return sigWithoutParam1(tsig)
				}
			}
		}
		return sig
	}
	return nil
}

// CheckSignatures checks param idx of typ signature.
// If nin >= 0, it means param idx is a function, and length of its params == nin;
// If nin == -1, it means param idx is a CompositeLit;
// If nin == -2, it means param idx is a SliceLit.
func CheckSignatures(typ types.Type, idx, nin int) []*types.Signature {
	if sig, ok := typ.(*types.Signature); ok {
		if t, ok := CheckFuncEx(sig); ok {
			switch t := t.(type) {
			case *TyOverloadFunc:
				return selOverloadFuncs(t.Funcs, idx, nin)
			case *TyOverloadMethod:
				return selOverloadFuncs(t.Methods, idx, nin)
			case *TyTemplateRecvMethod:
				if tsig, ok := t.Func.Type().(*types.Signature); ok {
					if tf, ok := CheckFuncEx(tsig); ok {
						if t, ok := tf.(*TyOverloadFunc); ok {
							return selOverloadFuncs(t.Funcs, idx, nin)
						}
					}
					sig = sigWithoutParam1(tsig)
				}
			}
		}
		return []*types.Signature{sig}
	}
	return nil
}

func sigWithoutParam1(sig *types.Signature) *types.Signature {
	params := sig.Params()
	n := params.Len()
	mparams := make([]*types.Var, n-1)
	for i := range mparams {
		mparams[i] = params.At(i + 1)
	}
	return types.NewSignatureType(nil, nil, nil, types.NewTuple(mparams...), sig.Results(), sig.Variadic())
}

func selOverloadFunc(funcs []types.Object, idx, nin int) *types.Signature {
	for _, v := range funcs {
		if sig, ok := v.Type().(*types.Signature); ok {
			params := sig.Params()
			if idx < params.Len() && checkSigParam(params.At(idx).Type(), nin) {
				return sig
			}
		}
	}
	return nil
}

func selOverloadFuncs(funcs []types.Object, idx, nin int) (sigs []*types.Signature) {
	for _, v := range funcs {
		if sig, ok := v.Type().(*types.Signature); ok {
			params := sig.Params()
			if idx < params.Len() && checkSigParam(params.At(idx).Type(), nin) {
				sigs = append(sigs, sig)
			}
		}
	}
	return
}

func checkSigParam(typ types.Type, nin int) bool {
	switch nin {
	case -1: // input is CompositeLit
		if t, ok := typ.(*types.Pointer); ok {
			typ = t.Elem()
		}
		switch typ.(type) {
		case *types.Struct, *types.Named:
			return true
		}
	case -2: // input is SliceLit
		switch typ.(type) {
		case *types.Slice, *types.Named:
			return true
		}
	default:
		if t, ok := typ.(*types.Signature); ok {
			return t.Params().Len() == nin
		}
	}
	return false
}

// ----------------------------------------------------------------------------
