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
	"go/token"
	"go/types"
	"testing"
)

func TestBasic(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	if pkg.Ref("main") == nil {
		t.Fatal("main not found")
	}
	domTest(t, pkg, `package main

func main()
`)
}

func TestZeroLitAlias(t *testing.T) {
	pkg := newPackage("main")
	bar := pkg.AliasType("bar", types.Typ[types.Float64])
	results := types.NewTuple(types.NewVar(token.NoPos, pkg.Types, "", bar))
	pkg.NewFunc(nil, "foo", nil, results, false).BodyStart(pkg).
		ZeroLit(bar).Return(1).End()
	domTest(t, pkg, `package main

type bar = float64

func foo() bar
`)
}
