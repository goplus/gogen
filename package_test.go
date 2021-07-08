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

func TestAssignableTo(t *testing.T) {
	if !types.AssignableTo(types.Typ[types.UntypedInt], types.Typ[types.Int]) {
		t.Fatal("Failed: AssignableTo untyped int => int")
	}
	if types.AssignableTo(types.Typ[types.Int], types.Typ[types.UntypedInt]) {
		t.Fatal("AssignableTo int => untyped int?")
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

func TestBasic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func main() {
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

func TestBuiltinFunc(t *testing.T) {
	var a, n *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	builtin := pkg.Builtin()
	v := pkg.NewParam("v", types.NewSlice(types.Typ[types.Int]))
	array := pkg.NewParam("array", types.NewArray(types.Typ[types.Int], 10))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v, array), nil, false).BodyStart(pkg).
		NewVar("a", &a).NewVar("n", &n).
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

func _TestOverloadFunc(t *testing.T) {
	var f, g *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	builtin := pkg.Builtin()
	c64 := pkg.NewParam("c64", types.Typ[types.Complex64])
	c128 := pkg.NewParam("c128", types.Typ[types.Complex128])
	pkg.NewFunc(nil, "foo", gox.NewTuple(c64, c128), nil, false).BodyStart(pkg).
		NewVar("f", &f).NewVar("g", &g).
		VarRef(f).Val(builtin.Ref("imag")).Val(c128).Call(1).Assign(1).EndStmt().
		VarRef(g).Val(builtin.Ref("real")).Val(c64).Call(1).Assign(1).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(c64 complex64, c128 complex128) {
	var f float64
	var g float32
	f = imag(c128)
	g = real(c64)
}
func main() {
}
`)
}

func TestEmptyInterface(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", types.NewSlice(types.NewInterfaceType(nil, nil)))
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

func TestAssign(t *testing.T) {
	var a, b, c, d, e, f, g *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("a", &a).NewVar("b", &b).NewVar("c", &c).NewVar("d", &d).
		NewVar("e", &e).NewVar("f", &f).NewVar("g", &g).
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
	var n, err *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("n", &n).NewVar("err", &err).
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
	var err *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("err", &err).
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
	var a, b, c, d *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("a", &a).NewVar("b", &b).NewVar("c", &c).NewVar("d", &d).
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

func TestBinaryOpUntyped(t *testing.T) {
	var a *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("a", &a).
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
		/**/ Val("Hello").Call(1).EndStmt(). // func(v string) { fmt.Println(v) } ("Hello")
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
