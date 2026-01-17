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

package gogen_test

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"testing"

	"github.com/goplus/gogen"
)

func initXGoBuiltin(big gogen.PkgRef, conf *gogen.Config) {
	conf.UntypedBigInt = big.Ref("XGo_untyped_bigint").Type().(*types.Named)
	conf.UntypedBigRat = big.Ref("XGo_untyped_bigrat").Type().(*types.Named)
	conf.UntypedBigFloat = big.Ref("XGo_untyped_bigfloat").Type().(*types.Named)
}

func newXGoBuiltinDefault(pkg *gogen.Package, conf *gogen.Config) *types.Package {
	fmt := pkg.Import("fmt")
	b := pkg.Import("github.com/goplus/gogen/internal/builtin")
	builtin := types.NewPackage("", "")
	if builtin.Scope().Insert(gogen.NewOverloadFunc(token.NoPos, builtin, "println", fmt.Ref("Println"))) != nil {
		panic("println exists")
	}
	gogen.InitBuiltin(pkg, builtin, conf)
	initXGoBuiltin(b, conf)
	tiStr := pkg.BuiltinTI(types.Typ[types.String])
	tiStr.AddMethods(
		&gogen.BuiltinMethod{Name: "Capitalize", Fn: b.Ref("Capitalize")},
	)
	return builtin
}

func newXGoMainPackage() *gogen.Package {
	conf := &gogen.Config{
		Fset:       gblFset,
		Importer:   gblImp,
		NewBuiltin: newXGoBuiltinDefault,
	}
	return gogen.NewPackage("", "main", conf)
}

// ----------------------------------------------------------------------------

func TestConvertToClosure(t *testing.T) {
	pkg := newMainPackage()
	p := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "x").Val(1, &ast.Ident{
		Name:    "x",
		NamePos: token.Pos(1),
	}).ConvertToClosure()

	// check the closure has src node
	arg := pkg.CB().InternalStack().Get(-1)
	if arg.Src == nil {
		t.Fatal("closure has no src node")
	}

	p.EndInit(1).End()

	domTest(t, pkg, `package main

func main() {
	x := func() int {
		return 1
	}
}
`)
}

func TestXGooConst(t *testing.T) {
	pkg := newPackage("foo", false)
	pkg.CB().NewConstStart(nil, "XGoo_x").
		Val("Hello").EndInit(1)
	domTest(t, pkg, `package foo

const XGoPackage = true
const XGoo_x = "Hello"
`)
}

func TestFmtPrintln(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(token.NoPos, "p").Val(ctxRef(pkg, "println")).EndInit(1).
		VarRef(ctxRef(pkg, "p")).Val(ctxRef(pkg, "println")).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import "fmt"

func main() {
	p := fmt.Println
	p = fmt.Println
}
`)
}

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
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVar(big.Ref("XGo_bigint").Type(), "a")
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a builtin.XGo_bigint
`)
}

func TestBigIntVarInit(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(mbig.Ref("XGo_bigint").Type(), "a").
		UntypedBigInt(big.NewInt(6)).EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.XGo_bigint = builtin.XGo_bigint_Init__1(big.NewInt(6))
`)
}

func TestBigInt(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVar(big.Ref("XGo_bigint").Type(), "a", "b")
	pkg.CB().NewVarStart(big.Ref("XGo_bigint").Type(), "c").
		VarVal("a").VarVal("b").BinaryOp(token.ADD).EndInit(1)
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a, b builtin.XGo_bigint
var c builtin.XGo_bigint = (builtin.XGo_bigint).XGo_Add(a, b)
`)
}

func TestBigInt2(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	typ := types.NewPointer(big.Ref("XGo_bigint").Type())
	pkg.CB().NewVar(typ, "a", "b")
	pkg.CB().NewVarStart(typ, "c").
		VarVal("a").VarVal("b").BinaryOp(token.AND_NOT).EndInit(1)
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a, b *builtin.XGo_bigint
var c *builtin.XGo_bigint = (*builtin.XGo_bigint).XGo_AndNot__0(a, b)
`)
}

func TestBigRat(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a", "b")
	pkg.CB().NewVarStart(big.Ref("XGo_bigrat").Type(), "c").
		VarVal("a").VarVal("b").BinaryOp(token.QUO).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("XGo_bigrat").Type(), "d").
		VarVal("a").UnaryOp(token.SUB).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("XGo_bigrat").Type(), "e").
		Val(big.Ref("XGo_bigrat_Cast")).Call(0).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("XGo_bigrat").Type(), "f").
		Val(big.Ref("XGo_bigrat_Cast")).Val(1).Val(2).Call(2).EndInit(1)
	pkg.CB().NewVarStart(big.Ref("XGo_bigint").Type(), "g")
	pkg.CB().NewVarStart(big.Ref("XGo_bigrat").Type(), "h").
		Val(big.Ref("XGo_bigrat_Cast")).Val(ctxRef(pkg, "g")).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a, b builtin.XGo_bigrat
var c builtin.XGo_bigrat = (builtin.XGo_bigrat).XGo_Quo(a, b)
var d builtin.XGo_bigrat = a.XGo_Neg()
var e builtin.XGo_bigrat = builtin.XGo_bigrat_Cast__5()
var f builtin.XGo_bigrat = builtin.XGo_bigrat_Cast__3(1, 2)
var g builtin.XGo_bigint
var h builtin.XGo_bigrat = builtin.XGo_bigrat_Cast__1(g)
`)
}

func TestBigRatInit(t *testing.T) {
	pkg := newXGoMainPackage()
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(ng.Ref("XGo_bigrat").Type(), "a").
		Val(1).Val(65).BinaryOp(token.SHL).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.XGo_bigrat = builtin.XGo_bigrat_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("36893488147419103232", 10)
	return v
}())
`)
}

func TestBigRatInit2(t *testing.T) {
	pkg := newXGoMainPackage()
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(ng.Ref("XGo_bigrat").Type(), "a").
		Val(-1).Val(65).BinaryOp(token.SHL).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.XGo_bigrat = builtin.XGo_bigrat_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("-36893488147419103232", 10)
	return v
}())
`)
}

func TestBigRatCast(t *testing.T) {
	pkg := newXGoMainPackage()
	fmt := pkg.Import("fmt")
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).
		Val(ng.Ref("XGo_bigrat")).Val(1).Val(65).BinaryOp(token.SHL).Call(1).           // bigrat(1 << 65)
		Typ(types.Typ[types.Float64]).Val(ng.Ref("XGo_bigrat")).Call(0).Call(1).        // float64(bigrat())
		Typ(types.Typ[types.Float64]).Val(ng.Ref("XGo_bigint")).Val(1).Call(1).Call(1). // float64(bigint(1))
		Typ(types.Typ[types.Int]).Call(0).                                              // int()
		Call(4).EndStmt().
		End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	fmt.Println(builtin.XGo_bigrat_Cast__0(func() *big.Int {
		v, _ := new(big.Int).SetString("36893488147419103232", 10)
		return v
	}()), builtin.XGo_bigrat_Cast__5().XGo_Rcast__2(), builtin.XGo_bigint_Cast__0(1).XGo_Rcast(), 0)
}
`)
}

func TestCastIntTwoValue(t *testing.T) {
	pkg := newXGoMainPackage()
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "v", "inRange").
		Typ(types.Typ[types.Int]).
		Val(ng.Ref("XGo_bigrat")).Val(1).Call(1).
		CallWith(1, gogen.InstrFlagTwoValue).
		EndInit(1).
		End()
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	v, inRange := builtin.XGo_bigrat_Cast__0(big.NewInt(1)).XGo_Rcast__0()
}
`)
}

func TestCastBigIntTwoValue(t *testing.T) {
	pkg := newXGoMainPackage()
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "v", "inRange").
		Val(ng.Ref("XGo_bigint")).
		Val(ng.Ref("XGo_bigrat")).Val(1).Call(1).
		CallWith(1, gogen.InstrFlagTwoValue).
		EndInit(1).
		End()
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	v, inRange := builtin.XGo_bigint_Cast__7(builtin.XGo_bigrat_Cast__0(big.NewInt(1)))
}
`)
}

func TestErrCast(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestErrCast: no error?")
		}
	}()
	pkg := newXGoMainPackage()
	ng := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "v", "inRange").
		Typ(types.Typ[types.Int64]).
		Val(ng.Ref("XGo_bigrat")).Val(1).Call(1).
		CallWith(1, gogen.InstrFlagTwoValue).
		EndInit(1).
		End()
}

func TestUntypedBigIntAdd(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(6)).
		UntypedBigInt(big.NewInt(63)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigint_Init__1(big.NewInt(69))
`)
}

func TestBigRatIncDec(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef("a").IncDec(token.INC).
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a builtin.XGo_bigrat

func main() {
	a.XGo_Inc()
}
`)
}

func TestErrValRef(t *testing.T) {
	defer func() {
		if e := recover(); e == nil ||
			e.(error).Error() != "-:  is not a variable" {
			t.Fatal("TestErrValRef:", e)
		}
	}()
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef("a").
		End()
}

func TestErrBigRatIncDec(t *testing.T) {
	defer func() {
		if e := recover(); e == nil ||
			e.(error).Error() != "-: operator XGo_Dec should return no results\n" {
			t.Fatal("TestErrBigRatIncDec:", e)
		}
	}()
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef("a").IncDec(token.DEC).
		End()
}

func TestErrBigRatAssignOp(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestErrBigRatAssignOp: no error?")
		}
	}()
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a", "b")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(ctxRef(pkg, "a")).
		VarVal("b").
		AssignOp(token.SUB_ASSIGN).
		End()
}

func TestBigRatAssignOp(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a", "b")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(ctxRef(pkg, "a")).
		VarVal("b").
		AssignOp(token.ADD_ASSIGN).
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a, b builtin.XGo_bigrat

func main() {
	a.XGo_AddAssign(b)
}
`)
}

func TestBigRatAssignOp2(t *testing.T) {
	pkg := newXGoMainPackage()
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, big.Ref("XGo_bigrat").Type(), "a")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(ctxRef(pkg, "a")).
		Val(1).
		AssignOp(token.ADD_ASSIGN).
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a builtin.XGo_bigrat

func main() {
	a.XGo_AddAssign(builtin.XGo_bigrat_Init__0(1))
}
`)
}

func TestUntypedBigIntQuo(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(6)).
		UntypedBigInt(big.NewInt(63)).
		BinaryOp(token.QUO).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(2, 21))
`)
}

func TestUntypedBigIntQuo2(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		Val(6).
		UntypedBigInt(big.NewInt(63)).
		BinaryOp(token.QUO).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(2, 21))
`)
}

func TestUntypedBigIntQuo3(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(63)).
		Val(6).
		BinaryOp(token.QUO).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(21, 2))
`)
}

func TestUntypedBigIntRem(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(100)).
		UntypedBigInt(big.NewInt(7)).
		BinaryOp(token.REM).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigint_Init__1(big.NewInt(2))
`)
}

func TestUntypedBigIntShift(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(1)).
		Val(128).
		BinaryOp(token.SHL).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigint_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("340282366920938463463374607431768211456", 10)
	return v
}())
`)
}

func TestUntypedBigRatAdd(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigRat(big.NewRat(1, 3)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(1, 2))
`)
}

func TestUntypedBigRatAdd2(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigInt(big.NewInt(3)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(19, 6))
`)
}

func TestUntypedBigRatAdd3(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigInt(big.NewInt(3)).
		UntypedBigRat(big.NewRat(1, 6)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(19, 6))
`)
}

func TestUntypedBigRatAdd4(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, mbig.Ref("XGo_bigrat").Type(), "a")
	pkg.CB().NewVarStart(nil, "b").
		VarVal("a").
		UntypedBigRat(big.NewRat(1, 6)).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.XGo_bigrat
var b = (builtin.XGo_bigrat).XGo_Add(a, builtin.XGo_bigrat_Init__2(big.NewRat(1, 6)))
`)
}

func TestUntypedBigRatAdd5(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, mbig.Ref("XGo_bigrat").Type(), "a")
	pkg.CB().NewVarStart(nil, "b").
		VarVal("a").
		Val(100).
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a builtin.XGo_bigrat
var b = (builtin.XGo_bigrat).XGo_Add(a, builtin.XGo_bigrat_Init__0(100))
`)
}

func TestUntypedBigRatAdd6(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, mbig.Ref("XGo_bigrat").Type(), "a")
	pkg.CB().NewVarStart(nil, "b").
		Val(100).
		VarVal("a").
		BinaryOp(token.ADD).
		EndInit(1)
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/builtin"

var a builtin.XGo_bigrat
var b = (builtin.XGo_bigrat).XGo_Add(builtin.XGo_bigrat_Init__0(100), a)
`)
}

func TestUntypedBigRatSub(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigRat(big.NewRat(1, 3)).
		BinaryOp(token.SUB).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(-1, 6))
`)
}

func TestUntypedBigRatSub2(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.NewVar(token.NoPos, mbig.Ref("XGo_bigrat").Type(), "a")
	pkg.CB().NewVarStart(nil, "b").
		VarVal("a").
		UntypedBigRat(big.NewRat(1, 6)).
		BinaryOp(token.SUB).
		EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.XGo_bigrat
var b = (builtin.XGo_bigrat).XGo_Sub__0(a, builtin.XGo_bigrat_Init__2(big.NewRat(1, 6)))
`)
}

func TestUntypedBigRatLT(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.CB().NewVarStart(nil, "a").
		UntypedBigRat(big.NewRat(1, 6)).
		UntypedBigRat(big.NewRat(1, 3)).
		BinaryOp(token.LSS).
		EndInit(1)
	domTest(t, pkg, `package main

var a = true
`)
}

func TestUntypedBigRat(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(nil, "a").UntypedBigRat(big.NewRat(6, 63)).EndInit(1)
	pkg.CB().NewVarStart(mbig.Ref("XGo_bigrat").Type(), "b").VarVal("a").EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigrat_Init__2(big.NewRat(2, 21))
var b builtin.XGo_bigrat = a
`)
}

func TestUntypedBigRat2(t *testing.T) {
	pkg := newXGoMainPackage()
	one := big.NewInt(1)
	denom := new(big.Int).Lsh(one, 128)
	v := new(big.Rat).SetFrac(one, denom)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "a").UntypedBigRat(v).EndInit(1).
		End()
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	a := builtin.XGo_bigrat_Init__2(new(big.Rat).SetFrac(big.NewInt(1), func() *big.Int {
		v, _ := new(big.Int).SetString("340282366920938463463374607431768211456", 10)
		return v
	}()))
}
`)
}

// ----------------------------------------------------------------------------

func TestForRangeUDT(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	nodeSet := foo.Ref("NodeSet").Type()
	v := pkg.NewParam(token.NoPos, "v", nodeSet)
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("_", "val").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "val")).Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v foo.NodeSet) {
	for _xgo_it := v.Gop_Enum(); ; {
		var _xgo_ok bool
		_, val, _xgo_ok := _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println(val)
	}
}
`)
}

func TestForRangeUDT2(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Bar").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("_", "val").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "val")).Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Bar) {
	for _xgo_it := v.XGo_Enum(); ; {
		var _xgo_ok bool
		val, _xgo_ok := _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println(val)
	}
}
`)
}

func TestForRangeUDT3_WithAssign(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Bar").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		NewVar(types.Typ[types.String], "val").
		ForRange().VarRef(ctxRef(pkg, "val")).Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "val")).Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Bar) {
	var val string
	for _xgo_it := v.XGo_Enum(); ; {
		var _xgo_ok bool
		val, _xgo_ok = _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println(val)
	}
}
`)
}

// bugfix: for range udt { ... }
func TestForRangeUDT3_NoAssign(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Bar").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange().Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Bar) {
	for _xgo_it := v.XGo_Enum(); ; {
		var _xgo_ok bool
		_, _xgo_ok = _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println("Hi")
	}
}
`)
}

// bugfix: for _ = range udt { ... }
// bugfix: [" " for _ <- :10]
func TestForRangeUDT_UNDERLINE(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Bar").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange().VarRef(ctxRef(pkg, "_")).Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Bar) {
	for _xgo_it := v.XGo_Enum(); ; {
		var _xgo_ok bool
		_, _xgo_ok = _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println("Hi")
	}
}
`)
}

// bugfix: for _,_ = range udt { ... }
func TestForRangeUDT_UNDERLINE2(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	nodeSet := foo.Ref("NodeSet").Type()
	v := pkg.NewParam(token.NoPos, "v", nodeSet)
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("_", "_").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v foo.NodeSet) {
	for _xgo_it := v.Gop_Enum(); ; {
		var _xgo_ok bool
		_, _, _xgo_ok = _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println("Hi")
	}
}
`)
}

func TestForRangeUDT4(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Foo").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("elem").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "elem")).Call(1).EndStmt().
		SetBodyHandler(func(body *ast.BlockStmt, kind int) {
			gogen.InsertStmtFront(body, &ast.ExprStmt{X: ast.NewIdent("__sched__")})
		}).
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Foo) {
	v.XGo_Enum(func(elem string) {
		__sched__
		fmt.Println(elem)
	})
}
`)
}

func TestForRangeUDT5(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	bar := foo.Ref("Foo2").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("key", "elem").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "key")).Val(ctxRef(pkg, "elem")).
		Call(2).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/foo"
)

func bar(v *foo.Foo2) {
	v.XGo_Enum(func(key int, elem string) {
		fmt.Println(key, elem)
	})
}
`)
}

func TestForRangeUDT6(t *testing.T) {
	pkg := newMainPackage()
	os := pkg.Import("os")
	file := os.Ref("File").Type()
	v := pkg.NewParam(token.NoPos, "v", types.NewPointer(file))
	pkg.NewFunc(nil, "bar", types.NewTuple(v), nil, false).BodyStart(pkg).
		ForRange("line").Val(v).RangeAssignThen(token.NoPos).
		Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "line")).
		Call(1).EndStmt().
		End().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/iox"
	"os"
)

func bar(v *os.File) {
	for _xgo_it := iox.EnumLines(v); ; {
		var _xgo_ok bool
		line, _xgo_ok := _xgo_it.Next()
		if !_xgo_ok {
			break
		}
		fmt.Println(line)
	}
}
`)
}

// ----------------------------------------------------------------------------

func TestStaticMethod(t *testing.T) {
	pkg := newMainPackage()
	bar := pkg.Import("github.com/goplus/gogen/internal/bar")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Typ(bar.Ref("Game").Type()).MemberVal("New").Call(0).EndStmt().
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/bar"

func main() {
	bar.XGos_Game_New()
}
`)
}

func TestTemplateRecvMethod(t *testing.T) {
	pkg := newMainPackage()
	bar := pkg.Import("github.com/goplus/gogen/internal/bar")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(bar.Ref("Game").Type(), "g").
		VarVal("g").MemberVal("Run").Val("Hi").Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/bar"

func main() {
	var g bar.Game
	bar.Gopt_Game_Run(&g, "Hi")
}
`)
}

func TestTemplateRecvMethod2(t *testing.T) {
	pkg := newMainPackage()
	bar := pkg.Import("github.com/goplus/gogen/internal/bar")
	tyGame := bar.Ref("Game").Type()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyGame, "g").
		Typ(tyGame).MemberVal("Run").VarRef("g").UnaryOp(token.AND).Val("Hi").Call(2).EndStmt().
		End()
	domTest(t, pkg, `package main

import "github.com/goplus/gogen/internal/bar"

func main() {
	var g bar.Game
	bar.Gopt_Game_Run(&g, "Hi")
}
`)
}

func TestErrTemplateRecvMethod(t *testing.T) {
	pkg := newMainPackage()
	bar := pkg.Import("github.com/goplus/gogen/internal/bar")
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestErrTemplateRecvMethod: no error?")
		}
	}()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.NewPointer(bar.Ref("Game").Type()), "g").
		VarVal("g").MemberVal("Run").Call(0).EndStmt().
		End()
}

func TestBigIntCastUntypedFloat(t *testing.T) {
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(nil, "a").
		Val(mbig.Ref("XGo_bigint")).
		Val(&ast.BasicLit{Kind: token.FLOAT, Value: "1e20"}).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a = builtin.XGo_bigint_Cast__1(func() *big.Int {
	v, _ := new(big.Int).SetString("100000000000000000000", 10)
	return v
}())
`)
}

func TestBigIntCastUntypedFloatError(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestBigIntCastUntypedFloatError: no error?")
		}
	}()
	pkg := newXGoMainPackage()
	mbig := pkg.Import("github.com/goplus/gogen/internal/builtin")
	pkg.CB().NewVarStart(nil, "a").
		Val(mbig.Ref("XGo_bigint")).
		Val(&ast.BasicLit{Kind: token.FLOAT, Value: "10000000000000000000.1"}).
		Call(1).EndInit(1)
}

func TestUntypedBigDefault(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(ctxRef(pkg, "println")).
		UntypedBigInt(big.NewInt(1)).Call(1).EndStmt().
		Val(ctxRef(pkg, "println")).
		UntypedBigRat(big.NewRat(1, 2)).Call(1).EndStmt().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	fmt.Println(builtin.XGo_bigint_Init__1(big.NewInt(1)))
	fmt.Println(builtin.XGo_bigrat_Init__2(big.NewRat(1, 2)))
}
`)
}

func TestUntypedBigDefaultCall(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		UntypedBigInt(big.NewInt(1)).MemberVal("Int64").Call(0).EndStmt().
		UntypedBigRat(big.NewRat(1, 2)).MemberVal("Float64").Call(0).EndStmt().End()
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

func main() {
	builtin.XGo_bigint_Init__1(big.NewInt(1)).Int64()
	builtin.XGo_bigrat_Init__2(big.NewRat(1, 2)).Float64()
}
`)
}

func TestUntypedBigIntToInterface(t *testing.T) {
	pkg := newXGoMainPackage()
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "Int64", types.NewSignatureType(nil, nil, nil, nil,
			types.NewTuple(types.NewVar(token.NoPos, nil, "v", types.Typ[types.Int64])), false)),
	}
	tyInterf := types.NewInterfaceType(methods, nil).Complete()
	tyA := pkg.NewType("A").InitType(pkg, tyInterf)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyA, "a").UntypedBigInt(big.NewInt(1)).EndInit(1).
		Val(ctxRef(pkg, "println")).
		VarVal("a").MemberVal("Int64").Call(0).Call(1).EndStmt().End()
	domTest(t, pkg, `package main

import (
	"fmt"
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

type A interface {
	Int64() (v int64)
}

func main() {
	var a A = builtin.XGo_bigint_Init__1(big.NewInt(1))
	fmt.Println(a.Int64())
}
`)
}

func TestInt128(t *testing.T) {
	pkg := newXGoMainPackage()
	builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
	n1 := big.NewInt(1)
	n1.Lsh(n1, 127).Sub(n1, big.NewInt(1))
	n2 := big.NewInt(-1)
	n2.Lsh(n2, 127)
	uint128 := builtin.Ref("Uint128").Type()
	int128 := builtin.Ref("Int128").Type()
	pkg.CB().NewVarStart(int128, "a").UntypedBigInt(n1).EndInit(1)
	pkg.CB().NewVarStart(int128, "b").Val(1).EndInit(1)
	pkg.CB().NewVarStart(int128, "c").Val(&ast.BasicLit{Kind: token.FLOAT, Value: "1e30"}).EndInit(1)
	pkg.CB().NewVarStart(int128, "d").Typ(int128).UntypedBigInt(n2).Call(1).EndInit(1)
	pkg.CB().NewVarStart(int128, "e").Typ(int128).Typ(uint128).Val(1).Call(1).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.Int128 = builtin.Int128_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("170141183460469231731687303715884105727", 10)
	return v
}())
var b builtin.Int128 = builtin.Int128_Init__0(1)
var c builtin.Int128 = builtin.Int128_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("1000000000000000000000000000000", 10)
	return v
}())
var d builtin.Int128 = builtin.Int128_Cast__1(func() *big.Int {
	v, _ := new(big.Int).SetString("-170141183460469231731687303715884105728", 10)
	return v
}())
var e builtin.Int128 = builtin.Int128(builtin.Uint128_Cast__0(1))
`)
}

func TestUint128(t *testing.T) {
	pkg := newXGoMainPackage()
	builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
	n1 := big.NewInt(1)
	n1.Lsh(n1, 128).Sub(n1, big.NewInt(1))
	uint128 := builtin.Ref("Uint128").Type()
	int128 := builtin.Ref("Int128").Type()
	pkg.CB().NewVarStart(uint128, "a").UntypedBigInt(n1).EndInit(1)
	pkg.CB().NewVarStart(uint128, "b").Val(0).EndInit(1)
	pkg.CB().NewVarStart(uint128, "c").Val(&ast.BasicLit{Kind: token.FLOAT, Value: "1e30"}).EndInit(1)
	pkg.CB().NewVarStart(uint128, "d").Typ(uint128).Typ(int128).Val(1).Call(1).Call(1).EndInit(1)
	domTest(t, pkg, `package main

import (
	"github.com/goplus/gogen/internal/builtin"
	"math/big"
)

var a builtin.Uint128 = builtin.Uint128_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("340282366920938463463374607431768211455", 10)
	return v
}())
var b builtin.Uint128 = builtin.Uint128_Init__0(0)
var c builtin.Uint128 = builtin.Uint128_Init__1(func() *big.Int {
	v, _ := new(big.Int).SetString("1000000000000000000000000000000", 10)
	return v
}())
var d builtin.Uint128 = builtin.Uint128(builtin.Int128_Cast__0(1))
`)
}

func TestErrInt128(t *testing.T) {
	t.Run("Int128_Max", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Int128_Max: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		n := big.NewInt(1)
		n.Lsh(n, 127)
		int128 := builtin.Ref("Int128").Type()
		pkg.CB().NewVarStart(int128, "a").UntypedBigInt(n).EndInit(1)
	})
	t.Run("Int128_Max_Float", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Int128_Max_Float: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		int128 := builtin.Ref("Int128").Type()
		pkg.CB().NewVarStart(int128, "a").Val(&ast.BasicLit{Kind: token.FLOAT, Value: "1e60"}).EndInit(1)
	})
	t.Run("Int128_Min", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Int128_Min: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		n := big.NewInt(-1)
		n.Lsh(n, 127).Sub(n, big.NewInt(1))
		int128 := builtin.Ref("Int128").Type()
		pkg.CB().NewVarStart(int128, "a").Typ(int128).UntypedBigInt(n).Call(1).EndInit(1)
	})
	t.Run("Int128_Uint128", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Int128_Uint128: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		n := big.NewInt(1)
		n.Lsh(n, 127)
		int128 := builtin.Ref("Int128").Type()
		uint128 := builtin.Ref("Uint128").Type()
		pkg.CB().NewVarStart(int128, "a").Typ(int128).Typ(uint128).UntypedBigInt(n).Call(1).Call(1).EndInit(1)
	})
}

func TestErrUint128(t *testing.T) {
	t.Run("Uint128_Max", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Uint128_Max: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		n := big.NewInt(1)
		n.Lsh(n, 128)
		uint128 := builtin.Ref("Uint128").Type()
		pkg.CB().NewVarStart(uint128, "a").UntypedBigInt(n).EndInit(1)
	})
	t.Run("Uint128_Max_Float", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Uint128_Max_Float: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		uint128 := builtin.Ref("Uint128").Type()
		pkg.CB().NewVarStart(uint128, "a").Val(&ast.BasicLit{Kind: token.FLOAT, Value: "1e60"}).EndInit(1)
	})
	t.Run("Uint128_Min", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Uint128_Min: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		uint128 := builtin.Ref("Uint128").Type()
		pkg.CB().NewVarStart(uint128, "a").Typ(uint128).Val(-1).Call(1).EndInit(1)
	})
	t.Run("Unt128_Int128", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("Unt128_Int128: no error?")
			} else {
				t.Log(e)
			}
		}()
		pkg := newXGoMainPackage()
		builtin := pkg.Import("github.com/goplus/gogen/internal/builtin")
		int128 := builtin.Ref("Int128").Type()
		uint128 := builtin.Ref("Uint128").Type()
		pkg.CB().NewVarStart(uint128, "a").Typ(uint128).Typ(int128).Val(-1).Call(1).Call(1).EndInit(1)
	})
}

// ----------------------------------------------------------------------------
