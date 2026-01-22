/*
 Copyright 2026 The XGo Authors (xgo.dev)
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

package gogen_test

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gogen"
)

func TestIsTupleType(t *testing.T) {
	var cb gogen.CodeBuilder
	x := types.NewField(token.NoPos, nil, "x", types.Typ[types.Int], false)
	y := types.NewField(token.NoPos, nil, "y", types.Typ[types.Int], false)
	flds := []*types.Var{x, y}
	if cb.IsTupleType(types.NewStruct(flds, nil)) {
		t.Fatal("TestIsTupleType: failed")
	}
}

func TestTupleLit(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "foo", nil, nil, false).BodyStart(pkg).
		Val(1).
		Val(2).
		TupleLit(nil, 2).
		EndStmt().
		End()
	domTest(t, pkg, `package main

func foo() {
	struct {
		X_0 int
		X_1 int
	}{1, 2}
}
`)
}

func TestTupleMember(t *testing.T) {
	pkg := newMainPackage()
	x := types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false)
	y := types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.Int], false)
	typ := pkg.NewTuple(true, x, y)
	pt := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "Point", typ), typ, nil)
	a := types.NewParam(token.NoPos, pkg.Types, "a", typ)
	b := types.NewParam(token.NoPos, pkg.Types, "b", pt)
	typf := types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(b), false)
	f := types.NewParam(token.NoPos, pkg.Types, "f", typf)
	pkg.NewFunc(nil, "foo", types.NewTuple(a, f), nil, false).BodyStart(pkg).
		Val(ctxRef(pkg, "a")).
		MemberRef("x").
		Val(ctxRef(pkg, "a")).
		MemberVal("y").
		Assign(1).
		EndStmt().
		DefineVarStart(token.NoPos, "x", "y").
		Val(ctxRef(pkg, "a")).
		EndInit(1).
		DefineVarStart(token.NoPos, "x2", "y2").
		Val(ctxRef(pkg, "f")).Call(0).
		EndInit(1).
		Debug(func(cb *gogen.CodeBuilder) {
			cb.Val(ctxRef(pkg, "a"))
			cb.Member("unknown", gogen.MemberFlagRef)
			cb.Member("unknown", gogen.MemberFlagVal)
			cb.ResetStmt()
		}).
		End()
	domTest(t, pkg, `package main

func foo(a struct {
	X_0 int
	X_1 int
}, f func() (b Point)) {
	a.X_0 = a.X_1
	x, y := a.X_0, a.X_1
	x2, y2 := func(v Point) (int, int) {
		return v.X_0, v.X_1
	}(f())
}
`)
}

func newFields(names ...string) []*types.Var {
	ret := make([]*types.Var, len(names))
	for i, name := range names {
		ret[i] = types.NewField(token.NoPos, nil, name, types.Typ[types.Int], false)
	}
	return ret
}

func TestCodeBuilder_LookupField(t *testing.T) {
	p := newMainPackage()
	cb := p.CB()
	tests := []struct {
		name string // description of this test case
		t    *types.Struct
		fld  string
		want int
	}{
		{"test1", types.NewStruct(newFields("a"), nil), "b", -1},
		{"test2", types.NewStruct(newFields("a"), nil), "a", 0},
		{"test3", p.NewTuple(true, newFields("a")...), "a", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cb.LookupField(tt.t, tt.fld)
			if got != tt.want {
				t.Errorf("LookupField() = %v, want %v", got, tt.want)
			}
		})
	}
}
