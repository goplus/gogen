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
	"bytes"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
	"golang.org/x/tools/go/gcexportdata"
)

func init() {
	gox.SetDebug(true)
}

func domTest(t *testing.T, pkg *gox.Package, expected string) {
	var b bytes.Buffer
	err := gox.WriteTo(&b, pkg)
	if err != nil {
		t.Fatal("gox.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\nResult:\n%s\nExpected:\n%s\n", result, expected)
	}
}

// ----------------------------------------------------------------------------

func TestAssignableTo(t *testing.T) {
	assigns := []struct {
		v, t types.Type
		ret  bool
	}{
		{types.Typ[types.UntypedInt], types.Typ[types.Int], true},
		{types.Typ[types.Int], types.Typ[types.UntypedInt], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedComplex], true},
		{types.Typ[types.UntypedComplex], types.Typ[types.UntypedFloat], false},
		{types.Typ[types.UntypedInt], types.Typ[types.UntypedFloat], true},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedInt], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedBool], false},
		{types.Typ[types.UntypedInt], types.Typ[types.UntypedRune], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedRune], false},
		{types.Typ[types.UntypedRune], types.Typ[types.UntypedInt], true},
		{types.Typ[types.UntypedRune], types.Typ[types.UntypedFloat], true},
	}
	for _, a := range assigns {
		if ret := gox.AssignableTo(a.v, a.t); ret != a.ret {
			t.Fatalf("Failed: AssignableTo %v => %v returns %v\n", a.v, a.t, ret)
		}
	}
}

func TestGoTypesPkg(t *testing.T) {
	const src = `package foo

type mytype = byte

func bar(v mytype) rune {
	return 0
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "foo.go", src, 0)
	if err != nil {
		t.Fatal("parser.ParseFile:", err)
	}

	packages := make(map[string]*types.Package)
	imp := gcexportdata.NewImporter(fset, packages)
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check("foo", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal("conf.Check:", err)
	}
	bar := pkg.Scope().Lookup("bar")
	if bar.String() != "func foo.bar(v byte) rune" {
		t.Fatal("bar.Type:", bar)
	}
}

// ----------------------------------------------------------------------------

func TestBasic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	if pkg.Ref("main") == nil {
		t.Fatal("main not found")
	}
	domTest(t, pkg, `package main

func main() {
}
`)
}

func TestMapLit(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewVarStart(nil, "a").
		Val("a").Val(1).Val("b").Val(2).MapLit(nil, 4).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val("a").Val(1).Val("b").Val(1.2).MapLit(nil, 4).EndInit(1)
	pkg.NewVarStart(nil, "c").
		MapLit(nil, 0).EndInit(1)
	pkg.NewVarStart(nil, "d").
		MapLit(types.NewMap(types.Typ[types.Int], types.Typ[types.Bool]), 0).EndInit(1)
	domTest(t, pkg, `package main

var a = map[string]int{"a": 1, "b": 2}
var b = map[string]float64{"a": 1, "b": 1.2}
var c = map[string]interface {
}{}
var d = map[int]bool{}
`)
}

func TestSliceLit(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewVarStart(nil, "a").
		Val("a").Val("b").SliceLit(nil, 2).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(1).Val(1.2).Val(3).SliceLit(nil, 3).EndInit(1)
	pkg.NewVarStart(nil, "c").
		SliceLit(nil, 0).EndInit(1)
	pkg.NewVarStart(nil, "d").
		SliceLit(types.NewSlice(types.Typ[types.Int]), 0).EndInit(1)
	domTest(t, pkg, `package main

var a = []string{"a", "b"}
var b = []float64{1, 1.2, 3}
var c = []interface {
}{}
var d = []int{}
`)
}

func TestArrayLit(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewVarStart(nil, "a").
		Val("a").Val("b").ArrayLit(types.NewArray(types.Typ[types.String], 2), 2).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(1).Val(1.2).Val(3).ArrayLit(types.NewArray(types.Typ[types.Float64], -1), 3).EndInit(1)
	pkg.NewVarStart(nil, "c").
		ArrayLit(types.NewArray(gox.TyEmptyInterface, 10), 0).EndInit(1)
	domTest(t, pkg, `package main

var a = [2]string{"a", "b"}
var b = [3]float64{1, 1.2, 3}
var c = [10]interface {
}{}
`)
}

func TestConst(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	tv := pkg.ConstStart().Val(1).Val(2).BinaryOp(token.ADD).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(3)) {
		t.Fatal("TestConst: != 3, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val("1").Val("2").BinaryOp(token.ADD).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeString("12")) {
		t.Fatal("TestConst: != 12, it is", tv.Value)
	}
}

func TestConstLenArray(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	typ := types.NewArray(types.Typ[types.Int], 10)
	pkg.Types.Scope().Insert(types.NewVar(token.NoPos, pkg.Types, "array", typ))
	tv := pkg.ConstStart().Val(pkg.Builtin().Ref("len")).Val(pkg.Ref("array")).Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(10)) {
		t.Fatal("TestConst: != 10, it is", tv.Value)
	}
}

func TestConstDecl(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewConstStart(nil, "n").
		Val(1).Val(2).BinaryOp(token.ADD).EndInit(1)
	pkg.NewConstStart(types.Typ[types.String], "x").
		Val("1").Val("2").BinaryOp(token.ADD).EndInit(1)
	pkg.CB().NewConstStart(types.Typ[types.String], "y").
		Val("Hello").EndInit(1)
	domTest(t, pkg, `package main

const n = 1 + 2
const x string = "1" + "2"
const y string = "Hello"
`)
}

func TestVarDecl(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewVarStart(nil, "n", "s").
		Val(1).Val(2).BinaryOp(token.ADD).
		Val("1").Val("2").BinaryOp(token.ADD).
		EndInit(2)
	pkg.NewVarStart(types.Typ[types.String], "x").
		Val("Hello, ").Val("Go+").BinaryOp(token.ADD).
		EndInit(1)
	pkg.CB().NewVarStart(types.Typ[types.String], "y").
		Val("Hello").
		EndInit(1)
	domTest(t, pkg, `package main

var n, s = 1 + 2, "1" + "2"
var x string = "Hello, " + "Go+"
var y string = "Hello"
`)
}

func TestVarDeclNoBody(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewVar(types.Typ[types.String], "x")
	domTest(t, pkg, `package main

var x string
`)
}

func TestVarDeclInFunc(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.Typ[types.String], "x", "y").
		NewVarStart(nil, "a", "_").Val(1).Val(2).BinaryOp(token.ADD).Val("Hi").EndInit(2).
		NewVarStart(nil, "n", "_").Val(fmt.Ref("Println")).Val(2).Call(1).EndInit(1).
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var x, y string
	var a, _ = 1 + 2, "Hi"
	var n, _ = fmt.Println(2)
}
`)
}

func TestDefineVar(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.Typ[types.Int], "n").
		DefineVarStart("n", "err").Val(fmt.Ref("Println")).Val(2).Call(1).EndInit(1).
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var n int
	n, err := fmt.Println(2)
}
`)
}

func TestFuncBasic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", gox.TyByte)
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, false).BodyStart(pkg).End().Pkg().
		NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v byte) {
}
func main() {
}
`)
}

func TestFuncVariadic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", types.NewSlice(gox.TyByte))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v ...byte) {
}
func main() {
}
`)
}

func TestFuncAsParam(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", types.NewSignature(nil, nil, nil, false))
	x := pkg.NewParam("x", types.NewPointer(types.Typ[types.Bool]))
	y := pkg.NewParam("y", types.NewChan(types.SendOnly, types.Typ[types.Bool]))
	z := pkg.NewParam("z", types.Typ[types.UnsafePointer])
	pkg.NewFunc(nil, "foo", gox.NewTuple(v, x, y, z), nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import unsafe "unsafe"

func foo(v func(), x *bool, y chan<- bool, z unsafe.Pointer) {
}
func main() {
}
`)
}

func TestBuiltinFunc(t *testing.T) {
	var a, n *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	builtin := pkg.Builtin()
	v := pkg.NewParam("v", types.NewSlice(types.Typ[types.Int]))
	array := pkg.NewParam("array", types.NewArray(types.Typ[types.Int], 10))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v, array), nil, false).BodyStart(pkg).
		NewAutoVar("a", &a).NewAutoVar("n", &n).
		VarRef(a).
		/**/ Val(builtin.Ref("append")).Val(v).Val(1).Val(2).Call(3).
		/**/ Assign(1).EndStmt().
		VarRef(n).Val(builtin.Ref("len")).Val(a).Call(1).Assign(1).EndStmt().
		VarRef(n).Val(builtin.Ref("cap")).Val(array).Call(1).Assign(1).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v []int, array [10]int) {
	var a []int
	var n int
	a = append(v, 1, 2)
	n = len(a)
	n = cap(array)
}
func main() {
}
`)
}

func TestOverloadFunc(t *testing.T) {
	var f, g, x, y *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	builtin := pkg.Builtin()
	c64 := pkg.NewParam("c64", types.Typ[types.Complex64])
	c128 := pkg.NewParam("c128", types.Typ[types.Complex128])
	pkg.NewFunc(nil, "foo", gox.NewTuple(c64, c128), nil, false).BodyStart(pkg).
		NewAutoVar("f", &f).NewAutoVar("g", &g).NewAutoVar("x", &x).NewAutoVar("y", &y).
		VarRef(f).Val(builtin.Ref("imag")).Val(c128).Call(1).Assign(1).EndStmt().
		VarRef(g).Val(builtin.Ref("real")).Val(c64).Call(1).Assign(1).EndStmt().
		VarRef(x).Val(builtin.Ref("complex")).Val(0).Val(f).Call(2).Assign(1).EndStmt().
		VarRef(y).Val(builtin.Ref("complex")).Val(g).Val(1).Call(2).Assign(1).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(c64 complex64, c128 complex128) {
	var f float64
	var g float32
	var x complex128
	var y complex64
	f = imag(c128)
	g = real(c64)
	x = complex(0, f)
	y = complex(g, 1)
}
func main() {
}
`)
}

func TestEmptyInterface(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", types.NewSlice(gox.TyEmptyInterface))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v ...interface {
}) {
}
func main() {
}
`)
}

func TestInterfaceMethods(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	bar := types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false))
	methods := []*types.Func{bar}
	v := pkg.NewParam("v", types.NewSlice(types.NewInterfaceType(methods, nil)))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v ...interface {
	Bar()
}) {
}
func main() {
}
`)
}

func TestFuncCall(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1, false).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	fmt.Println("Hello")
}
`)
}

func TestFuncCallEllipsis(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	bar := types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false))
	methods := []*types.Func{bar}
	v := pkg.NewParam("v", types.NewSlice(types.NewInterfaceType(methods, nil)))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val(v).Call(1, true).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(v ...interface {
	Bar()
}) {
	fmt.Println(v...)
}
func main() {
}
`)
}

func TestDelayedLoad(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	println := gox.NewDelayedLoad(token.NoPos, pkg.Types, "println", func() types.Object {
		return pkg.Import("fmt").Ref("Println")
	})
	pkg.Types.Scope().Insert(println)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		Val(println).Val(format).Val(args).Call(2, true).Return(1).
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(format string, args ...interface {
}) ( int,  error) {
	return fmt.Println(format, args...)
}
func main() {
}
`)
}

func TestReturn(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		Val(pkg.Import("fmt").Ref("Println")).Val(format).Val(args).Call(2, true).Return(1).
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(format string, args ...interface {
}) ( int,  error) {
	return fmt.Println(format, args...)
}
func main() {
}
`)
}

func TestReturnExpr(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		Val(0).Val(types.Universe.Lookup("nil")).Return(2).
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(format string, args ...interface {
}) ( int,  error) {
	return 0, nil
}
func main() {
}
`)
}

func TestReturnNamedResults(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("n", types.Typ[types.Int])
	err := pkg.NewParam("err", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		VarRef(pkg.CB().Scope().Lookup("n")).VarRef(err).Val(1).Val(nil).Assign(2).
		Return(0).
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(format string, args ...interface {
}) (n int, err error) {
	n, err = 1, nil
	return
}
func main() {
}
`)
}

func TestImport(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")

	v := pkg.NewParam("v", types.NewSlice(gox.TyByte))
	pkg.NewFunc(nil, "fmt", gox.NewTuple(v), nil, false).BodyStart(pkg).End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt1 "fmt"

func fmt(v []byte) {
}
func main() {
	fmt1.Println("Hello")
}
`)
}

func TestImportUnused(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func main() {
}
`)
}

func TestImportAnyWhere(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)

	v := pkg.NewParam("v", types.NewSlice(gox.TyByte))
	pkg.NewFunc(nil, "fmt", gox.NewTuple(v), nil, false).BodyStart(pkg).End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(pkg.Import("fmt").Ref("Println")).Val("Hello").Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt1 "fmt"

func fmt(v []byte) {
}
func main() {
	fmt1.Println("Hello")
}
`)
}

func TestImportAndCallMethod(t *testing.T) {
	var x *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	strings := pkg.Import("strings")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("x", &x).
		VarRef(x).Val(strings.Ref("NewReplacer")).Val("?").Val("!").Call(2).
		/**/ MemberVal("Replace").Val("hello, world???").Call(1).Assign(1).EndStmt().
		Val(pkg.Builtin().Ref("println")).Val(x).Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import strings "strings"

func main() {
	var x string
	x = strings.NewReplacer("?", "!").Replace("hello, world???")
	println(x)
}
`)
}

func TestAssign(t *testing.T) {
	var a, b, c, d, e, f, g *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("a", &a).NewAutoVar("b", &b).NewAutoVar("c", &c).NewAutoVar("d", &d).
		NewAutoVar("e", &e).NewAutoVar("f", &f).NewAutoVar("g", &g).
		VarRef(a).VarRef(b).VarRef(d).VarRef(e).VarRef(f).VarRef(g).
		Val("Hi").Val(3).Val(true).Val('!').Val(1.2).Val(&ast.BasicLit{Kind: token.FLOAT, Value: "12.3"}).
		Assign(6).EndStmt().
		VarRef(c).Val(b).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a string
	var b int
	var c int
	var d bool
	var e rune
	var f float64
	var g float64
	a, b, d, e, f, g = "Hi", 3, true, '!', 1.2, 12.3
	c = b
}
`)
}

func TestAssignFnCall(t *testing.T) {
	var n, err *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("n", &n).NewAutoVar("err", &err).
		VarRef(n).VarRef(err).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).
		Assign(2, 1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var n int
	var err error
	n, err = fmt.Println("Hello")
}
`)
}

func TestAssignUnderscore(t *testing.T) {
	var err *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("err", &err).
		VarRef(nil).VarRef(err).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).
		Assign(2, 1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var err error
	_, err = fmt.Println("Hello")
}
`)
}

func TestOperator(t *testing.T) {
	var a, b, c, d *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("a", &a).NewAutoVar("b", &b).NewAutoVar("c", &c).NewAutoVar("d", &d).
		VarRef(a).Val("Hi").Assign(1).EndStmt().
		VarRef(b).Val(a).Val("!").BinaryOp(token.ADD).Assign(1).EndStmt().
		VarRef(c).Val(&ast.BasicLit{Kind: token.INT, Value: "13"}).Assign(1).EndStmt().
		VarRef(d).Val(c).UnaryOp(token.SUB).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a string
	var b string
	var c int
	var d int
	a = "Hi"
	b = a + "!"
	c = 13
	d = -c
}
`)
}

func TestOperatorComplex(t *testing.T) {
	var a *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("a", &a).
		VarRef(a).Val(123.1).Val(&ast.BasicLit{Kind: token.IMAG, Value: "3i"}).BinaryOp(token.SUB).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a complex128
	a = 123.1 - 3i
}
`)
}

func TestBinaryOpUntyped(t *testing.T) {
	var a *gox.AutoVar
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar("a", &a).
		VarRef(a).Val("Hi").Val("!").BinaryOp(token.ADD).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a string
	a = "Hi" + "!"
}
`)
}

func TestClosure(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	paramV := pkg.NewParam("v", types.Typ[types.String]) // v string
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewClosure(gox.NewTuple(paramV), nil, false).BodyStart(pkg).
		/**/ Val(fmt.Ref("Println")).Val(paramV).Call(1).EndStmt().
		/**/ End().
		Val("Hello").Call(1).EndStmt(). // func(v string) { fmt.Println(v) } ("Hello")
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	func(v string) {
		fmt.Println(v)
	}("Hello")
}
`)
}

// ----------------------------------------------------------------------------

func ctxRef(pkg *gox.Package, name string) gox.Ref {
	return pkg.CB().Scope().Lookup(name)
}

func TestExample(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)

	fmt := pkg.Import("fmt")

	v := pkg.NewParam("v", types.Typ[types.String]) // v string

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a", "b").Val("Hi").Val(3).EndInit(2).   // a, b := "Hi", 3
		NewVarStart(nil, "c").Val(ctxRef(pkg, "b")).EndInit(1). // var c = b
		NewVar(gox.TyEmptyInterface, "x", "y").                 // var x, y interface{}
		Val(fmt.Ref("Println")).
		/**/ Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).Val(ctxRef(pkg, "c")). // fmt.Println(a, b, c)
		/**/ Call(3).EndStmt().
		NewClosure(gox.NewTuple(v), nil, false).BodyStart(pkg).
		/**/ Val(fmt.Ref("Println")).Val(v).Call(1).EndStmt(). // fmt.Println(v)
		/**/ End().
		Val("Hello").Call(1).EndStmt(). // func(v string) { ... } ("Hello")
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	a, b := "Hi", 3
	var c = b
	var x, y interface {
	}
	fmt.Println(a, b, c)
	func(v string) {
		fmt.Println(v)
	}("Hello")
}
`)
}

// ----------------------------------------------------------------------------
