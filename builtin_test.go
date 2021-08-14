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
	udt := 0
	if _, ok := checkUdt(o, &udt); ok {
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
