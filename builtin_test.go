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
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"testing"
)

func TestContractName(t *testing.T) {
	testcases := []struct {
		Contract
		name string
	}{
		{any, "any"},
		{capable, "capable"},
		{lenable, "lenable"},
		{makable, "makable"},
		{cbool, "bool"},
		{ninteger, "ninteger"},
		{orderable, "orderable"},
		{integer, "integer"},
		{number, "number"},
		{addable, "addable"},
		{comparable, "comparable"},
	}
	for _, c := range testcases {
		if c.String() != c.name {
			t.Fatal("Unexpected contract name:", c.name)
		}
	}
}

func TestContract(t *testing.T) {
	pkg := &Package{}
	at := types.NewPackage("foo", "foo")
	testcases := []struct {
		Contract
		typ    types.Type
		result bool
	}{
		{integer, tyInt, true},
		{makable, types.NewMap(tyInt, tyInt), true},
		{makable, types.NewChan(0, tyInt), true},
		{makable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), tyInt, nil), false},
		{comparable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), tyInt, nil), true},
		{comparable, types.NewSlice(tyInt), false},
		{comparable, types.NewMap(tyInt, tyInt), false},
		{comparable, types.NewChan(0, tyInt), true},
		{comparable, types.NewSignature(nil, nil, nil, false), false},
		{comparable, NewTemplateSignature(nil, nil, nil, nil, false), false},
	}
	for _, c := range testcases {
		if c.Match(pkg, c.typ) != c.result {
			t.Fatalf("%s.Match %v expect %v\n", c.String(), c.typ, c.result)
		}
	}
}

func TestToIndex(t *testing.T) {
	if toIndex('b') != 11 {
		t.Fatal("toIndex('b') != 11")
	}
	defer func() {
		if recover() != "invalid character out of [0-9,a-z]" {
			t.Fatal("toIndex('!') not panic?")
		}
	}()
	toIndex('!')
}

func TestCheckOverloadMethod(t *testing.T) {
	sig := types.NewSignature(nil, nil, nil, false)
	if _, ok := CheckOverloadMethod(sig); ok {
		t.Fatal("TestCheckOverloadMethod failed:")
	}
}

func TestCheckUdt(t *testing.T) {
	o := types.NewNamed(types.NewTypeName(token.NoPos, nil, "foo", nil), types.Typ[types.Int], nil)
	var frs forRangeStmt
	if _, ok := frs.checkUdt(o); ok {
		t.Fatal("findMethod failed: bar exists?")
	}
}

func TestNodeInterp(t *testing.T) {
	interp := nodeInterp{}
	if pos := interp.Position(1); pos.Line != 0 {
		t.Fatal("TestNodeInterp interp.Position failed:", pos)
	}
	if interp.Caller(nil) != "the function call" {
		t.Fatal("TestNodeInterp interp.Caller failed")
	}
	if src, pos := interp.LoadExpr(nil); src != "" || pos.Line != 0 {
		t.Fatal("TestNodeInterp interp.LoadExpr failed:", src, pos)
	}
	var cb CodeBuilder
	if cb.getCaller(nil) != "" {
		t.Fatal("TestNodeInterp cb.getCaller failed")
	}
}

func TestInternalStack(t *testing.T) {
	var cb CodeBuilder
	cb.InternalStack().Push(nil)
	if cb.Get(-1) != nil {
		t.Fatal("InternalStack/Get failed")
	}
}

func TestGetUnderlying(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	cb.loadNamed = func(at *Package, t *types.Named) {
		panic("loadNamed")
	}
	defaultLoadNamed(nil, nil)
	defer func() {
		if e := recover(); e != "loadNamed" {
			t.Fatal("TestGetUnderlying failed")
		}
	}()
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	cb.getUnderlying(named)
}

func TestGetUnderlying2(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	cb.pkg = pkg
	cb.loadNamed = func(at *Package, t *types.Named) {
		panic("loadNamed")
	}
	defaultLoadNamed(nil, nil)
	defer func() {
		if e := recover(); e != "loadNamed" {
			t.Fatal("TestGetUnderlying2 failed")
		}
	}()
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	getUnderlying(pkg, named)
}

func TestWriteFile(t *testing.T) {
	if WriteFile("/", nil, false) == nil {
		t.Fatal("WriteFile: no error?")
	}
}

func TestScopeHasName(t *testing.T) {
	scope := types.NewScope(types.Universe, 0, 0, "")
	child := types.NewScope(scope, 0, 0, "")
	child.Insert(types.NewVar(0, nil, "foo", types.Typ[types.Int]))
	has := scopeHasName(scope, "foo")
	if !has {
		t.Fatal("scopeHasName failed: foo not found?")
	}
}

func TestToPersistNamedType(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	o := types.NewTypeName(token.NoPos, pkg, "bar", types.Typ[types.Int])
	typ := types.NewNamed(o, types.Typ[types.Int], nil)
	val := toPersistNamedType(typ)
	if val != "int" {
		t.Fatal("TestToPersistNamedType:", val)
	}
}

func TestFromPersistStruct(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	ctx := &persistPkgCtx{}
	ctx.pkg = pkg
	val := fromPersistStruct(ctx, pobj{"type": "struct", "fields": []interface{}{
		pobj{"name": "foo", "type": "int", "tag": "hello"},
	}})
	if val.NumFields() != 1 || val.Tag(0) != "hello" {
		t.Fatal("TestFromPersistStruct:", val)
	}
}

func TestPersistSignature(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	recv := types.NewParam(token.NoPos, pkg, "bar", TyEmptyInterface)
	sig := types.NewSignature(recv, nil, nil, false)
	val := toPersistSignature(sig)
	empty := []persistVar{}
	if !reflect.DeepEqual(val, pobj{"type": "sig", "params": empty, "results": empty}) {
		t.Fatal("TestPersistSignature:", val)
	}
	defer func() {
		if e := recover(); e != "unexpected signature" {
			t.Fatal("TestPersistSignature:", e)
		}
	}()
	fromPersistSignature(nil, pobj{"type": "foo"})
}

func TestToPersistType(t *testing.T) {
	defer func() {
		if e := recover(); e != "unsupported type - overloadFuncType{funcs: []}" {
			t.Fatal("TestToPersistType:", e)
		}
	}()
	toPersistType(&overloadFuncType{})
}

func TestFromPersistType(t *testing.T) {
	defer func() {
		if e := recover(); e != "unexpected type" {
			t.Fatal("TestFromPersistType:", e)
		}
	}()
	fromPersistType(nil, 0)
}

func TestImported(t *testing.T) {
	pkg := &PkgRef{
		pkgf: &pkgFingerp{fingerp: "abc"},
	}
	imports := map[string]*PkgRef{"foo": pkg}
	cached := &LoadPkgsCached{imports: imports}
	if cached.Save() != nil {
		t.Fatal("cached.Save failed")
	}
	if _, ok := cached.imported("foo"); ok {
		t.Fatal("TestImported failed")
	}
}

func TestToFields(t *testing.T) {
	pkg := new(Package)
	pkg.Types = types.NewPackage("", "foo")
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "bar", nil), types.Typ[types.Int], nil)
	flds := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "bar", typ, true),
	}
	struc := types.NewStruct(flds, []string{"`bar`"})
	out := toFields(pkg, struc)
	if !(len(out) == 1 && out[0].Names == nil) {
		t.Fatal("TestToFields failed:", out)
	}
}

func TestToVariadic(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestToVariadic: no error?")
		}
	}()
	toVariadic(&ast.Field{Type: &ast.Ident{Name: "int"}})
}

// ----------------------------------------------------------------------------
