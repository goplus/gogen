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
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"testing"

	"github.com/goplus/gox"
)

var (
	gopNamePrefix = "Gop_"
)

func toIndex(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'z' {
		return int(c - ('a' - 10))
	}
	panic("TODO: invalid character out of [0-9,a-z]")
}

func initGopPkg(pkg *types.Package) {
	scope := pkg.Scope()
	overloads := make(map[string][]types.Object)
	names := scope.Names()
	for _, name := range names {
		if n := len(name); n > 3 && name[n-3:n-1] == "__" { // overload function
			key := name[:n-3]
			overloads[key] = append(overloads[key], scope.Lookup(name))
		}
	}
	for key, items := range overloads {
		off := len(key) + 2
		fns := make([]types.Object, len(items))
		for _, item := range items {
			idx := toIndex(item.Name()[off])
			if idx >= len(items) {
				panic("overload function must be from 0 to N")
			}
			if fns[idx] != nil {
				panic("overload function exists?")
			}
			fns[idx] = item
		}
		scope.Insert(gox.NewOverloadFunc(token.NoPos, pkg, key, fns...))
	}
}

func initGopBuiltin(pkg gox.PkgImporter, conf *gox.Config) {
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	big.EnsureImported()
	conf.UntypedBigInt = big.Ref("Gop_untyped_bigint").Type().(*types.Named)
	conf.UntypedBigRat = big.Ref("Gop_untyped_bigrat").Type().(*types.Named)
	conf.UntypedBigFloat = big.Ref("Gop_untyped_bigfloat").Type().(*types.Named)
	initGopPkg(big.Types)
}

func newGopBuiltinDefault(pkg gox.PkgImporter, prefix string, conf *gox.Config) *types.Package {
	builtin := types.NewPackage("", "")
	initGopBuiltin(pkg, conf)
	gox.InitBuiltinOps(builtin, prefix, conf)
	gox.InitBuiltinFuncs(builtin)
	return builtin
}

func newGopMainPackage() *gox.Package {
	conf := &gox.Config{
		Fset:       gblFset,
		LoadPkgs:   gblLoadPkgs,
		NewBuiltin: newGopBuiltinDefault,
		Prefix:     gopNamePrefix,
	}
	return gox.NewPackage("", "main", conf)
}

// ----------------------------------------------------------------------------

func TestBigRatConstant(t *testing.T) {
	a := constant.Make(new(big.Rat).SetInt64(1))
	b := constant.Make(new(big.Rat).SetInt64(2))
	c := constant.BinaryOp(a, token.ADD, b)
	if c.Kind() != constant.Float {
		t.Fatal("c.Kind() != constant.Float -", c)
	}
	d := constant.BinaryOp(a, token.QUO, b)
	if !constant.Compare(constant.Make(big.NewRat(1, 2)), token.EQL, d) {
		t.Fatal("d != 1/2r")
	}

	e := constant.MakeFromLiteral("1.2", token.FLOAT, 0)
	if _, ok := constant.Val(e).(*big.Rat); !ok {
		t.Fatal("constant.MakeFromLiteral 1.2 not *big.Rat", constant.Val(e))
	}
}

func TestBigIntVar(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.CB().NewVar(big.Ref("Gop_bigint").Type(), "a")
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a builtin.Gop_bigint
`)
}

func TestBigInt(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.CB().NewVar(big.Ref("Gop_bigint").Type(), "a", "b")
	pkg.CB().NewVarStart(big.Ref("Gop_bigint").Type(), "c").
		Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.ADD).EndInit(1)
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a, b builtin.Gop_bigint
var c builtin.Gop_bigint = a.Gop_Add(b)
`)
}

func TestBigRat(t *testing.T) {
	pkg := newGopMainPackage()
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("Gop_bigrat").Type(), "a", "b")
	pkg.CB().NewVarStart(big.Ref("Gop_bigrat").Type(), "c").
		Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.QUO).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("Gop_bigrat").Type(), "d").
		Val(ctxRef(pkg, "a")).UnaryOp(token.SUB).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("Gop_bigrat").Type(), "e").
		Val(big.Ref("Gop_bigrat_Cast")).Call(0).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("Gop_bigrat").Type(), "f").
		Val(big.Ref("Gop_bigrat_Cast")).Val(1).Val(2).Call(2).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("Gop_bigint").Type(), "g")
	pkg.CB().NewVarStart(big.Ref("Gop_bigrat").Type(), "h").
		Val(big.Ref("Gop_bigrat_Cast")).Val(ctxRef(pkg, "g")).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import builtin "github.com/goplus/gox/internal/builtin"

var a, b builtin.Gop_bigrat
var c builtin.Gop_bigrat = a.Gop_Quo(b)
var d builtin.Gop_bigrat = a.Gop_Neg()
var e builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__0()
var f builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__3(1, 2)
var g builtin.Gop_bigint
var h builtin.Gop_bigrat = builtin.Gop_bigrat_Cast__1(g)
`)
}

func TestUntypedBigIntAdd(t *testing.T) {
	pkg := newGopMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(6)).
		UntypedBigInt(big.NewInt(63)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigint_Init__1(big.NewInt(69))
`)
}

func TestUntypedBigIntQuo(t *testing.T) {
	pkg := newGopMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(6)).
		UntypedBigInt(big.NewInt(63)).
		BinaryOp(token.QUO).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigrat_Init__1(big.NewRat(2, 21))
`)
}

func TestUntypedBigIntRem(t *testing.T) {
	pkg := newGopMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(100)).
		UntypedBigInt(big.NewInt(7)).
		BinaryOp(token.REM).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigint_Init__1(big.NewInt(2))
`)
}

func TestUntypedBigRatAdd(t *testing.T) {
	pkg := newGopMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigRat(big.NewRat(1, 3)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigrat_Init__1(big.NewRat(1, 2))
`)
}

func TestUntypedBigRatSub(t *testing.T) {
	pkg := newGopMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigRat(big.NewRat(1, 3)).
		BinaryOp(token.SUB).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigrat_Init__1(big.NewRat(-1, 6))
`)
}

func TestUntypedBigRat(t *testing.T) {
	pkg := newGopMainPackage()
	mbig := pkg.Import("github.com/goplus/gox/internal/builtin")
	pkg.CB().NewVarStart(nil, "a").UntypedBigRat(big.NewRat(6, 63)).EndInit(1)
	pkg.CB().NewVarStart(mbig.Ref("Gop_bigrat").Type(), "b").Val(ctxRef(pkg, "a")).EndInit(1)
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

var a = builtin.Gop_bigrat_Init__1(big.NewRat(2, 21))
var b builtin.Gop_bigrat = a
`)
}

func TestUntypedBigRat2(t *testing.T) {
	pkg := newGopMainPackage()
	one := big.NewInt(1)
	denom := new(big.Int).Lsh(one, 128)
	v := new(big.Rat).SetFrac(one, denom)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a").UntypedBigRat(v).EndInit(1).
		End()
	domTest(t, pkg, `package main

import (
	builtin "github.com/goplus/gox/internal/builtin"
	big "math/big"
)

func main() {
	a := builtin.Gop_bigrat_Init__1(new(big.Rat).SetFrac(big.NewInt(1), func() *big.Int {
		v, _ := new(big.Int).SetString("340282366920938463463374607431768211456", 10)
		return v
	}()))
}
`)
}

// ----------------------------------------------------------------------------
