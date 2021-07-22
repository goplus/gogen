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

var (
	gblFset     *token.FileSet
	gblLoadPkgs gox.LoadPkgsFunc
)

func init() {
	gox.SetDebug(gox.DbgFlagAll)
	gblFset = token.NewFileSet()
	gblLoadPkgs = gox.NewLoadPkgsCached(nil)
}

func newMainPackage(noCache ...bool) *gox.Package {
	conf := &gox.Config{
		Fset:     gblFset,
		LoadPkgs: gblLoadPkgs,
	}
	if noCache != nil {
		conf = nil
	}
	return gox.NewPackage("", "main", conf)
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

type goxVar = types.Var

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
		{types.Typ[types.UntypedFloat], types.Typ[types.Int], false},
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

func TestMethods(t *testing.T) {
	const src = `package foo

type foo struct {}
func (a foo) A() {}
func (p *foo) Bar() {}
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
	foo, ok := pkg.Scope().Lookup("foo").Type().(*types.Named)
	if !ok {
		t.Fatal("foo not found")
	}
	if foo.NumMethods() != 2 {
		t.Fatal("foo.NumMethods:", foo.NumMethods())
	}
	if foo0 := foo.Method(0).String(); foo0 != "func (foo.foo).A()" {
		t.Fatal("foo.0 =", foo0)
	}
	if foo1 := foo.Method(1).String(); foo1 != "func (*foo.foo).Bar()" {
		t.Fatal("foo.1 =", foo1)
	}
}

// ----------------------------------------------------------------------------

func fileline(file string, line int) *gox.FileLine {
	return &gox.FileLine{File: file, Line: line}
}

func TestBasic(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	if pkg.Ref("main") == nil {
		t.Fatal("main not found")
	}
	domTest(t, pkg, `package main

func main() {
}
`)
}

func TestMake(t *testing.T) {
	pkg := newMainPackage()
	tySlice := types.NewSlice(types.Typ[types.Int])
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tySlice, "a").Val(pkg.Builtin().Ref("make")).
		/**/ Typ(tySlice).Val(0).Val(2).Call(3).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a []int = make([]int, 0, 2)
}
`)
}

func TestNew(t *testing.T) {
	pkg := newMainPackage()
	tyInt := types.Typ[types.Int]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(types.NewPointer(tyInt), "a").Val(pkg.Builtin().Ref("new")).
		Val(ctxRef(pkg, "int")).Call(1).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a *int = new(int)
}
`)
}

func TestTypeConv(t *testing.T) {
	pkg := newMainPackage()
	tyInt := types.Typ[types.Uint32]
	tyPInt := types.NewPointer(tyInt)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyInt, "a").Typ(tyInt).Val(0).Call(1).EndInit(1).
		NewVarStart(tyPInt, "b").Typ(tyPInt).Val(nil).Call(1).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a uint32 = uint32(0)
	var b *uint32 = (*uint32)(nil)
}
`)
}

func TestIncDec(t *testing.T) {
	pkg := newMainPackage()
	tyInt := types.Typ[types.Uint]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		SetFileLine(fileline("./foo.gop", 1), false).
		NewVar(tyInt, "a").
		SetFileLine(fileline("./foo.gop", 2), true).
		VarRef(ctxRef(pkg, "a")).IncDec(token.INC).EndStmt().
		End()
	if pkg.CB().FileLine() != nil {
		t.Fatal("fileLine is not nil")
	}
	domTest(t, pkg, `package main

func main() {
//line ./foo.gop:1
	var a uint
//line ./foo.gop:2
	a++
}
`)
}

func TestSend(t *testing.T) {
	pkg := newMainPackage()
	tyChan := types.NewChan(types.SendRecv, types.Typ[types.Uint])
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyChan, "a").
		Val(ctxRef(pkg, "a")).Val(1).Send().
		End()
	domTest(t, pkg, `package main

func main() {
	var a chan uint
	a <- 1
}
`)
}

func TestRecv(t *testing.T) {
	pkg := newMainPackage()
	tyChan := types.NewChan(types.SendRecv, types.Typ[types.Uint])
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyChan, "a").
		NewVarStart(types.Typ[types.Uint], "b").Val(ctxRef(pkg, "a")).UnaryOp(token.ARROW).EndInit(1).
		DefineVarStart("c", "ok").Val(ctxRef(pkg, "a")).UnaryOp(token.ARROW, true).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a chan uint
	var b uint = <-a
	c, ok := <-a
}
`)
}

func TestZeroLit(t *testing.T) {
	pkg := newMainPackage()
	tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	ret := pkg.NewAutoParam("ret")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyMap, "a").
		/**/ NewClosure(nil, gox.NewTuple(ret), false).BodyStart(pkg).
		/******/ VarRef(ctxRef(pkg, "ret")).ZeroLit(ret.Type()).Assign(1).
		/******/ Val(ctxRef(pkg, "ret")).Val("Hi").IndexRef(1).Val(1).Assign(1).
		/******/ Return(0).
		/**/ End().Call(0).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a map[string]int = func() (ret map[string]int) {
		ret = map[string]int{}
		ret["Hi"] = 1
		return
	}()
}
`)
}

func TestZeroLitAllTypes(t *testing.T) {
	pkg := newMainPackage()
	tyString := types.Typ[types.String]
	tyBool := types.Typ[types.Bool]
	tyUP := types.Typ[types.UnsafePointer]
	tyMap := types.NewMap(tyString, types.Typ[types.Int])
	tySlice := types.NewSlice(types.Typ[types.Int])
	tyArray := types.NewArray(types.Typ[types.Int], 10)
	tyPointer := types.NewPointer(types.Typ[types.Int])
	tyChan := types.NewChan(types.SendRecv, types.Typ[types.Int])
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyMap, "a").ZeroLit(tyMap).EndInit(1).
		NewVarStart(tySlice, "b").ZeroLit(tySlice).EndInit(1).
		NewVarStart(tyPointer, "c").ZeroLit(tyPointer).EndInit(1).
		NewVarStart(tyChan, "d").ZeroLit(tyChan).EndInit(1).
		NewVarStart(tyBool, "e").ZeroLit(tyBool).EndInit(1).
		NewVarStart(tyString, "f").ZeroLit(tyString).EndInit(1).
		NewVarStart(tyUP, "g").ZeroLit(tyUP).EndInit(1).
		NewVarStart(gox.TyEmptyInterface, "h").ZeroLit(gox.TyEmptyInterface).EndInit(1).
		NewVarStart(tyArray, "i").ZeroLit(tyArray).EndInit(1).
		End()
	domTest(t, pkg, `package main

import unsafe "unsafe"

func main() {
	var a map[string]int = nil
	var b []int = nil
	var c *int = nil
	var d chan int = nil
	var e bool = false
	var f string = ""
	var g unsafe.Pointer = nil
	var h interface {
	} = nil
	var i [10]int = [10]int{}
}
`)
}

func TestTypeDeclInFunc(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typ := types.NewStruct(fields, nil)
	cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg)
	foo := cb.NewType("foo").InitType(pkg, typ)
	cb.AliasType("bar", typ)
	a := cb.AliasType("a", foo)
	cb.AliasType("b", a)
	cb.End()
	domTest(t, pkg, `package main

func main() {
	type foo struct {
		x int
		y string
	}
	type bar = struct {
		x int
		y string
	}
	type a = foo
	type b = a
}
`)
}

func TestTypeDecl(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typ := types.NewStruct(fields, nil)
	foo := pkg.NewType("foo").InitType(pkg, typ)
	pkg.AliasType("bar", typ)
	a := pkg.AliasType("a", foo)
	pkg.AliasType("b", a)
	domTest(t, pkg, `package main

type foo struct {
	x int
	y string
}
type bar = struct {
	x int
	y string
}
type a = foo
type b = a
`)
}

func TestTypeCycleDef(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.NewType("foo")
	a := pkg.AliasType("a", foo.Type())
	b := pkg.AliasType("b", a)
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "p", types.NewPointer(b), false),
	}
	foo.InitType(pkg, types.NewStruct(fields, nil))
	domTest(t, pkg, `package main

type foo struct {
	p *b
}
type a = foo
type b = a
`)
}

func TestTypeMethods(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typ := types.NewStruct(fields, nil)
	foo := pkg.NewType("foo").InitType(pkg, typ)
	recv := pkg.NewParam("a", foo)
	precv := pkg.NewParam("p", types.NewPointer(foo))
	pkg.NewFunc(recv, "Bar", nil, nil, false).BodyStart(pkg).End()
	pkg.NewFunc(precv, "Print", nil, nil, false).BodyStart(pkg).End()
	if foo.NumMethods() != 2 {
		t.Fatal("foo.NumMethods != 2")
	}
	domTest(t, pkg, `package main

type foo struct {
	x int
	y string
}

func (a foo) Bar() {
}
func (p *foo) Print() {
}
`)
}

func TestAssignInterface(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.NewType("foo").InitType(pkg, types.Typ[types.Int])
	recv := pkg.NewParam("a", foo)
	ret := pkg.NewParam("ret", types.Typ[types.String])
	pkg.NewFunc(recv, "Error", nil, types.NewTuple(ret), false).BodyStart(pkg).
		Return(0).
		End()
	pkg.NewVarStart(gox.TyError, "err").
		Typ(foo).ZeroLit(foo).Call(1).
		EndInit(1)
	domTest(t, pkg, `package main

type foo int

func (a foo) Error() (ret string) {
	return
}

var err error = foo(0)
`)
}

func TestAssignUserInterface(t *testing.T) {
	pkg := newMainPackage()
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
	}
	tyInterf := types.NewInterfaceType(methods, nil).Complete()
	typStruc := types.NewStruct(nil, nil)
	foo := pkg.NewType("foo").InitType(pkg, tyInterf)
	bar := pkg.NewType("bar").InitType(pkg, typStruc)
	pbar := types.NewPointer(bar)
	recv := pkg.NewParam("p", pbar)
	vfoo := types.NewTuple(pkg.NewParam("v", types.NewSlice(foo)))
	pkg.NewFunc(recv, "Bar", nil, nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "f", vfoo, nil, true).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(pbar, "v").
		Val(ctxRef(pkg, "f")).
		Val(ctxRef(pkg, "v")).Val(ctxRef(pkg, "v")).Call(2).EndStmt().
		End()
	domTest(t, pkg, `package main

type foo interface {
	Bar()
}
type bar struct {
}

func (p *bar) Bar() {
}
func f(v ...foo) {
}
func main() {
	var v *bar
	f(v, v)
}
`)
}

func TestTypeAssert(t *testing.T) {
	pkg := newMainPackage()
	params := types.NewTuple(pkg.NewParam("v", gox.TyEmptyInterface))
	pkg.NewFunc(nil, "foo", params, nil, false).BodyStart(pkg).
		DefineVarStart("x").Val(ctxRef(pkg, "v")).TypeAssert(types.Typ[types.Int], false).EndInit(1).
		DefineVarStart("y", "ok").Val(ctxRef(pkg, "v")).TypeAssert(types.Typ[types.String], true).EndInit(1).
		End()
	domTest(t, pkg, `package main

func foo(v interface {
}) {
	x := v.(int)
	y, ok := v.(string)
}
`)
}

func TestTypeSwitch(t *testing.T) {
	pkg := newMainPackage()
	p := pkg.NewParam("p", types.NewPointer(gox.TyEmptyInterface))
	v := pkg.NewParam("v", gox.TyEmptyInterface)
	pkg.NewFunc(nil, "bar", types.NewTuple(p), nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
		/**/ TypeSwitch("t").Val(v).TypeAssertThen().
		/****/ Typ(types.Typ[types.Int]).Typ(types.Typ[types.String]).TypeCase(2).
		/******/ Val(ctxRef(pkg, "bar")).Val(ctxRef(pkg, "t")).UnaryOp(token.AND).Call(1).EndStmt().
		/****/ End().
		/**/ Typ(types.Typ[types.Bool]).TypeCase(1).
		/******/ NewVarStart(types.Typ[types.Bool], "x").Val(ctxRef(pkg, "t")).EndInit(1).
		/****/ End().
		/****/ TypeCase(0).
		/******/ Val(ctxRef(pkg, "bar")).Val(ctxRef(pkg, "t")).UnaryOp(token.AND).Call(1).EndStmt().
		/****/ End().
		/**/ End().
		End()
	domTest(t, pkg, `package main

func bar(p *interface {
}) {
}
func foo(v interface {
}) {
	switch t := v.(type) {
	case int, string:
		bar(&t)
	case bool:
		var x bool = t
	default:
		bar(&t)
	}
}
`)
}

func TestTypeSwitch2(t *testing.T) {
	pkg := newMainPackage()
	p := pkg.NewParam("p", types.NewPointer(gox.TyEmptyInterface))
	v := pkg.NewParam("v", gox.TyEmptyInterface)
	pkg.NewFunc(nil, "bar", types.NewTuple(p), nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
		/**/ TypeSwitch("").Val(ctxRef(pkg, "bar")).Val(nil).Call(1).EndStmt().Val(v).TypeAssertThen().
		/****/ Typ(types.Typ[types.Int]).TypeCase(1).
		/******/ Val(ctxRef(pkg, "bar")).Val(ctxRef(pkg, "v")).UnaryOp(token.AND).Call(1).EndStmt().
		/****/ End().
		/**/ End().
		End()
	domTest(t, pkg, `package main

func bar(p *interface {
}) {
}
func foo(v interface {
}) {
	switch bar(nil); v.(type) {
	case int:
		bar(&v)
	}
}
`)
}

func TestSelect(t *testing.T) {
	pkg := newMainPackage()
	tyXchg := types.NewChan(types.SendRecv, types.Typ[types.Int])
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyXchg, "xchg").
		/**/ Select().
		/****/ DefineVarStart("x").Val(ctxRef(pkg, "xchg")).UnaryOp(token.ARROW).EndInit(1).CommCase(1).
		/******/ NewVarStart(types.Typ[types.Int], "t").Val(ctxRef(pkg, "x")).EndInit(1).
		/****/ End().
		/****/ Val(ctxRef(pkg, "xchg")).Val(1).Send().CommCase(1).
		/******/ DefineVarStart("x").Val(1).EndInit(1).
		/****/ End().
		/****/ CommCase(0).
		/******/ DefineVarStart("x").Val("Hi").EndInit(1).
		/****/ End().
		/**/ End().
		End()
	domTest(t, pkg, `package main

func main() {
	var xchg chan int
	select {
	case x := <-xchg:
		var t int = x
	case xchg <- 1:
		x := 1
	default:
		x := "Hi"
	}
}
`)
}

func TestStructLit(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typ := types.NewStruct(fields, nil)
	pkg.NewVarStart(nil, "a").
		StructLit(typ, 0, false).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(123).Val("Hi").
		StructLit(typ, 2, false).EndInit(1)
	pkg.NewVarStart(nil, "c").
		Val(1).Val("abc").
		StructLit(typ, 2, true).EndInit(1)
	domTest(t, pkg, `package main

var a = struct {
	x int
	y string
}{}
var b = struct {
	x int
	y string
}{123, "Hi"}
var c = struct {
	x int
	y string
}{y: "abc"}
`)
}

func TestNamedStructLit(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typU := types.NewStruct(fields, nil)
	typ := pkg.NewType("foo").InitType(pkg, typU)
	bar := pkg.AliasType("bar", typ)
	pkg.NewVarStart(typ, "a").
		StructLit(typ, 0, false).EndInit(1)
	pkg.NewVarStart(types.NewPointer(bar), "b").
		Val(123).Val("Hi").
		StructLit(bar, 2, false).
		UnaryOp(token.AND).
		EndInit(1)
	pkg.NewVarStart(bar, "c").
		Val(1).Val("abc").
		StructLit(typ, 2, true).EndInit(1)
	domTest(t, pkg, `package main

type foo struct {
	x int
	y string
}
type bar = foo

var a foo = foo{}
var b *bar = &bar{123, "Hi"}
var c bar = foo{y: "abc"}
`)
}

func TestMapLit(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewVarStart(nil, "a").
		Val("a").Val(1).Val("b").Val(2).MapLit(nil, 4).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val("a").Val(1).Val("b").Val(1.2).MapLit(nil, 4).EndInit(1)
	pkg.NewVarStart(nil, "c").
		MapLit(nil, 0).EndInit(1)
	pkg.NewVarStart(nil, "d").
		Val(1).Val(true).
		MapLit(types.NewMap(types.Typ[types.Int], types.Typ[types.Bool]), 2).EndInit(1)
	domTest(t, pkg, `package main

var a = map[string]int{"a": 1, "b": 2}
var b = map[string]float64{"a": 1, "b": 1.2}
var c = map[string]interface {
}{}
var d = map[int]bool{1: true}
`)
}

func TestNamedMapLit(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.NewType("foo").InitType(pkg, types.NewMap(types.Typ[types.Int], types.Typ[types.Bool]))
	bar := pkg.AliasType("bar", foo)
	pkg.NewVarStart(foo, "a").
		Val(1).Val(true).
		MapLit(foo, 2).EndInit(1)
	pkg.NewVarStart(bar, "b").
		Val(1).Val(true).
		MapLit(bar, 2).EndInit(1)
	domTest(t, pkg, `package main

type foo map[int]bool
type bar = foo

var a foo = foo{1: true}
var b bar = bar{1: true}
`)
}

func TestSliceLit(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewVarStart(nil, "a").
		Val("a").Val("b").SliceLit(nil, 2).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(1).Val(1.2).Val(3).SliceLit(nil, 3).EndInit(1)
	pkg.NewVarStart(nil, "c").
		SliceLit(nil, 0).EndInit(1)
	pkg.NewVarStart(nil, "d").
		Val(1).
		SliceLit(types.NewSlice(types.Typ[types.Int]), 1).EndInit(1)
	domTest(t, pkg, `package main

var a = []string{"a", "b"}
var b = []float64{1, 1.2, 3}
var c = []interface {
}{}
var d = []int{1}
`)
}

func TestNamedSliceLit(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.NewType("foo").InitType(pkg, types.NewSlice(types.Typ[types.Int]))
	bar := pkg.AliasType("bar", foo)
	pkg.NewVarStart(foo, "a").
		Val(1).
		SliceLit(foo, 1).EndInit(1)
	pkg.NewVarStart(bar, "b").
		Val(1).
		SliceLit(bar, 1).EndInit(1)
	domTest(t, pkg, `package main

type foo []int
type bar = foo

var a foo = foo{1}
var b bar = bar{1}
`)
}

func TestKeyValModeLit(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewVarStart(nil, "a").
		None().Val(1).Val(3).Val(3.4).None().Val(5).
		ArrayLit(types.NewArray(types.Typ[types.Float64], -1), 6, true).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(2).Val(1.2).None().Val(3).Val(6).Val(4.5).
		SliceLit(types.NewSlice(types.Typ[types.Float64]), 6, true).EndInit(1)
	domTest(t, pkg, `package main

var a = [...]float64{1, 3: 3.4, 5}
var b = []float64{2: 1.2, 3, 6: 4.5}
`)
}

func TestNamedArrayLit(t *testing.T) {
	pkg := newMainPackage()
	foo := pkg.NewType("foo").InitType(pkg, types.NewArray(types.Typ[types.String], 2))
	bar := pkg.AliasType("bar", foo)
	pkg.NewVarStart(foo, "a").
		Val("a").Val("b").ArrayLit(foo, 2).EndInit(1)
	pkg.NewVarStart(bar, "b").
		Val("a").Val("b").ArrayLit(bar, 2).EndInit(1)
	domTest(t, pkg, `package main

type foo [2]string
type bar = foo

var a foo = foo{"a", "b"}
var b bar = bar{"a", "b"}
`)
}

func TestArrayLit(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewVarStart(nil, "a").
		Val("a").Val("b").ArrayLit(types.NewArray(types.Typ[types.String], 2), 2).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(1).Val(1.2).Val(3).ArrayLit(types.NewArray(types.Typ[types.Float64], -1), 3).EndInit(1)
	pkg.NewVarStart(nil, "c").
		ArrayLit(types.NewArray(gox.TyEmptyInterface, 10), 0).EndInit(1)
	domTest(t, pkg, `package main

var a = [2]string{"a", "b"}
var b = [...]float64{1, 1.2, 3}
var c = [10]interface {
}{}
`)
}

func TestConst(t *testing.T) {
	pkg := newMainPackage()
	tv := pkg.ConstStart().Val(1).Val(2).BinaryOp(token.ADD).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(3)) {
		t.Fatal("TestConst: != 3, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val("1").Val("2").BinaryOp(token.ADD).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeString("12")) {
		t.Fatal("TestConst: != 12, it is", tv.Value)
	}
}

func TestConstLenCap(t *testing.T) {
	pkg := newMainPackage()
	typ := types.NewArray(types.Typ[types.Int], 10)
	typAP := types.NewPointer(typ)
	pkg.Types.Scope().Insert(types.NewVar(token.NoPos, pkg.Types, "array", typ))
	pkg.Types.Scope().Insert(types.NewVar(token.NoPos, pkg.Types, "parray", typAP))
	tv := pkg.ConstStart().Val(pkg.Builtin().Ref("len")).Val(pkg.Ref("array")).Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(10)) {
		t.Fatal("TestConst: != 10, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val(pkg.Builtin().Ref("len")).Val(pkg.Ref("parray")).Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(10)) {
		t.Fatal("TestConst: != 10, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val(pkg.Builtin().Ref("len")).Val("Hi").Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(2)) {
		t.Fatal("TestConst: != 2, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val(pkg.Builtin().Ref("cap")).Val(pkg.Ref("array")).Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(10)) {
		t.Fatal("TestConst: != 10, it is", tv.Value)
	}
	tv = pkg.ConstStart().Val(pkg.Builtin().Ref("cap")).Val(pkg.Ref("parray")).Call(1).EndConst()
	if constant.Compare(tv.Value, token.NEQ, constant.MakeInt64(10)) {
		t.Fatal("TestConst: != 10, it is", tv.Value)
	}
}

func TestConstDecl(t *testing.T) {
	pkg := newMainPackage()
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
	pkg := newMainPackage()
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
	pkg := newMainPackage()
	pkg.NewVar(types.Typ[types.String], "x")
	domTest(t, pkg, `package main

var x string
`)
}

func TestVarDeclInFunc(t *testing.T) {
	pkg := newMainPackage()
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
	pkg := newMainPackage()
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
	pkg := newMainPackage()
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
	pkg := newMainPackage()
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

func TestInitFunc(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "init", nil, nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "init", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func init() {
}
func init() {
}
`)
}

func TestFuncAsParam(t *testing.T) {
	pkg := newMainPackage()
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
	var a, n *goxVar
	pkg := newMainPackage()
	builtin := pkg.Builtin()
	v := pkg.NewParam("v", types.NewSlice(types.Typ[types.Int]))
	array := pkg.NewParam("array", types.NewArray(types.Typ[types.Int], 10))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v, array), nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "a", &a).NewAutoVar(token.NoPos, "n", &n).
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

func TestAppend(t *testing.T) {
	pkg := newMainPackage()
	builtin := pkg.Builtin()
	tySlice := types.NewSlice(types.Typ[types.Int])
	pkg.NewFunc(nil, "foo", gox.NewTuple(pkg.NewParam("a", tySlice)), nil, false).BodyStart(pkg).
		NewVar(tySlice, "b").VarRef(ctxRef(pkg, "b")).Val(builtin.Ref("append")).
		Val(ctxRef(pkg, "b")).Val(ctxRef(pkg, "a")).Call(2, true).Assign(1).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(a []int) {
	var b []int
	b = append(b, a...)
}
func main() {
}
`)
}

func TestAppendString(t *testing.T) {
	pkg := newMainPackage()
	builtin := pkg.Builtin()
	tySlice := types.NewSlice(types.Typ[types.Byte])
	pkg.NewFunc(nil, "foo", gox.NewTuple(pkg.NewParam("a", tySlice)), nil, false).BodyStart(pkg).
		VarRef(ctxRef(pkg, "a")).Val(builtin.Ref("append")).
		Val(ctxRef(pkg, "a")).Val("Hi").Call(2, true).Assign(1).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(a []uint8) {
	a = append(a, "Hi"...)
}
func main() {
}
`)
}

func TestOverloadFunc(t *testing.T) {
	var f, g, x, y *goxVar
	pkg := newMainPackage()
	builtin := pkg.Builtin()
	c64 := pkg.NewParam("c64", types.Typ[types.Complex64])
	c128 := pkg.NewParam("c128", types.Typ[types.Complex128])
	pkg.NewFunc(nil, "foo", gox.NewTuple(c64, c128), nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "f", &f).NewAutoVar(token.NoPos, "g", &g).
		NewAutoVar(token.NoPos, "x", &x).NewAutoVar(token.NoPos, "y", &y).
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
	pkg := newMainPackage()
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
	pkg := newMainPackage()
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
	}
	tyInterf := types.NewInterfaceType(methods, nil).Complete()
	bar := pkg.NewType("bar").InitType(pkg, tyInterf)
	b := pkg.NewParam("b", bar)
	v := pkg.NewParam("v", types.NewSlice(tyInterf))
	pkg.NewFunc(nil, "foo", gox.NewTuple(b, v), nil, true).BodyStart(pkg).
		Val(b).MemberVal("Bar").Call(0).EndStmt().
		Val(v).Val(0).Index(1, false).MemberVal("Bar").Call(0).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

type bar interface {
	Bar()
}

func foo(b bar, v ...interface {
	Bar()
}) {
	b.Bar()
	v[0].Bar()
}
func main() {
}
`)
}

func TestFuncCall(t *testing.T) {
	pkg := newMainPackage()
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
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	v := pkg.NewParam("v", types.NewSlice(types.NewInterfaceType(nil, nil).Complete()))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val(v).Call(1, true).EndStmt().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(v ...interface {
}) {
	fmt.Println(v...)
}
func main() {
}
`)
}

func TestDelayedLoadUnused(t *testing.T) {
	pkg := newMainPackage()
	println := gox.NewOverloadFunc(token.NoPos, pkg.Types, "println", pkg.Import("fmt").Ref("Println"))
	pkg.Types.Scope().Insert(println)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		End()
	domTest(t, pkg, `package main

func foo(format string, args ...interface {
}) (int, error) {
}
`)
}

func TestDelayedLoadUsed(t *testing.T) {
	pkg := newMainPackage()
	printf := gox.NewOverloadFunc(token.NoPos, pkg.Types, "printf", pkg.Import("fmt").Ref("Printf"))
	pkg.Types.Scope().Insert(printf)
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		Val(printf).Val(format).Val(args).Call(2, true).Return(1).
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(format string, args ...interface {
}) (int, error) {
	return fmt.Printf(format, args...)
}
`)
}

func TestIf(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ If().DefineVarStart("x").Val(3).EndInit(1).
		/******/ Val(ctxRef(pkg, "x")).Val(1).BinaryOp(token.GTR).Then().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val("OK!").Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	if x := 3; x > 1 {
		fmt.Println("OK!")
	}
}
`)
}

func TestIfElse(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ If().DefineVarStart("x").Val(3).EndInit(1).
		/******/ Val(ctxRef(pkg, "x")).Val(1).BinaryOp(token.GTR).Then().
		/******/ Val(fmt.Ref("Println")).Val("OK!").Call(1).EndStmt().
		/**/ Else().
		/******/ Val(fmt.Ref("Println")).Val("Error!").Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	if x := 3; x > 1 {
		fmt.Println("OK!")
	} else {
		fmt.Println("Error!")
	}
}
`)
}

func TestGoto(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Label("retry").Goto("retry").
		End()
	domTest(t, pkg, `package main

func main() {
retry:
	goto retry
}
`)
}

func TestBreakContinue(t *testing.T) { // TODO: check invalid syntax
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Label("retry").Break("").Continue("").
		Break("retry").Continue("retry").
		End()
	domTest(t, pkg, `package main

func main() {
retry:
	break
	continue
	break retry
	continue retry
}
`)
}

func TestGoDefer(t *testing.T) { // TODO: check invalid syntax
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hi").Call(1).Go().
		Val(fmt.Ref("Println")).Val("Go+").Call(1).Defer().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	go fmt.Println("Hi")
	defer fmt.Println("Go+")
}
`)
}

func TestSwitch(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ Switch().DefineVarStart("x").Val(3).EndInit(1).Val(ctxRef(pkg, "x")).Then(). // switch x := 3; x {
		/**/ Val(1).Val(2).Case(2). // case 1, 2:
		/******/ Val(fmt.Ref("Println")).Val("1 or 2").Call(1).EndStmt().
		/******/ End().
		/**/ Val(3).Case(1). // case 3:
		/******/ Val(fmt.Ref("Println")).Val("3").Call(1).EndStmt().
		/******/ End().
		/**/ Case(0). // default:
		/******/ Val(fmt.Ref("Println")).Val("other").Call(1).EndStmt().
		/******/ End().
		/**/ End(). // end switch
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	switch x := 3; x {
	case 1, 2:
		fmt.Println("1 or 2")
	case 3:
		fmt.Println("3")
	default:
		fmt.Println("other")
	}
}
`)
}

func TestSwitchNoTag(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ DefineVarStart("x").Val(3).EndInit(1).
		/**/ Switch().None().Then(). // switch {
		/**/ Val(ctxRef(pkg, "x")).Val(2).BinaryOp(token.EQL).Case(1). // case x == 2:
		/******/ Val(fmt.Ref("Println")).Val("x = 2").Call(1).EndStmt().
		/******/ Fallthrough().
		/******/ End().
		/**/ Val(ctxRef(pkg, "x")).Val(3).BinaryOp(token.LSS).Case(1). // case x < 3:
		/******/ Val(fmt.Ref("Println")).Val("x < 3").Call(1).EndStmt().
		/******/ End().
		/**/ Case(0). // default:
		/******/ Val(fmt.Ref("Println")).Val("other").Call(1).EndStmt().
		/******/ End().
		/**/ End(). // end switch
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	x := 3
	switch {
	case x == 2:
		fmt.Println("x = 2")
		fallthrough
	case x < 3:
		fmt.Println("x < 3")
	default:
		fmt.Println("other")
	}
}
`)
}

func TestFor(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ For().DefineVarStart("i").Val(0).EndInit(1). // for i := 0; i < len("Hello"); i=i+1 {
		/******/ Val(ctxRef(pkg, "i")).Val(ctxRef(pkg, "len")).Val("Hello").Call(1).BinaryOp(token.LSS).Then().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "i")).Call(1).EndStmt().
		/******/ Post().
		/******/ VarRef(ctxRef(pkg, "i")).Val(ctxRef(pkg, "i")).Val(1).BinaryOp(token.ADD).Assign(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	for i := 0; i < len("Hello"); i = i + 1 {
		fmt.Println(i)
	}
}
`)
}

func TestLoopFor(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ For().None().Then(). // for {
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	for {
		fmt.Println("Hi")
	}
}
`)
}

func TestLabeledFor(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Label("label").
		/**/ For().DefineVarStart("i").Val(0).EndInit(1). // for i := 0; i < 10; i=i+1 {
		/******/ Val(ctxRef(pkg, "i")).Val(10).BinaryOp(token.LSS).Then().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "i")).Call(1).EndStmt().
		/******/ Post().
		/******/ VarRef(ctxRef(pkg, "i")).Val(ctxRef(pkg, "i")).Val(1).BinaryOp(token.ADD).Assign(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
label:
	for i := 0; i < 10; i = i + 1 {
		fmt.Println(i)
	}
}
`)
}

func TestForRange(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a").Val(1).Val(1.2).Val(3).SliceLit(nil, 3).EndInit(1).
		/**/ ForRange("i").Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "i")).Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	a := []float64{1, 1.2, 3}
	for i := range a {
		fmt.Println(i)
	}
}
`)
}

func TestForRangeChan(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.NewChan(types.SendRecv, types.Typ[types.Int]), "a").
		/**/ ForRange("_", "i").Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "i")).Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var a chan int
	for i := range a {
		fmt.Println(i)
	}
}
`)
}

func TestForRangeKV(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a").Val(1).Val(1.2).Val(3).ArrayLit(types.NewArray(types.Typ[types.Float64], 3), 3).EndInit(1).
		/**/ ForRange("_", "x").Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "x")).Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	a := [3]float64{1, 1.2, 3}
	for _, x := range a {
		fmt.Println(x)
	}
}
`)
}

func TestForRangeArrayPointer(t *testing.T) {
	pkg := newMainPackage()
	v := pkg.NewParam("a", types.NewPointer(types.NewArray(types.Typ[types.Float64], 3)))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, false).BodyStart(pkg).
		/**/ ForRange("_", "x").Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "x")).Call(1).EndStmt().
		/**/ End().
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(a *[3]float64) {
	for _, x := range a {
		fmt.Println(x)
	}
}
func main() {
}
`)
}

func TestForRangeNoAssign(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a").Val(1).Val(1.2).Val(3).SliceLit(nil, 3).EndInit(1).
		/**/ ForRange().Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	a := []float64{1, 1.2, 3}
	for range a {
		fmt.Println("Hi")
	}
}
`)
}

func TestForRangeAssignKV(t *testing.T) {
	pkg := newMainPackage()
	tyString := types.Typ[types.String]
	tyInt := types.Typ[types.Int]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyString, "k").NewVar(tyInt, "v").
		DefineVarStart("a").Val("a").Val(1).Val("b").Val(3).MapLit(nil, 4).EndInit(1).
		/**/ ForRange().VarRef(ctxRef(pkg, "k")).VarRef(ctxRef(pkg, "v")).Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "k")).Val(ctxRef(pkg, "v")).Call(2).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var k string
	var v int
	a := map[string]int{"a": 1, "b": 3}
	for k, v = range a {
		fmt.Println(k, v)
	}
}
`)
}

func TestForRangeAssign(t *testing.T) {
	pkg := newMainPackage()
	tyBool := types.Typ[types.Bool]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(tyBool, "k").
		NewVar(types.NewChan(types.SendRecv, tyBool), "a").
		/**/ ForRange().VarRef(ctxRef(pkg, "k")).Val(ctxRef(pkg, "a")).RangeAssignThen().
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val(ctxRef(pkg, "k")).Call(1).EndStmt().
		/**/ End().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var k bool
	var a chan bool
	for k = range a {
		fmt.Println(k)
	}
}
`)
}

func TestReturn(t *testing.T) {
	pkg := newMainPackage()
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", types.Universe.Lookup("error").Type())
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		Val(pkg.Import("fmt").Ref("Printf")).Val(format).Val(args).Call(2, true).Return(1).
		End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo(format string, args ...interface {
}) (int, error) {
	return fmt.Printf(format, args...)
}
func main() {
}
`)
}

func TestReturnExpr(t *testing.T) {
	pkg := newMainPackage()
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
}) (int, error) {
	return 0, nil
}
func main() {
}
`)
}

func TestReturnNamedResults(t *testing.T) {
	pkg := newMainPackage()
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
	pkg := newMainPackage(true)
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
	pkg := newMainPackage()
	pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func main() {
}
`)
}

func TestImportAnyWhere(t *testing.T) {
	pkg := newMainPackage()

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
	var x *goxVar
	pkg := newMainPackage()
	strings := pkg.Import("strings")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "x", &x).
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

func TestPkgVar(t *testing.T) {
	pkg := newMainPackage()
	flag := pkg.Import("flag")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(flag.Ref("Usage")).Val(nil).Assign(1).
		End()
	domTest(t, pkg, `package main

import flag "flag"

func main() {
	flag.Usage = nil
}
`)
}

func TestStructMember(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	typ := types.NewStruct(fields, nil)
	foo := pkg.NewType("foo").InitType(pkg, typ)
	mflagUnk, mflagY := 0, 0
	pkg.NewVarStart(nil, "a").
		Val(123).Val("Hi").
		StructLit(typ, 2, false).EndInit(1)
	pkg.NewVarStart(nil, "b").
		Val(ctxRef(pkg, "a")).
		MemberVal("unknown", &mflagUnk).
		MemberVal("y", &mflagY).EndInit(1)
	pkg.NewVarStart(nil, "c").
		Val(123).Val("Hi").
		StructLit(foo, 2, false).EndInit(1)
	pkg.NewVarStart(nil, "d").
		Val(ctxRef(pkg, "c")).MemberVal("x").EndInit(1)
	pkg.NewVarStart(nil, "e").
		Val(ctxRef(pkg, "a")).UnaryOp(token.AND).EndInit(1)
	pkg.NewVarStart(nil, "f").
		Val(ctxRef(pkg, "e")).MemberVal("x").EndInit(1)
	pkg.NewVarStart(nil, "g").
		Val(ctxRef(pkg, "c")).UnaryOp(token.AND).EndInit(1)
	pkg.NewVarStart(nil, "h").
		Val(ctxRef(pkg, "g")).MemberVal("y").EndInit(1)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(ctxRef(pkg, "c")).MemberRef("x").Val(1).Assign(1).
		Val(ctxRef(pkg, "a")).MemberRef("y").Val("1").Assign(1).
		End()
	if mflagUnk != 0 || mflagY != gox.MFlagVar {
		t.Fatal(`mflagUnk != 0 || mflagY != gox.MFlagVar ?`)
	}
	domTest(t, pkg, `package main

type foo struct {
	x int
	y string
}

var a = struct {
	x int
	y string
}{123, "Hi"}
var b = a.y
var c = foo{123, "Hi"}
var d = c.x
var e = &a
var f = e.x
var g = &c
var h = g.y

func main() {
	c.x = 1
	a.y = "1"
}
`)
}

func TestSlice(t *testing.T) {
	pkg := newMainPackage()

	tySlice := types.NewSlice(types.Typ[types.Int])
	tyArray := types.NewArray(types.Typ[types.Int], 10)
	tyPArray := types.NewPointer(tyArray)
	tyString := types.Typ[types.String]
	p := pkg.NewParam("p", tyPArray)
	x := pkg.NewParam("x", tySlice)
	y := pkg.NewParam("y", tyString)
	z := pkg.NewParam("z", tyArray)
	pkg.NewFunc(nil, "foo", gox.NewTuple(p, x, y, z), nil, false).BodyStart(pkg).
		NewVarStart(tySlice, "a").Val(x).None().Val(2).Slice(false).EndInit(1).
		NewVarStart(tySlice, "b").Val(x).None().None().Slice(false).EndInit(1).
		NewVarStart(tySlice, "c").Val(x).Val(1).Val(3).Val(10).Slice(true).EndInit(1).
		NewVarStart(tyString, "d").Val(y).Val(1).Val(3).Slice(false).EndInit(1).
		NewVarStart(tySlice, "e").Val(p).None().Val(5).Slice(false).EndInit(1).
		NewVarStart(tySlice, "f").Val(z).None().Val(5).Slice(false).EndInit(1).
		End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(p *[10]int, x []int, y string, z [10]int) {
	var a []int = x[:2]
	var b []int = x[:]
	var c []int = x[1:3:10]
	var d string = y[1:3]
	var e []int = p[:5]
	var f []int = z[:5]
}
func main() {
}
`)
}

func TestIndex(t *testing.T) {
	pkg := newMainPackage()

	x := pkg.NewParam("x", types.NewSlice(types.Typ[types.Int]))
	y := pkg.NewParam("y", types.NewMap(types.Typ[types.String], types.Typ[types.Int]))
	ret := pkg.NewParam("", types.Typ[types.Int])
	pkg.NewFunc(nil, "foo", gox.NewTuple(x, y), gox.NewTuple(ret), false).BodyStart(pkg).
		DefineVarStart("v", "ok").Val(y).Val("a").Index(1, true).EndInit(1).
		Val(x).Val(0).Index(1, false).Return(1).
		End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(x []int, y map[string]int) int {
	v, ok := y["a"]
	return x[0]
}
func main() {
}
`)
}

func TestIndexRef(t *testing.T) {
	pkg := newMainPackage()

	tyArray := types.NewArray(types.Typ[types.Int], 10)
	x := pkg.NewParam("x", tyArray)
	y := pkg.NewParam("y", types.NewPointer(tyArray))
	z := pkg.NewParam("z", types.NewMap(types.Typ[types.String], types.Typ[types.Int]))
	pkg.NewFunc(nil, "foo", gox.NewTuple(x, y, z), nil, false).BodyStart(pkg).
		Val(x).Val(0).IndexRef(1).Val(1).Assign(1).
		Val(y).Val(1).IndexRef(1).Val(2).Assign(1).
		Val(z).Val("a").IndexRef(1).Val(3).Assign(1).
		Val(y).ElemRef().Val(x).Assign(1).
		VarRef(x).Val(y).Elem().Assign(1).
		End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(x [10]int, y *[10]int, z map[string]int) {
	x[0] = 1
	y[1] = 2
	z["a"] = 3
	*y = x
	x = *y
}
func main() {
}
`)
}

func TestStar(t *testing.T) {
	pkg := newMainPackage()
	tyInt := types.Typ[types.Uint32]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart("a").Typ(tyInt).Star().Val(nil).Call(1).EndInit(1).
		NewVarStart(tyInt, "b").Val(ctxRef(pkg, "a")).Star().EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	a := (*uint32)(nil)
	var b uint32 = *a
}
`)
}

func TestAssignOp(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.Typ[types.String], "a", "b").
		VarRef(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).AssignOp(token.ADD_ASSIGN).
		End()
	domTest(t, pkg, `package main

func main() {
	var a, b string
	a += b
}
`)
}

func TestAssign(t *testing.T) {
	var a, b, c, d, e, f, g *goxVar
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "a", &a).NewAutoVar(token.NoPos, "b", &b).
		NewAutoVar(token.NoPos, "c", &c).NewAutoVar(token.NoPos, "d", &d).
		NewAutoVar(token.NoPos, "e", &e).NewAutoVar(token.NoPos, "f", &f).
		NewAutoVar(token.NoPos, "g", &g).
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
	var n, err *goxVar
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "n", &n).NewAutoVar(token.NoPos, "err", &err).
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
	var err *goxVar
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "err", &err).
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
	var a, b, c, d *goxVar
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "a", &a).NewAutoVar(token.NoPos, "b", &b).
		NewAutoVar(token.NoPos, "c", &c).NewAutoVar(token.NoPos, "d", &d).
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
	var a *goxVar
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "a", &a).
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
	var a *goxVar
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewAutoVar(token.NoPos, "a", &a).
		VarRef(a).Val("Hi").Val("!").BinaryOp(token.ADD).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a string
	a = "Hi" + "!"
}
`)
}

func TestBinaryOpCmpNil(t *testing.T) {
	pkg := newMainPackage()
	typ := types.NewSlice(gox.TyEmptyInterface)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(typ, "a").
		NewVarStart(types.Typ[types.Bool], "b").
		Val(ctxRef(pkg, "a")).Val(nil).BinaryOp(token.NEQ).EndInit(1).
		NewVarStart(types.Typ[types.Bool], "c").
		Val(nil).Val(ctxRef(pkg, "a")).BinaryOp(token.EQL).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a []interface {
	}
	var b bool = a != nil
	var c bool = a == nil
}
`)
}

func TestClosure(t *testing.T) {
	pkg := newMainPackage()
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

func TestClosureAutoParamRet(t *testing.T) {
	pkg := newMainPackage()
	ret := pkg.NewAutoParam("ret")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(types.NewSlice(types.Typ[types.Int]), "a").
		/**/ NewClosure(nil, gox.NewTuple(ret), false).BodyStart(pkg).
		/******/ VarRef(ctxRef(pkg, "ret")).Val(pkg.Builtin().Ref("append")).Val(ctxRef(pkg, "ret")).Val(1).Call(2).Assign(1).
		/******/ Return(0).
		/**/ End().Call(0).EndInit(1).
		End()
	domTest(t, pkg, `package main

func main() {
	var a []int = func() (ret []int) {
		ret = append(ret, 1)
		return
	}()
}
`)
}

func TestReturnErr(t *testing.T) {
	pkg := newMainPackage()
	tyErr := types.Universe.Lookup("error").Type()
	format := pkg.NewParam("format", types.Typ[types.String])
	args := pkg.NewParam("args", types.NewSlice(gox.TyEmptyInterface))
	n := pkg.NewParam("", types.Typ[types.Int])
	err := pkg.NewParam("", tyErr)
	pkg.NewFunc(nil, "foo", gox.NewTuple(format, args), gox.NewTuple(n, err), true).BodyStart(pkg).
		NewVar(tyErr, "_gop_err").
		Val(ctxRef(pkg, "_gop_err")).ReturnErr(false).
		End()
	domTest(t, pkg, `package main

func foo(format string, args ...interface {
}) (int, error) {
	var _gop_err error
	return 0, _gop_err
}
`)
}

func TestCallInlineClosure(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	ret := pkg.NewAutoParam("ret")
	err := pkg.NewParam("", gox.TyError)
	sig := types.NewSignature(nil, nil, types.NewTuple(ret), false)
	pkg.NewFunc(nil, "foo", nil, types.NewTuple(err), false).BodyStart(pkg).
		DefineVarStart("n").
		CallInlineClosureStart(sig, 0, false).
		/**/ DefineVarStart("n", "err").Val(fmt.Ref("Println")).Val("Hi").Call(1).EndInit(1).
		/**/ If().Val(ctxRef(pkg, "err")).CompareNil(token.NEQ).Then().
		/******/ Val(ctxRef(pkg, "err")).ReturnErr(true).
		/******/ End().
		/**/ Val(ctxRef(pkg, "n")).Return(1).
		/**/ End().
		EndInit(1).
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func foo() error {
	var _autoGo_1 int
	{
		n, err := fmt.Println("Hi")
		if err != nil {
			return err
		}
		_autoGo_1 = n
		goto _autoGo_2
	_autoGo_2:
	}
	n := _autoGo_1
}
`)
}

func TestCallInlineClosureAssign(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	ret := pkg.NewAutoParam("ret")
	sig := types.NewSignature(nil, nil, gox.NewTuple(ret), false)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).
		CallInlineClosureStart(sig, 0, false).
		/**/ NewVar(types.Universe.Lookup("error").Type(), "err").
		/**/ VarRef(ret).VarRef(ctxRef(pkg, "err")).Val(fmt.Ref("Println")).Val("Hi").Call(1).Assign(2, 1).
		/**/ Return(0).
		/**/ End().
		Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var _autoGo_1 int
	{
		var err error
		_autoGo_1, err = fmt.Println("Hi")
		goto _autoGo_2
	_autoGo_2:
	}
	fmt.Println(_autoGo_1)
}
`)
}

func TestCallInlineClosureEllipsis(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	x := pkg.NewParam("x", types.NewSlice(gox.TyEmptyInterface))
	ret := pkg.NewAutoParam("ret")
	sig := types.NewSignature(nil, types.NewTuple(x), types.NewTuple(ret), true)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).
		Val(1).SliceLit(types.NewSlice(gox.TyEmptyInterface), 1).
		CallInlineClosureStart(sig, 1, true).
		/**/ DefineVarStart("n", "err").Val(fmt.Ref("Println")).Val(x).Call(1, true).EndInit(1).
		/**/ Val(ctxRef(pkg, "n")).Return(1).
		/**/ End().
		Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	var _autoGo_1 int
	{
		var _autoGo_2 []interface {
		} = []interface {
		}{1}
		n, err := fmt.Println(_autoGo_2...)
		_autoGo_1 = n
		goto _autoGo_3
	_autoGo_3:
	}
	fmt.Println(_autoGo_1)
}
`)
}

// ----------------------------------------------------------------------------

func ctxRef(pkg *gox.Package, name string) gox.Ref {
	_, o := pkg.CB().Scope().LookupParent(name, token.NoPos)
	return o
}

func TestExample(t *testing.T) {
	pkg := newMainPackage()

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
