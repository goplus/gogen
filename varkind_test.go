// Copyright 2021 The XGo Authors (xgo.dev)
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build go1.25

package gogen_test

import (
	"go/types"
	"testing"

	"github.com/goplus/gogen"
)

func TestVarKind(t *testing.T) {
	pkg := newPackage("foo")
	kinds := []gogen.VarKind{
		gogen.PackageVar,
		gogen.LocalVar,
		gogen.RecvVar,
		gogen.ParamVar,
		gogen.FieldVar,
		gogen.ParamOptionalVar,
	}
	v := pkg.NewParam(0, "v", types.Typ[types.Int])
	for _, kind := range kinds {
		pkg.SetVarKind(v, kind)
		if pkg.VarKind(v).String() != kind.String() {
			t.Fatal("bad kind string")
		}
		if types.VarKind(pkg.VarKind(v)) != v.Kind() {
			t.Fatal("bad kind")
		}
	}
	if gogen.VarKind(gogen.FieldVar+1).String() != types.VarKind(gogen.FieldVar+1).String() {
		t.Fatal("bad string")
	}
}

func TestRecvVarKind(t *testing.T) {
	pkg := newPackage("foo")
	typ := pkg.NewType("T").InitType(pkg, types.Typ[types.Int])
	recv := pkg.NewRecv(0, "this", typ)
	if recv.Kind() != types.RecvVar {
		t.Fatal("error", recv.Kind())
	}
}

func TestResultVarKind(t *testing.T) {
	pkg := newPackage("foo")
	typ := pkg.NewType("T").InitType(pkg, types.Typ[types.Int])
	recv := pkg.NewResult(0, "this", typ)
	if recv.Kind() != types.ResultVar {
		t.Fatal("error", recv.Kind())
	}
}
