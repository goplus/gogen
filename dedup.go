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
	"reflect"
)

// ----------------------------------------------------------------------------

func dedupNamedType(imports map[string]*PkgRef, t *types.Named) (types.Type, bool) {
	o := t.Obj()
	if pkg := o.Pkg(); pkg != nil {
		if imp, ok := imports[pkg.Path()]; ok && imp.Types != pkg {
			if newo, ok := imp.Types.Scope().Lookup(o.Name()).(*types.TypeName); ok {
				if newo.IsAlias() {
					return newo.Type(), true
				}
				return newo.Type().(*types.Named), true
			} else {
				return dedupTypeName(imports, t.Obj()), true
			}
		}
	}
	return t, false
}

func dedupInterface(imports map[string]*PkgRef, t *types.Interface) (*types.Interface, bool) {
	var dedup, ok bool
	var methods []*types.Func
	if n := t.NumExplicitMethods(); n > 0 {
		methods = make([]*types.Func, n)
		for i := 0; i < n; i++ {
			if methods[i], ok = dedupFunc(imports, t.ExplicitMethod(i)); ok {
				dedup = true
			}
		}
	}
	var embeddeds []types.Type
	if n := t.NumEmbeddeds(); n > 0 {
		embeddeds = make([]types.Type, n)
		for i := 0; i < n; i++ {
			if embeddeds[i], ok = dedupType(imports, t.EmbeddedType(i)); ok {
				dedup = true
			}
		}
	}
	if dedup {
		return types.NewInterfaceType(methods, embeddeds).Complete(), true
	}
	return t, false
}

func dedupStruct(imports map[string]*PkgRef, t *types.Struct) (*types.Struct, bool) {
	var dedup, ok bool
	n := t.NumFields()
	fields := make([]*types.Var, n)
	tags := make([]string, n)
	for i := 0; i < n; i++ {
		if fields[i], ok = dedupField(imports, t.Field(i)); ok {
			dedup = true
		}
		tags[i] = t.Tag(i)
	}
	if dedup {
		return types.NewStruct(fields, tags), true
	}
	return t, false
}

func dedupSignature(imports map[string]*PkgRef, sig *types.Signature) (*types.Signature, bool) {
	var dedup, ok bool
	recv := sig.Recv()
	if recv != nil {
		if recv, ok = dedupParam(imports, recv); ok {
			dedup = true
		}
	}
	params, ok := dedupParams(imports, sig.Params())
	if ok {
		dedup = true
	}
	results, ok := dedupParams(imports, sig.Results())
	if ok {
		dedup = true
	}
	if dedup {
		return types.NewSignature(recv, params, results, sig.Variadic()), true
	}
	return sig, false
}

func dedupType(imports map[string]*PkgRef, typ types.Type) (types.Type, bool) {
	switch t := typ.(type) {
	case *types.Basic:
	case *types.Slice:
		if e, ok := dedupType(imports, t.Elem()); ok {
			return types.NewSlice(e), true
		}
	case *types.Map:
		k, ok1 := dedupType(imports, t.Key())
		e, ok2 := dedupType(imports, t.Elem())
		if ok1 || ok2 {
			return types.NewMap(k, e), true
		}
	case *types.Named:
		return dedupNamedType(imports, t)
	case *types.Pointer:
		if e, ok := dedupType(imports, t.Elem()); ok {
			return types.NewPointer(e), true
		}
	case *types.Interface:
		return dedupInterface(imports, t)
	case *types.Struct:
		return dedupStruct(imports, t)
	case *types.Signature:
		return dedupSignature(imports, t)
	case *types.Array:
		if e, ok := dedupType(imports, t.Elem()); ok {
			return types.NewArray(e, t.Len()), true
		}
	case *types.Chan:
		if e, ok := dedupType(imports, t.Elem()); ok {
			return types.NewChan(t.Dir(), e), true
		}
	default:
		panic("unsupported type - " + t.String())
	}
	return typ, false
}

func dedupField(imports map[string]*PkgRef, v *types.Var) (*types.Var, bool) {
	if typ, ok := dedupType(imports, v.Type()); ok {
		pkg := imports[v.Pkg().Path()].Types
		return types.NewField(token.NoPos, pkg, v.Name(), typ, v.Embedded()), true
	}
	return v, false
}

func dedupConst(imports map[string]*PkgRef, v *types.Const) (*types.Const, bool) {
	pkg := imports[v.Pkg().Path()].Types
	if typ, ok := dedupType(imports, v.Type()); ok {
		return types.NewConst(token.NoPos, pkg, v.Name(), typ, v.Val()), true
	}
	return v, false
}

func dedupParam(imports map[string]*PkgRef, v *types.Var) (*types.Var, bool) {
	if typ, ok := dedupType(imports, v.Type()); ok {
		pkg := imports[v.Pkg().Path()].Types
		return types.NewParam(token.NoPos, pkg, v.Name(), typ), true
	}
	return v, false
}

func dedupParams(imports map[string]*PkgRef, v *types.Tuple) (*types.Tuple, bool) {
	var dedup, ok bool
	n := v.Len()
	vars := make([]*types.Var, n)
	for i := 0; i < n; i++ {
		if vars[i], ok = dedupParam(imports, v.At(i)); ok {
			dedup = true
		}
	}
	if dedup {
		return types.NewTuple(vars...), true
	}
	return v, false
}

func dedupFunc(imports map[string]*PkgRef, v *types.Func) (*types.Func, bool) {
	if sig, ok := dedupSignature(imports, v.Type().(*types.Signature)); ok {
		pkg := imports[v.Pkg().Path()].Types
		return types.NewFunc(token.NoPos, pkg, v.Name(), sig), true
	}
	return v, false
}

func dedupTypeName(imports map[string]*PkgRef, v *types.TypeName) *types.Named {
	pkg := imports[v.Pkg().Path()].Types
	if v.IsAlias() {
		typ, _ := dedupType(imports, v.Type())
		o := types.NewTypeName(token.NoPos, pkg, v.Name(), typ)
		pkg.Scope().Insert(o)
		return types.NewNamed(o, typ.Underlying(), nil)
	}

	o := types.NewTypeName(token.NoPos, pkg, v.Name(), nil)
	pkg.Scope().Insert(o)
	ret := types.NewNamed(o, nil, nil)

	t := v.Type().(*types.Named)
	for i, n := 0, t.NumMethods(); i < n; i++ {
		mthd, _ := dedupFunc(imports, t.Method(i))
		ret.AddMethod(mthd)
	}
	typ, _ := dedupType(imports, t.Underlying())
	ret.SetUnderlying(typ)
	return ret
}

func dedupPkg(imports map[string]*PkgRef, self *PkgRef) *types.Package {
	pkgTypes := self.Types
	pkg := types.NewPackage(pkgTypes.Path(), pkgTypes.Name())
	scope := pkgTypes.Scope()
	self.Types = pkg
	for _, name := range scope.Names() {
		o := scope.Lookup(name)
		// need private types because they may be refered by public functions
		if v, ok := o.(*types.TypeName); ok {
			if pkg.Scope().Lookup(name) == nil {
				dedupTypeName(imports, v)
			}
			continue
		}
		if !token.IsExported(name) {
			continue
		}
		switch v := o.(type) {
		case *types.Func:
			o, _ = dedupFunc(imports, v)
		case *types.Const:
			o, _ = dedupConst(imports, v)
		case *types.Var:
			o, _ = dedupParam(imports, v)
		default:
			log.Panicln("unexpected object -", reflect.TypeOf(o), o.Name())
		}
		pkg.Scope().Insert(o)
	}
	return pkg
}

// ----------------------------------------------------------------------------
