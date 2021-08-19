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
	"reflect"
	"strconv"
)

type pobj = map[string]interface{}

// ----------------------------------------------------------------------------

func toPersistNamedType(t *types.Named) interface{} {
	o := t.Obj()
	if o.IsAlias() {
		return toPersistType(o.Type())
	}
	ret := pobj{"type": "named", "name": o.Name()}
	if pkg := o.Pkg(); pkg != nil {
		ret["pkg"] = pkg.Path()
	}
	return ret
}

func fromPersistNamedType(ctx *persistPkgCtx, t pobj) *types.Named {
	name := t["name"].(string)
	if v, ok := t["pkg"]; ok {
		return ctx.ref(v.(string), name)
	}
	o := types.Universe.Lookup(name)
	return o.Type().(*types.Named)
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

func fromPersistInterface(ctx *persistPkgCtx, t pobj) *types.Interface {
	methods := t["methods"].([]interface{})
	mthds := fromPersistInterfaceMethods(ctx, methods)
	var embeddeds []types.Type
	if v, ok := t["embeddeds"]; ok {
		in := v.([]interface{})
		embeddeds = make([]types.Type, len(in))
		for i, t := range in {
			embeddeds[i] = fromPersistType(ctx, t)
		}
	}
	return types.NewInterfaceType(mthds, embeddeds)
}

func toPersistStruct(t *types.Struct) interface{} {
	n := t.NumFields()
	fields := make([]persistVar, n)
	for i := 0; i < n; i++ {
		fields[i] = toPersistField(t.Field(i))
		fields[i].Tag = t.Tag(i)
	}
	return pobj{"type": "struct", "fields": fields}
}

func fromPersistStruct(ctx *persistPkgCtx, t pobj) *types.Struct {
	in := t["fields"].([]interface{})
	fields := make([]*types.Var, len(in))
	tags := make([]string, len(in))
	for i, v := range in {
		fields[i] = fromPersistField(ctx, v)
		if tag, ok := v.(pobj)["tag"]; ok {
			tags[i] = tag.(string)
		}
	}
	return types.NewStruct(fields, tags)
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
		ret["recv"] = toPersistParam(recv)
	}
	if variadic {
		ret["variadic"] = true
	}
	return ret
}

func fromPersistSignature(ctx *persistPkgCtx, v interface{}) *types.Signature {
	sig := v.(pobj)
	if sig["type"].(string) != "sig" {
		panic("unexpected signature")
	}
	params := fromPersistVars(ctx, sig["params"].([]interface{}))
	results := fromPersistVars(ctx, sig["results"].([]interface{}))
	var variadic bool
	if v, ok := sig["variadic"]; ok {
		variadic = v.(bool)
	}
	var recv *types.Var
	if v, ok := sig["recv"]; ok {
		recv = fromPersistParam(ctx, v)
	}
	return types.NewSignature(recv, params, results, variadic)
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

func fromPersistType(ctx *persistPkgCtx, typ interface{}) types.Type {
	switch t := typ.(type) {
	case string:
		if ret := types.Universe.Lookup(t); ret != nil {
			return ret.Type()
		} else if kind, ok := typsUntyped[t]; ok {
			return types.Typ[kind]
		}
	case pobj:
		switch t["type"].(string) {
		case "slice":
			elem := fromPersistType(ctx, t["elem"])
			return types.NewSlice(elem)
		case "map":
			key := fromPersistType(ctx, t["key"])
			elem := fromPersistType(ctx, t["elem"])
			return types.NewMap(key, elem)
		case "named":
			return fromPersistNamedType(ctx, t)
		case "ptr":
			elem := fromPersistType(ctx, t["elem"])
			return types.NewPointer(elem)
		case "interface":
			return fromPersistInterface(ctx, t)
		case "struct":
			return fromPersistStruct(ctx, t)
		case "sig":
			return fromPersistSignature(ctx, t)
		case "array":
			iv, err := strconv.ParseInt(t["len"].(string), 10, 64)
			if err == nil {
				elem := fromPersistType(ctx, t["elem"])
				return types.NewArray(elem, iv)
			}
		case "chan":
			elem := fromPersistType(ctx, t["elem"])
			return types.NewChan(types.ChanDir(t["dir"].(float64)), elem)
		}
	}
	panic("unexpected type")
}

var typsUntyped = map[string]types.BasicKind{
	"untyped int":     types.UntypedInt,
	"untyped float":   types.UntypedFloat,
	"untyped string":  types.UntypedString,
	"untyped bool":    types.UntypedBool,
	"untyped rune":    types.UntypedRune,
	"untyped complex": types.UntypedComplex,
	"untyped nil":     types.UntypedNil,
	"Pointer":         types.UnsafePointer,
}

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

func fromPersistVal(val interface{}) constant.Value {
	var ret interface{}
	switch v := val.(type) {
	case string:
		ret = v
	case pobj:
		switch typ := v["type"].(string); typ {
		case "int64":
			iv, err := strconv.ParseInt(v["val"].(string), 10, 64)
			if err == nil {
				ret = iv
			}
		case "bigint":
			bval := v["val"].(string)
			if bv, ok := new(big.Int).SetString(bval, 10); ok {
				ret = bv
			}
		case "bigrat":
			num := v["num"].(string)
			denom := v["denom"].(string)
			bnum, ok1 := new(big.Int).SetString(num, 10)
			bdenom, ok2 := new(big.Int).SetString(denom, 10)
			if ok1 && ok2 {
				ret = new(big.Rat).SetFrac(bnum, bdenom)
			}
		}
	case bool:
		ret = v
	}
	if ret != nil {
		return constant.Make(ret)
	}
	panic("unsupported constant")
}

// ----------------------------------------------------------------------------

type persistVar struct {
	Name     string      `json:"name,omitempty"`
	Type     interface{} `json:"type"`
	Tag      string      `json:"tag,omitempty"`
	Embedded bool        `json:"embedded,omitempty"`
}

func toPersistParam(v *types.Var) persistVar {
	return persistVar{
		Name: v.Name(),
		Type: toPersistType(v.Type()),
	}
}

func toPersistField(v *types.Var) persistVar {
	return persistVar{
		Name:     v.Name(),
		Type:     toPersistType(v.Type()),
		Embedded: v.Embedded(),
	}
}

func fromPersistVarDecl(ctx *persistPkgCtx, v persistVar) {
	typ := fromPersistType(ctx, v.Type)
	o := types.NewVar(token.NoPos, ctx.pkg, v.Name, typ)
	ctx.scope.Insert(o)
}

func fromPersistParam(ctx *persistPkgCtx, v interface{}) *types.Var {
	obj := v.(pobj)
	var name string
	if o, ok := obj["name"]; ok {
		name = o.(string)
	}
	typ := fromPersistType(ctx, obj["type"])
	return types.NewParam(token.NoPos, ctx.pkg, name, typ)
}

func fromPersistField(ctx *persistPkgCtx, v interface{}) *types.Var {
	obj := v.(pobj)
	var name string
	if o, ok := obj["name"]; ok {
		name = o.(string)
	}
	typ := fromPersistType(ctx, obj["type"])
	_, embedded := obj["embedded"]
	return types.NewField(token.NoPos, ctx.pkg, name, typ, embedded)
}

func toPersistVars(v *types.Tuple) []persistVar {
	n := v.Len()
	vars := make([]persistVar, n)
	for i := 0; i < n; i++ {
		vars[i] = toPersistParam(v.At(i))
	}
	return vars
}

func fromPersistVars(ctx *persistPkgCtx, vars []interface{}) *types.Tuple {
	n := len(vars)
	if n == 0 {
		return nil
	}
	ret := make([]*types.Var, n)
	for i, v := range vars {
		ret[i] = fromPersistParam(ctx, v)
	}
	return types.NewTuple(ret...)
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

func fromPersistConst(ctx *persistPkgCtx, v persistConst) {
	typ := fromPersistType(ctx, v.Type)
	val := fromPersistVal(v.Val)
	o := types.NewConst(token.NoPos, ctx.pkg, v.Name, typ, val)
	ctx.scope.Insert(o)
}

// ----------------------------------------------------------------------------

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

func fromPersistFunc(ctx *persistPkgCtx, v persistFunc) *types.Func {
	sig := fromPersistSignature(ctx, v.Sig)
	return types.NewFunc(token.NoPos, ctx.pkg, v.Name, sig)
}

func fromPersistInterfaceMethod(ctx *persistPkgCtx, fn interface{}) *types.Func {
	in := fn.(pobj)
	sig := fromPersistSignature(ctx, in["sig"])
	return types.NewFunc(token.NoPos, ctx.pkg, in["name"].(string), sig)
}

func fromPersistInterfaceMethods(ctx *persistPkgCtx, funcs []interface{}) []*types.Func {
	mthds := make([]*types.Func, len(funcs))
	for i, fn := range funcs {
		mthds[i] = fromPersistInterfaceMethod(ctx, fn)
	}
	return mthds
}

func fromPersistMethods(ctx *persistPkgCtx, t *types.Named, funcs []persistFunc) {
	for _, fn := range funcs {
		ret := fromPersistFunc(ctx, fn)
		t.AddMethod(ret)
	}
}

// ----------------------------------------------------------------------------

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
		methods = make([]persistFunc, 0, n)
		for i := 0; i < n; i++ {
			if mthd := toPersistFunc(t.Method(i)); mthd != nil {
				methods = append(methods, *mthd)
			}
		}
	}
	return persistNamed{
		Name:       v.Name(),
		Underlying: toPersistType(t.Underlying()),
		Methods:    methods,
	}
}

func fromPersistTypeName(ctx *persistPkgCtx, v persistNamed) {
	if v.IsAlias {
		typ := fromPersistType(ctx, v.Type)
		o := types.NewTypeName(token.NoPos, ctx.pkg, v.Name, typ)
		ctx.scope.Insert(o)
	} else {
		var o *types.TypeName
		var t *types.Named
		obj := ctx.scope.Lookup(v.Name)
		if obj != nil {
			o = obj.(*types.TypeName)
			t = o.Type().(*types.Named)
		} else {
			o = types.NewTypeName(token.NoPos, ctx.pkg, v.Name, nil)
			t = types.NewNamed(o, nil, nil)
			ctx.scope.Insert(o)
		}
		underlying := fromPersistType(ctx, v.Underlying)
		t.SetUnderlying(underlying)
		fromPersistMethods(ctx, t, v.Methods)
	}
}

// ----------------------------------------------------------------------------

type persistPkgRef struct {
	ID      string         `json:"id"`
	PkgPath string         `json:"pkgPath"`
	Name    string         `json:"name,omitempty"`
	Vars    []persistVar   `json:"vars,omitempty"`
	Consts  []persistConst `json:"consts,omitempty"`
	Types   []persistNamed `json:"types,omitempty"`
	Funcs   []persistFunc  `json:"funcs,omitempty"`
	Files   []string       `json:"files,omitempty"`
	Fingerp string         `json:"fingerp,omitempty"`
}

func toPersistPkg(pkg *PkgRef) *persistPkgRef {
	var (
		vars   []persistVar
		consts []persistConst
		typs   []persistNamed
		funcs  []persistFunc
	)
	pkgTypes := pkg.Types
	scope := pkgTypes.Scope()
	if debugPersistCache {
		log.Println("==> Persist", pkgTypes.Path())
	}
	for _, name := range scope.Names() {
		o := scope.Lookup(name)
		// persist private types because they may be refered by public functions
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
			vars = append(vars, toPersistParam(v))
		default:
			log.Panicln("unexpected object -", reflect.TypeOf(o), o.Name())
		}
	}
	ret := &persistPkgRef{
		ID:      pkg.ID,
		PkgPath: pkgTypes.Path(),
		Name:    pkgTypes.Name(),
		Vars:    vars,
		Types:   typs,
		Funcs:   funcs,
		Consts:  consts,
	}
	if pkg.pkgf != nil {
		ret.Fingerp = pkg.pkgf.getFingerp()
		ret.Files = pkg.pkgf.files
	}
	return ret
}

func fromPersistPkg(ctx *persistPkgCtx, pkg *persistPkgRef) *PkgRef {
	ctx.pkg = types.NewPackage(pkg.PkgPath, pkg.Name)
	ctx.scope = ctx.pkg.Scope()
	ctx.checks = nil
	var pkgf *pkgFingerp
	if pkg.Fingerp != "" {
		pkgf = &pkgFingerp{files: pkg.Files, fingerp: pkg.Fingerp}
	}
	ret := &PkgRef{ID: pkg.ID, Types: ctx.pkg, pkgf: pkgf}
	ctx.imports[pkg.PkgPath] = ret
	for _, typ := range pkg.Types {
		fromPersistTypeName(ctx, typ)
	}
	ctx.check()
	for _, c := range pkg.Consts {
		fromPersistConst(ctx, c)
	}
	for _, v := range pkg.Vars {
		fromPersistVarDecl(ctx, v)
	}
	for _, fn := range pkg.Funcs {
		o := fromPersistFunc(ctx, fn)
		ctx.scope.Insert(o)
	}
	initGopPkg(ctx.pkg)
	return ret
}

// ----------------------------------------------------------------------------

type persistPkgState struct {
	pkg    *types.Package
	scope  *types.Scope
	checks []*types.TypeName
}

type persistPkgCtx struct {
	imports map[string]*PkgRef
	from    map[string]*persistPkgRef
	persistPkgState
}

func (ctx *persistPkgCtx) check() {
	for _, c := range ctx.checks {
		if c.Type().Underlying() == nil {
			panic("type not found - " + ctx.pkg.Path() + "." + c.Name())
		}
	}
}

func (ctx *persistPkgCtx) ref(pkgPath, name string) *types.Named {
	pkg, ok := ctx.imports[pkgPath]
	if !ok {
		persist, ok := ctx.from[pkgPath]
		if !ok {
			panic("unexpected: package not found - " + pkgPath)
		}
		old := ctx.persistPkgState
		pkg = fromPersistPkg(ctx, persist)
		ctx.persistPkgState = old
	}
	if o := pkg.Types.Scope().Lookup(name); o != nil {
		return o.Type().(*types.Named)
	}
	if pkg.Types == ctx.pkg { // maybe not loaded
		o := types.NewTypeName(token.NoPos, pkg.Types, name, nil)
		t := types.NewNamed(o, nil, nil)
		ctx.scope.Insert(o)
		ctx.checks = append(ctx.checks, o)
		return t
	}
	panic("type not found: " + pkgPath + "." + name)
}

func fromPersistPkgs(from map[string]*persistPkgRef) map[string]*PkgRef {
	imports := make(map[string]*PkgRef, len(from))
	ctx := &persistPkgCtx{imports: imports, from: from}
	imports["unsafe"] = &PkgRef{
		ID:    "unsafe",
		Types: types.Unsafe,
	}
	for pkgPath, pkg := range from {
		if _, ok := imports[pkgPath]; ok {
			// already loaded
			continue
		}
		imports[pkgPath] = fromPersistPkg(ctx, pkg)
	}
	return imports
}

func toPersistPkgs(imports map[string]*PkgRef) map[string]*persistPkgRef {
	ret := make(map[string]*persistPkgRef, len(imports))
	for pkgPath, pkg := range imports {
		if pkg.Types == types.Unsafe {
			// don't persist unsafe package
			continue
		}
		ret[pkgPath] = toPersistPkg(pkg)
	}
	return ret
}

// ----------------------------------------------------------------------------

const (
	jsonFile = "go.json"
)

func savePkgsCache(file string, imports map[string]*PkgRef) (err error) {
	ret := toPersistPkgs(imports)
	buf := new(bytes.Buffer)
	zipf := zip.NewWriter(buf)
	zipw, _ := zipf.Create(jsonFile)
	err = json.NewEncoder(zipw).Encode(ret)
	if err != nil {
		return
	}
	zipf.Close()
	return ioutil.WriteFile(file, buf.Bytes(), 0666)
}

func loadPkgsCacheFrom(file string) map[string]*PkgRef {
	zipf, err := zip.OpenReader(file)
	if err == nil {
		defer zipf.Close()
		for _, item := range zipf.File {
			if item.Name == jsonFile {
				f, err := item.Open()
				if err == nil {
					defer f.Close()
					var ret map[string]*persistPkgRef
					if err = json.NewDecoder(f).Decode(&ret); err == nil {
						return fromPersistPkgs(ret)
					}
				}
				log.Println("[WARN] loadPkgsCache failed:", err)
				break
			}
		}
	}
	return make(map[string]*PkgRef)
}

// ----------------------------------------------------------------------------
