//go:build genjs
// +build genjs

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
	"bytes"
	"go/token"
	"go/types"
	"log"
	"testing"

	"github.com/goplus/gogen"
)

func domTestJs(t *testing.T, pkg *gogen.Package, expectedGo, expectedJs string) {
	doDomTest(t, pkg, expectedGo, "")
	var b bytes.Buffer
	t.Helper()
	err := pkg.WriteJSTo(&b, "")
	if err != nil {
		t.Fatal("pkg.WriteJSTo failed:", err)
	}
	result := b.String()
	if result != expectedJs {
		t.Fatalf("\nWriteJSTo Result:\n%s\nExpected:\n%s\n", result, expectedJs)
	}
	log.Printf("====================== %s End =========================\n", t.Name())
}

func TestBasic(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	if pkg.Ref("main") == nil {
		t.Fatal("main not found")
	}
	domTestJs(t, pkg, `package main

func main()
`, `function main() {
}
`)
}

func TestZeroLitAlias(t *testing.T) {
	pkg := newPackage("main")
	bar := pkg.AliasType("bar", types.Typ[types.Float64])
	results := types.NewTuple(types.NewVar(token.NoPos, pkg.Types, "", bar))
	pkg.NewFunc(nil, "foo", nil, results, false).BodyStart(pkg).
		ZeroLit(bar).Return(1).End()
	domTestJs(t, pkg, `package main

type bar = float64

func foo() bar
`, `function foo() {
	return 0
}
`)
}
