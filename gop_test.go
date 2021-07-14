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

package gox_test

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

func initGopBuiltin(pkg gox.PkgImporter, builtin *types.Package) {
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	big.Ref("Gop_bigint")
	scope := big.Types.Scope()
	for i, n := 0, scope.Len(); i < n; i++ {
		names := scope.Names()
		for _, name := range names {
			builtin.Scope().Insert(scope.Lookup(name))
		}
	}
}

func newGopBuiltinDefault(pkg gox.PkgImporter, prefix string, contracts *gox.BuiltinContracts) *types.Package {
	builtin := types.NewPackage("", "")
	initGopBuiltin(pkg, builtin)
	gox.InitBuiltinOps(builtin, prefix, contracts)
	gox.InitBuiltinFuncs(builtin)
	return builtin
}

func newGopMainPackage() *gox.Package {
	conf := &gox.Config{
		Fset:       gblFset,
		LoadPkgs:   gblLoadPkgs,
		NewBuiltin: newGopBuiltinDefault,
	}
	return gox.NewPackage("", "main", conf)
}

// ----------------------------------------------------------------------------

func TestBigIntVar(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.NewVar(big.Ref("Gop_bigint").Type(), "a")
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a builtin.Gop_bigint
`)
}

func _TestBigIntAdd(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.NewVar(big.Ref("Gop_bigint").Type(), "a", "b")
	pkg.NewVarStart(big.Ref("Gop_bigint").Type(), "c").
		Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "a")).BinaryOp(token.ADD).EndInit(1)
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a, b builtin.Gop_bigint
var c builtin.Gop_bigint = a.Gop_Add(b)
`)
}

// ----------------------------------------------------------------------------
