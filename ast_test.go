//go:build !genjs

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
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gogen/internal/go/format"
)

// TestToRecvTypeGenericAlias covers toRecvType emitting the type parameters of
// a generic alias receiver (e.g. Foo[T]) symmetrically to the *types.Named
// case, for both value and pointer receivers.
func TestToRecvTypeGenericAlias(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	alias := types.NewAlias(types.NewTypeName(token.NoPos, pkg.Types, "Foo", nil), types.Typ[types.Int])
	tp := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T", nil), nil)
	tp.SetConstraint(types.NewInterfaceType(nil, nil).Complete())
	alias.SetTypeParams([]*types.TypeParam{tp})

	fset := token.NewFileSet()
	for _, tc := range []struct {
		typ  types.Type
		want string
	}{
		{alias, "Foo[T]"},
		{types.NewPointer(alias), "*Foo[T]"},
	} {
		b := bytes.NewBuffer(nil)
		format.Node(b, fset, toRecvType(pkg, tc.typ))
		if b.String() != tc.want {
			t.Fatalf("toRecvType = %q, want %q", b.String(), tc.want)
		}
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

func TestToTypeAlias(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	alias := types.NewAlias(types.NewTypeName(token.NoPos, nil, "Int", nil), types.Typ[types.Int])
	expr := toType(pkg, alias)
	if ident, ok := expr.(*ast.Ident); !ok || ident.Name != "Int" {
		t.Fatalf("bad alias %#v", expr)
	}
}

func Test_embedName(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		typ  types.Type
		want string
	}{
		{
			name: "basic type",
			typ:  types.Typ[types.Int],
			want: "int",
		},
		{
			name: "named type",
			typ:  types.NewNamed(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int], nil),
			want: "MyInt",
		},
		{
			name: "pointer to named type",
			typ:  types.NewPointer(types.NewNamed(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int], nil)),
			want: "MyInt",
		},
		{
			name: "alias type",
			typ:  types.NewAlias(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int]),
			want: "MyInt",
		},
		{
			name: "pointer to alias type",
			typ:  types.NewPointer(types.NewAlias(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int])),
			want: "MyInt",
		},
		{
			name: "struct type (anonymous)",
			typ:  types.NewStruct(nil, nil),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := embedName(tt.typ)
			if got != tt.want {
				t.Errorf("embedName() = %v, want %v", got, tt.want)
			}
		})
	}
}
