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

package gogen

import (
	"go/token"
	"go/types"
	"log"
	"strings"
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
func CheckFuncEx(sig *types.Signature) (ext TyFuncEx, ok bool) {
	if typ, is := CheckSigFuncEx(sig); is {
		ext, ok = typ.(TyFuncEx)
	}
	return
}

// CheckSigFuncExObjects retruns hide recv type and objects from func($overloadArgs ...interface{$overloadMethod()})
// The return type can be OverloadType (*TyOverloadFunc, *TyOverloadMethod, *TyOverloadNamed) or
// *TyTemplateRecvMethod.
func CheckSigFuncExObjects(sig *types.Signature) (typ types.Type, objs []types.Object) {
	if ext, ok := CheckSigFuncEx(sig); ok {
		switch t := ext.(type) {
		case *TyOverloadFunc:
			typ, objs = t, t.Funcs
		case *TyOverloadMethod:
			typ, objs = t, t.Methods
		case *TyTemplateRecvMethod:
			typ = t
			if tsig, ok := t.Func.Type().(*types.Signature); ok {
				if ex, ok := CheckSigFuncEx(tsig); ok {
					if t, ok := ex.(*TyOverloadFunc); ok {
						objs = t.Funcs
						break
					}
				}
			}
			objs = []types.Object{t.Func}
		case *TyOverloadNamed:
			typ = t
			objs = make([]types.Object, len(t.Types))
			for i, typ := range t.Types {
				objs[i] = typ.Obj()
			}
		}
	}
	return
}

const (
	overloadArgs   = "__gop_overload_args__"
	overloadMethod = "_"
)

// CheckSigFuncEx retrun hide recv type from func($overloadArgs ...interface{$overloadMethod()})
// The return type can be OverloadType (*TyOverloadFunc, *TyOverloadMethod, *TyOverloadNamed) or
// *TyTemplateRecvMethod.
func CheckSigFuncEx(sig *types.Signature) (types.Type, bool) {
	if sig.Params().Len() == 1 {
		if param := sig.Params().At(0); param.Name() == overloadArgs {
			if typ, ok := param.Type().(*types.Interface); ok && typ.NumExplicitMethods() == 1 {
				if sig, ok := typ.ExplicitMethod(0).Type().(*types.Signature); ok {
					if recv := sig.Recv(); recv != nil {
						return recv.Type(), true
					}
				}
			}
		}
	}
	return nil, false
}

func isSigFuncEx(sig *types.Signature) bool {
	if sig.Params().Len() == 1 {
		if param := sig.Params().At(0); param.Name() == overloadArgs {
			return true
		}
	}
	return false
}

// sigFuncEx return func type ($overloadArgs ...interface{$overloadMethod()})
func sigFuncEx(pkg *types.Package, recv *types.Var, t types.Type) *types.Signature {
	sig := types.NewSignatureType(types.NewVar(token.NoPos, nil, "", t), nil, nil, nil, nil, false)
	typ := types.NewInterfaceType([]*types.Func{
		types.NewFunc(token.NoPos, nil, overloadMethod, sig),
	}, nil)
	param := types.NewVar(token.NoPos, pkg, overloadArgs, typ)
	return types.NewSignatureType(recv, nil, nil, types.NewTuple(param), nil, false)
}

func newFuncEx(pos token.Pos, pkg *types.Package, recv *types.Var, name string, t TyFuncEx) *types.Func {
	sig := sigFuncEx(pkg, recv, t)
	return types.NewFunc(pos, pkg, name, sig)
}

func newMethodEx(typ *types.Named, pos token.Pos, pkg *types.Package, name string, t TyFuncEx) *types.Func {
	recv := types.NewVar(token.NoPos, pkg, "recv", typ)
	ofn := newFuncEx(pos, pkg, recv, name, t)
	typ.AddMethod(ofn)
	if strings.HasPrefix(name, gopxPrefix) {
		aname := name[len(gopxPrefix):]
		ofnAlias := newFuncEx(pos, pkg, recv, aname, &tyTypeAsParams{ofn})
		typ.AddMethod(ofnAlias)
		if debugImport {
			log.Println("==> AliasMethod", typ, name, "=>", aname)
		}
	}
	return ofn
}

// ----------------------------------------------------------------------------

// TyOverloadFunc: overload function type
type TyOverloadFunc struct {
	Funcs []types.Object
}

func (p *TyOverloadFunc) At(i int) types.Object { return p.Funcs[i] }
func (p *TyOverloadFunc) Len() int              { return len(p.Funcs) }

func (p *TyOverloadFunc) Underlying() types.Type { return p }
func (p *TyOverloadFunc) String() string         { return "TyOverloadFunc" }
func (p *TyOverloadFunc) funcEx()                {}

// NewOverloadFunc creates an overload func.
func NewOverloadFunc(pos token.Pos, pkg *types.Package, name string, funcs ...types.Object) *types.Func {
	fn := newFuncEx(pos, pkg, nil, name, &TyOverloadFunc{funcs})
	return fn
}

// CheckOverloadFunc checks a func is overload func or not.
//
// Deprecated: please use CheckFuncEx.
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

func (p *TyOverloadMethod) At(i int) types.Object { return p.Methods[i] }
func (p *TyOverloadMethod) Len() int              { return len(p.Methods) }

func (p *TyOverloadMethod) Underlying() types.Type { return p }
func (p *TyOverloadMethod) String() string         { return "TyOverloadMethod" }
func (p *TyOverloadMethod) funcEx()                {}

// NewOverloadMethod creates an overload method.
func NewOverloadMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, methods ...types.Object) *types.Func {
	return newMethodEx(typ, pos, pkg, name, &TyOverloadMethod{methods})
}

// CheckOverloadMethod checks a func is overload method or not.
//
// Deprecated: please use CheckFuncEx.
func CheckOverloadMethod(sig *types.Signature) (methods []types.Object, ok bool) {
	if t, ok := CheckFuncEx(sig); ok {
		if oft, ok := t.(*TyOverloadMethod); ok {
			return oft.Methods, true
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------

type tyTypeAsParams struct { // see TestTypeAsParamsFunc
	obj types.Object
}

func (p *tyTypeAsParams) Obj() types.Object      { return p.obj }
func (p *tyTypeAsParams) Underlying() types.Type { return p }
func (p *tyTypeAsParams) String() string         { return "tyTypeAsParams" }
func (p *tyTypeAsParams) funcEx()                {}

// ----------------------------------------------------------------------------

type TyStaticMethod struct {
	Func types.Object
}

func (p *TyStaticMethod) Obj() types.Object      { return p.Func }
func (p *TyStaticMethod) Underlying() types.Type { return p }
func (p *TyStaticMethod) String() string         { return "TyStaticMethod" }
func (p *TyStaticMethod) funcEx()                {}

func NewStaticMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, fn types.Object) *types.Func {
	return newMethodEx(typ, pos, pkg, name, &TyStaticMethod{fn})
}

// ----------------------------------------------------------------------------

type TyTemplateRecvMethod struct {
	Func types.Object
}

func (p *TyTemplateRecvMethod) Obj() types.Object      { return p.Func }
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
