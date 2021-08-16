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
	"archive/zip"
	"bytes"
	"encoding/json"
	"go/constant"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"reflect"
	"strconv"
)

type pobj = map[string]interface{}

// ----------------------------------------------------------------------------

func toPersistNamedType(t *types.Named) interface{} {
	o := t.Obj()
	ret := pobj{"type": "named", "name": o.Name()}
	if pkg := o.Pkg(); pkg != nil {
		ret["pkg"] = pkg.Path()
	}
	return ret
}

func toPersistInterface(t *types.Interface) interface{} {
	n := t.NumExplicitMethods()
	methods := make([]persistFunc, n)
	for i := 0; i < n; i++ {
		methods[i] = *toPersistFunc(t.ExplicitMethod(i))
	}
	ret := pobj{"type": "interface", "methods": methods}
	if n := t.NumEmbeddeds(); n > 0 {
		embeddeds := make([]interface{}, n)
		for i := 0; i < n; i++ {
			embeddeds[i] = toPersistType(t.EmbeddedType(i))
		}
		ret["embeddeds"] = embeddeds
	}
	return ret
}

func toPersistStruct(t *types.Struct) interface{} {
	n := t.NumFields()
	fields := make([]persistVar, n)
	for i := 0; i < n; i++ {
		fields[i] = toPersistVar(t.Field(i))
		fields[i].Tag = t.Tag(i)
	}
	return pobj{"type": "struct", "fields": fields}
}

func toPersistSignature(sig *types.Signature) interface{} {
	recv := sig.Recv()
	if recv != nil {
		switch t := recv.Type().(type) {
		case *types.Named:
			if _, ok := t.Underlying().(*types.Interface); ok {
				recv = nil
			}
		case *overloadFuncType:
			return nil
		case *types.Interface:
			recv = nil
		}
	}
	params := toPersistVars(sig.Params())
	results := toPersistVars(sig.Results())
	variadic := sig.Variadic()
	ret := pobj{"type": "sig", "params": params, "results": results}
	if recv != nil {
		ret["recv"] = toPersistVar(recv)
	}
	if variadic {
		ret["variadic"] = true
	}
	return ret
}

func toPersistType(typ types.Type) interface{} {
	switch t := typ.(type) {
	case *types.Basic:
		return t.Name()
	case *types.Slice:
		return pobj{"type": "slice", "elem": toPersistType(t.Elem())}
	case *types.Map:
		return pobj{"type": "map", "key": toPersistType(t.Key()), "elem": toPersistType(t.Elem())}
	case *types.Named:
		return toPersistNamedType(t)
	case *types.Pointer:
		return pobj{"type": "ptr", "elem": toPersistType(t.Elem())}
	case *types.Interface:
		return toPersistInterface(t)
	case *types.Struct:
		return toPersistStruct(t)
	case *types.Signature:
		return toPersistSignature(t)
	case *types.Array:
		n := strconv.FormatInt(t.Len(), 10)
		return pobj{"type": "array", "elem": toPersistType(t.Elem()), "len": n}
	case *types.Chan:
		return pobj{"type": "chan", "elem": toPersistType(t.Elem()), "dir": int(t.Dir())}
	default:
		panic("unsupported type - " + t.String())
	}
}

/*
func fromPersistType(typ interface{}) types.Type {
	switch t := typ.(type) {
	case string:
		return types.Universe.Lookup(t).Type()
	default:
		panic("TODO: unexpected")
	}
}
*/

// ----------------------------------------------------------------------------

func toPersistVal(val constant.Value) interface{} {
	switch v := constant.Val(val).(type) {
	case string:
		return v
	case int64:
		return pobj{"type": "int64", "val": strconv.FormatInt(v, 10)}
	case bool:
		return v
	case *big.Int:
		return pobj{"type": "bigint", "val": v.String()}
	case *big.Rat:
		return pobj{"type": "bigrat", "num": v.Num().String(), "denom": v.Denom().String()}
	default:
		panic("unsupported constant")
	}
}

/*
func fromPersistVal(val interface{}) constant.Value {
	var ret interface{}
	switch v := val.(type) {
	case string:
		ret = v
	case pobj:
		switch typ := v["type"].(string); typ {
		case "int64":
			iv, err := strconv.ParseInt(v["val"].(string), 10, 64)
			if err != nil {
				panic("fromPersistVal failed: " + err.Error())
			}
			ret = iv
		case "bigint":
			bval := v["val"].(string)
			bv, ok := new(big.Int).SetString(bval, 10)
			if !ok {
				panic("fromPersistVal failed: " + bval)
			}
			ret = bv
		case "bigrat":
			num := v["num"].(string)
			denom := v["denom"].(string)
			bnum, ok1 := new(big.Int).SetString(num, 10)
			bdenom, ok2 := new(big.Int).SetString(denom, 10)
			if !ok1 || !ok2 {
				panic("fromPersistVal failed: " + num + "/" + denom)
			}
			ret = new(big.Rat).SetFrac(bnum, bdenom)
		default:
			panic("fromPersistVal failed: unknown type - " + typ)
		}
	case bool:
		ret = v
	default:
		panic("fromPersistVal: unsupported constant")
	}
	return constant.Make(ret)
}
*/

// ----------------------------------------------------------------------------

type persistVar struct {
	Name string      `json:"name,omitempty"`
	Type interface{} `json:"type"`
	Tag  string      `json:"tag,omitempty"`
}

func toPersistVar(v *types.Var) persistVar {
	return persistVar{
		Name: v.Name(),
		Type: toPersistType(v.Type()),
	}
}

func toPersistVars(v *types.Tuple) []persistVar {
	n := v.Len()
	vars := make([]persistVar, n)
	for i := 0; i < n; i++ {
		vars[i] = toPersistVar(v.At(i))
	}
	return vars
}

type persistConst struct {
	Name string      `json:"name"`
	Type interface{} `json:"type"`
	Val  interface{} `json:"val"`
}

func toPersistConst(v *types.Const) persistConst {
	return persistConst{
		Name: v.Name(),
		Type: toPersistType(v.Type()),
		Val:  toPersistVal(v.Val()),
	}
}

type persistFunc struct {
	Name string      `json:"name"`
	Sig  interface{} `json:"sig"`
}

func toPersistFunc(v *types.Func) *persistFunc {
	sig := toPersistSignature(v.Type().(*types.Signature))
	if sig == nil {
		return nil
	}
	return &persistFunc{Name: v.Name(), Sig: sig}
}

type persistNamed struct {
	Name       string        `json:"name"`
	Underlying interface{}   `json:"underlying,omitempty"` // only for named type
	Type       interface{}   `json:"type,omitempty"`       // only for alias type
	Methods    []persistFunc `json:"methods,omitempty"`
	IsAlias    bool          `json:"alias,omitempty"`
}

func toPersistTypeName(v *types.TypeName) persistNamed {
	if v.IsAlias() {
		typ := toPersistType(v.Type())
		return persistNamed{IsAlias: true, Name: v.Name(), Type: typ}
	}
	var methods []persistFunc
	t := v.Type().(*types.Named)
	if n := t.NumMethods(); n > 0 {
		methods = make([]persistFunc, n)
		for i := 0; i < n; i++ {
			if mthd := toPersistFunc(t.Method(i)); mthd != nil {
				methods[i] = *mthd
			}
		}
	}
	return persistNamed{
		Name:       v.Name(),
		Underlying: toPersistType(t.Underlying()),
		Methods:    methods,
	}
}

/*
func fromPersistTypeName(v persistNamed) *types.TypeName {
	panic("not impl")
}
*/

// ----------------------------------------------------------------------------

type persistPkgRef struct {
	ID      string         `json:"id"`
	PkgPath string         `json:"pkgPath"`
	Name    string         `json:"name,omitempty"`
	Vars    []persistVar   `json:"vars,omitempty"`
	Consts  []persistConst `json:"consts,omitempty"`
	Types   []persistNamed `json:"types,omitempty"`
	Funcs   []persistFunc  `json:"funcs,omitempty"`
}

func toPersistPkg(pkg *PkgRef) *persistPkgRef {
	pkgTypes := pkg.Types
	if pkgTypes == types.Unsafe {
		return &persistPkgRef{ID: pkg.ID, PkgPath: "unsafe"}
	}
	var (
		vars   []persistVar
		consts []persistConst
		typs   []persistNamed
		funcs  []persistFunc
	)
	scope := pkgTypes.Scope()
	for _, name := range scope.Names() {
		o := scope.Lookup(name)
		// export private types because they may be refered by public functions
		if v, ok := o.(*types.TypeName); ok {
			if _, ok := v.Type().(*overloadFuncType); !ok {
				typs = append(typs, toPersistTypeName(v))
			}
			continue
		}
		if !token.IsExported(name) {
			continue
		}
		switch v := o.(type) {
		case *types.Func:
			funcs = append(funcs, *toPersistFunc(v))
		case *types.Const:
			consts = append(consts, toPersistConst(v))
		case *types.Var:
			vars = append(vars, toPersistVar(v))
		default:
			log.Panicln("unexpected object -", reflect.TypeOf(o), o.Name())
		}
	}
	return &persistPkgRef{
		ID:      pkg.ID,
		PkgPath: pkgTypes.Path(),
		Name:    pkgTypes.Name(),
		Vars:    vars,
		Types:   typs,
		Funcs:   funcs,
		Consts:  consts,
	}
}

func fromPersistPkg(pkg *persistPkgRef) *PkgRef {
	panic("not impl")
}

// ----------------------------------------------------------------------------

func toPersistPkgs(imports map[string]*PkgRef) map[string]*persistPkgRef {
	ret := make(map[string]*persistPkgRef, len(imports))
	for pkgPath, pkg := range imports {
		ret[pkgPath] = toPersistPkg(pkg)
	}
	return ret
}

func fromPersistPkgs(from map[string]*persistPkgRef) map[string]*PkgRef {
	imports := make(map[string]*PkgRef, len(from))
	for pkgPath, pkg := range from {
		imports[pkgPath] = fromPersistPkg(pkg)
	}
	return imports
}

// ----------------------------------------------------------------------------

func savePkgsCache(file string, imports map[string]*PkgRef) (err error) {
	ret := toPersistPkgs(imports)
	buf := new(bytes.Buffer)
	zipf := zip.NewWriter(buf)
	zipw, _ := zipf.Create("go.json")
	err = json.NewEncoder(zipw).Encode(ret)
	if err != nil {
		return
	}
	zipf.Close()
	return ioutil.WriteFile(file, buf.Bytes(), 0666)
}

func loadPkgsCacheFrom(file string) map[string]*PkgRef {
	f, err := os.Open(file)
	if err == nil {
		defer f.Close()
		var ret map[string]*persistPkgRef
		err = json.NewDecoder(f).Decode(&ret)
		if err != nil {
			log.Println("[WARN] loadPkgsCache failed:", err)
		}
		return fromPersistPkgs(ret)
	}
	return make(map[string]*PkgRef)
}

// ----------------------------------------------------------------------------
