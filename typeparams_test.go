/*
 Copyright 2022 The GoPlus Authors (goplus.org)
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
	"log"
	"runtime"
	"testing"

	"github.com/goplus/gox"
	"github.com/goplus/gox/internal/goxdbg"
)

func TestMethodToFunc(t *testing.T) {
	const src = `package hello

type Itf interface {
	X()
}

type Base struct {
}

func (p Base) F() {}

func (p *Base) PtrF() {}

type Foo struct {
	Itf
	Base
	Val byte
}

func (a Foo) Bar() int {
	return 0
}

func (a *Foo) PtrBar() string {
	return ""
}

var _ = (Foo).Bar
var _ = (*Foo).PtrBar
var _ = (Foo).F
var _ = (*Foo).PtrF
var _ = (Foo).X
var _ = (*Foo).X
var _ = (Itf).X
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("hello", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("hello")
	objFoo := pkgRef.Ref("Foo")
	objItf := pkgRef.Ref("Itf")
	typ := objFoo.Type()
	typItf := objItf.Type()
	_, err = pkg.MethodToFunc(typ, "Val")
	if err == nil || err.Error() != "-:  undefined (type hello.Foo has no method Val)" {
		t.Fatal("MethodToFunc failed:", err)
	}
	checkMethodToFunc(t, pkg, typ, "Bar", "(hello.Foo).Bar")
	checkMethodToFunc(t, pkg, types.NewPointer(typ), "PtrBar", "(*hello.Foo).PtrBar")
	checkMethodToFunc(t, pkg, typ, "PtrBar", "(*hello.Foo).PtrBar")
	checkMethodToFunc(t, pkg, typ, "F", "(hello.Foo).F")
	checkMethodToFunc(t, pkg, types.NewPointer(typ), "PtrF", "(*hello.Foo).PtrF")
	checkMethodToFunc(t, pkg, typ, "PtrF", "(*hello.Foo).PtrF")
	checkMethodToFunc(t, pkg, typItf, "X", "(hello.Itf).X")
	checkMethodToFunc(t, pkg, typ, "X", "(hello.Foo).X")
	checkMethodToFunc(t, pkg, types.NewPointer(typ), "X", "(*hello.Foo).X")
}

func checkMethodToFunc(t *testing.T, pkg *gox.Package, typ types.Type, name, code string) {
	t.Helper()
	ret, err := pkg.MethodToFunc(typ, name)
	if err != nil {
		t.Fatal("MethodToFunc failed:", err)
	}
	if _, isPtr := typ.(*types.Pointer); isPtr {
		if recv := ret.Type.(*types.Signature).Params().At(0); !types.Identical(recv.Type(), typ) {
			t.Fatalf("MethodToFunc: ResultType: %v, Expected: %v\n", recv.Type(), typ)
		}
	}
	if v := goxdbg.Format(pkg.Fset, ret.Val); v != code {
		t.Fatalf("MethodToFunc:\nResult:\n%s\nExpected:\n%s\n", v, code)
	}
}

func TestTypeAsParamsFunc(t *testing.T) {
	const src = `package foo

const GopPackage = true

type basetype interface {
	int | string
}

func Gopx_Bar[T basetype](name string) {
}

func Gopx_Row__0[T basetype](name string) {
}

func Gopx_Row__1[Array any](v int) {
}

type Table struct {
}

func Gopt_Table_Gopx_Col__0[T basetype](p *Table, name string) {
}

func Gopt_Table_Gopx_Col__1[Array any](p *Table, v int) {
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "test")
	foo := pkg.Import("foo")
	objTable := foo.Ref("Table")
	typ := objTable.Type().(*types.Named)
	tyInt := types.Typ[types.Int]

	cb := pkg.NewFunc(nil, "Example", nil, nil, false).BodyStart(pkg).
		NewVar(types.NewPointer(typ), "tbl")
	_, err = cb.VarVal("tbl").Member("col", gox.MemberFlagMethodAlias)
	if err != nil {
		t.Fatal("tbl.Member(col):", err)
	}
	cb.Typ(tyInt).Val("bar").Call(2).EndStmt().
		Val(foo.Ref("Bar")).Typ(tyInt).Val("1").Call(2).EndStmt().
		Val(foo.Ref("Row")).Typ(tyInt).Val(1, source("1")).Call(2).EndStmt().
		End()

	domTest(t, pkg, `package test

import "foo"

func Example() {
	var tbl *foo.Table
	foo.Gopt_Table_Gopx_Col__0[int](tbl, "bar")
	foo.Gopx_Bar[int]("1")
	foo.Gopx_Row__1[int](1)
}
`)
}

func TestCheckGopPkg(t *testing.T) {
	const src = `package foo

import "io"

const GopPackage = "io"

type basetype interface {
	int | string
}

func Gopx_Bar[T basetype](name string) {
}

func Gopx_Row__0[T basetype](name string) {
}

func Gopx_Row__1[Array any](v int) {
}

type EmbIntf interface {
	io.Reader
	Close()
}

type Table struct {
	EmbIntf
	N int
	b string
}

func Gopt_Table_Gopx_Col__0[T basetype](p *Table, name string) {
}

func Gopt_Table_Gopx_Col__1[Array any](p *Table, v int) {
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "test")
	foo := pkg.Import("foo")
	objTable := foo.Ref("Table")
	typ := objTable.Type().(*types.Named)
	tyInt := types.Typ[types.Int]

	typSlice := types.NewSlice(types.NewPointer(typ))
	typMap := types.NewMap(types.Typ[types.String], typSlice)

	args := types.NewTuple(types.NewParam(0, pkg.Types, "tbls", typMap))
	cb := pkg.NewFunc(nil, "Example", args, nil, false).BodyStart(pkg)
	_, err = cb.VarVal("tbls").Val("Hi").Index(1, false).Val(0).Index(1, false).Member("col", gox.MemberFlagMethodAlias)
	if err != nil {
		t.Fatal("tbl.Member(col):", err)
	}
	cb.Typ(tyInt).Val("bar").Call(2).EndStmt().
		Val(foo.Ref("Bar")).Typ(tyInt).Val("1").Call(2).EndStmt().
		Val(foo.Ref("Row")).Typ(tyInt).Val(1, source("1")).Call(2).EndStmt().
		End()

	typChan := types.NewChan(types.SendRecv, typSlice)
	typArray := types.NewArray(typChan, 2)
	args = types.NewTuple(types.NewParam(0, pkg.Types, "", typArray))
	pkg.NewFunc(nil, "Create", args, nil, false).BodyStart(pkg).End()

	domTest(t, pkg, `package test

import "foo"

const GopPackage = "foo"

func Example(tbls map[string][]*foo.Table) {
	foo.Gopt_Table_Gopx_Col__0[int](tbls["Hi"][0], "bar")
	foo.Gopx_Bar[int]("1")
	foo.Gopx_Row__1[int](1)
}
func Create([2]chan []*foo.Table) {
}
`)
}

func TestOverloadNamed(t *testing.T) {
	const src = `package foo

const GopPackage = true

type M = map[string]any

type basetype interface {
	string | int | bool | float64
}

type Var__0[T basetype] struct {
	val T
}

type Var__1[T map[string]any] struct {
	val T
}

func Gopx_Var_Cast__0[T basetype]() *Var__0[T] {
	return new(Var__0[T])
}

func Gopx_Var_Cast__1[T map[string]any]() *Var__1[T] {
	return new(Var__1[T])
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	scope := pkgRef.Types.Scope()
	log.Println("==> Lookup", scope.Lookup("Var__0"))
	objVar := pkgRef.Ref("Var")
	typ := objVar.Type()
	on, ok := gox.CheckOverloadNamed(typ)
	if !ok {
		t.Fatal("TestOverloadNamed: not TyOverloadNamed?")
	}
	if on.Types[0].TypeParams() == nil {
		t.Fatal("TestOverloadNamed: not generic")
	}

	tyInt := types.Typ[types.Int]
	tyM := pkgRef.Ref("M").Type()
	ty1 := pkg.Instantiate(typ, []types.Type{tyInt})
	ty2 := pkg.Instantiate(typ, []types.Type{tyM})
	pkg.NewTypeDefs().NewType("t1").InitType(pkg, ty1)
	pkg.NewTypeDefs().NewType("t2").InitType(pkg, ty2)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(objVar).Typ(tyInt).Call(1).EndStmt().
		Val(objVar).Typ(tyM).Call(1).EndStmt().
		End()

	domTest(t, pkg, `package main

import "foo"

type t1 foo.Var__0[int]
type t2 foo.Var__1[map[string]any]

func main() {
	foo.Gopx_Var_Cast__0[int]()
	foo.Gopx_Var_Cast__1[map[string]any]()
}
`)

	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestOverloadNamed failed: no error?")
			}
		}()
		ty3 := pkg.Instantiate(on, []types.Type{gox.TyByte})
		pkg.NewTypeDefs().NewType("t3").InitType(pkg, ty3)
	}()
	func() {
		defer func() {
			if e := recover(); e != nil && e.(error).Error() != "-: 1 (type untyped int) is not a type" {
				t.Fatal("TestOverloadNamed failed:", e)
			}
		}()
		pkg.NewFunc(nil, "bar", nil, nil, false).BodyStart(pkg).
			Val(objVar).Val(1, source("1")).Call(1).EndStmt().
			End()
	}()
}

func TestInstantiate(t *testing.T) {
	const src = `package foo

type Data[T any] struct {
	v T
}

type sliceOf[E any] interface {
	~[]E
}

type Slice[S sliceOf[T], T any] struct {
	Data S
}

func (p *Slice[S, T]) Append(t ...T) S {
	p.Data = append(p.Data, t...)
	return p.Data
}

type (
	DataInt = Data[int]
	SliceInt = Slice[[]int,int]
)
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	tyData := pkgRef.Ref("Data").Type()
	tyInt := types.Typ[types.Int]
	tyInvalid := types.Typ[types.Invalid]
	tySlice := pkgRef.Ref("Slice").Type()
	if ret := pkg.Instantiate(tyData, []types.Type{tyInt}); ret == tyInvalid {
		t.Fatal("TestInstantiate failed: pkg.Instantiate")
	}
	func() {
		defer func() {
			if e := recover(); e.(error).Error() != "-: int is not a generic type" {
				t.Fatal("TestInstantiate failed:", e)
			}
		}()
		pkg.Instantiate(tyInt, nil)
	}()
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestInstantiate failed: no error?")
			}
		}()
		pkg.Instantiate(tySlice, []types.Type{tyInt})
	}()
}

func TestTypeParamsType(t *testing.T) {
	const src = `package foo

type Data[T any] struct {
	v T
}

type sliceOf[E any] interface {
	~[]E
}

type Slice[S sliceOf[T], T any] struct {
	Data S
}

func (p *Slice[S, T]) Append(t ...T) S {
	p.Data = append(p.Data, t...)
	return p.Data
}

type (
	DataInt = Data[int]
	SliceInt = Slice[[]int,int]
)
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	tySlice := pkgRef.Ref("Slice").Type()
	tySliceInt := pkgRef.Ref("SliceInt").Type()
	tyData := pkgRef.Ref("Data").Type()
	tyDataInt := pkgRef.Ref("DataInt").Type()
	tyInt := types.Typ[types.Int]
	tyIntSlice := types.NewSlice(tyInt)

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(types.NewPointer(tyDataInt), "data").Typ(tyData).Typ(tyInt).Index(1, false).Star().Val(nil).Call(1).EndInit(1).
		NewVarStart(types.NewPointer(tySliceInt), "slice").Typ(tySlice).Typ(tyIntSlice).Typ(tyInt).Index(2, false).Star().Val(nil).Call(1).EndInit(1).
		End()
	domTest(t, pkg, `package main

import "foo"

func main() {
	var data *foo.Data[int] = (*foo.Data[int])(nil)
	var slice *foo.Slice[[]int, int] = (*foo.Slice[[]int, int])(nil)
}
`)
}

func TestTypeParamsFunc(t *testing.T) {
	const src = `package foo

type Number interface {
	~int | float64
}

func Sum[T Number](vec []T) T {
	var sum T
	for _, elt := range vec {
		sum = sum + elt
	}
	return sum
}

func At[T interface{ ~[]E }, E any](x T, i int) E {
	return x[i]
}

func Loader[T1 any, T2 any](p1 T1, p2 T2) T1 {
	return p1
}

func Add[T1 any, T2 ~int](v1 T1, v2 ...T2) (sum T2) {
	println(v1)
	for _, v := range v2 {
		sum += v
	}
	return sum
}

type Int []int
var MyInts = Int{1,2,3,4}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnSum := pkgRef.Ref("Sum")
	fnAt := pkgRef.Ref("At")
	fnLoader := pkgRef.Ref("Loader")
	fnAdd := pkgRef.Ref("Add")
	myInts := pkgRef.Ref("MyInts")
	tyInt := types.Typ[types.Int]
	tyString := types.Typ[types.String]
	tyIntSlice := types.NewSlice(tyInt)
	tyIntPointer := types.NewPointer(tyInt)
	var fn1 *types.Var
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(nil).Val(fnAt).Typ(tyIntSlice).Index(1, false).Assign(1, 1).
		VarRef(nil).Val(fnSum).Typ(tyInt).Index(1, false).Assign(1, 1).
		VarRef(nil).Val(fnLoader).Typ(tyInt).Typ(tyInt).Index(2, false).Assign(1, 1).
		VarRef(nil).Val(fnAdd).Typ(tyString).Typ(tyInt).Index(2, false).Assign(1, 1).
		NewVarStart(tyInt, "s1").Val(fnSum).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Call(1).EndInit(1).
		NewVarStart(tyInt, "s2").Val(fnSum).Typ(tyInt).Index(1, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Call(1).EndInit(1).
		NewVarStart(tyInt, "v1").Val(fnAt).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyInt, "v2").Val(fnAt).Typ(tyIntSlice).Index(1, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyInt, "v3").Val(fnAt).Typ(tyIntSlice).Typ(tyInt).Index(2, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyInt, "n1").Val(fnAdd).Val("hello").Val(1).Val(2).Val(3).Call(4).EndInit(1).
		NewVarStart(tyInt, "n2").Val(fnAdd).Typ(tyString).Index(1, false).Val("hello").Val(1).Val(2).Val(3).Call(4).EndInit(1).
		NewVarStart(tyInt, "n3").Val(fnAdd).Typ(tyString).Typ(tyInt).Index(2, false).Val("hello").Val(1).Val(2).Val(3).Call(4).EndInit(1).
		NewVarStart(tyInt, "n4").Val(fnAdd).Val("hello").Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).CallWith(2, gox.InstrFlagEllipsis).EndInit(1).
		NewVarStart(tyInt, "n5").Val(fnAdd).Val("hello").Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).CallWith(2, gox.InstrFlagEllipsis).EndInit(1).
		NewVarStart(tyInt, "n6").Val(fnAdd).Typ(tyString).Index(1, false).Val("hello").Val(myInts).CallWith(2, gox.InstrFlagEllipsis).EndInit(1).
		NewVarStart(tyInt, "n7").Val(fnAdd).Typ(tyString).Typ(tyInt).Index(2, false).Val("hello").Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).CallWith(2, gox.InstrFlagEllipsis).EndInit(1).
		NewVarStart(tyIntPointer, "p1").Val(fnLoader).Typ(tyIntPointer).Index(1, false).Val(nil).Val(1).Call(2).EndInit(1).
		NewVarStart(tyIntPointer, "p2").Val(fnLoader).Typ(tyIntPointer).Typ(tyInt).Index(2, false).Val(nil).Val(1).Call(2).EndInit(1).
		NewAutoVar(0, "fn1", &fn1).VarRef(fn1).Val(fnLoader).Typ(tyIntPointer).Typ(tyInt).Index(2, false).Assign(1, 1).EndStmt().
		Val(fn1).Val(nil).Val(1).Call(2).EndStmt().
		End()
	domTest(t, pkg, `package main

import "foo"

func main() {
	_ = foo.At[[]int]
	_ = foo.Sum[int]
	_ = foo.Loader[int, int]
	_ = foo.Add[string, int]
	var s1 int = foo.Sum([]int{1, 2, 3})
	var s2 int = foo.Sum[int]([]int{1, 2, 3})
	var v1 int = foo.At([]int{1, 2, 3}, 1)
	var v2 int = foo.At[[]int]([]int{1, 2, 3}, 1)
	var v3 int = foo.At[[]int, int]([]int{1, 2, 3}, 1)
	var n1 int = foo.Add("hello", 1, 2, 3)
	var n2 int = foo.Add[string]("hello", 1, 2, 3)
	var n3 int = foo.Add[string, int]("hello", 1, 2, 3)
	var n4 int = foo.Add("hello", []int{1, 2, 3}...)
	var n5 int = foo.Add("hello", []int{1, 2, 3}...)
	var n6 int = foo.Add[string]("hello", foo.MyInts...)
	var n7 int = foo.Add[string, int]("hello", []int{1, 2, 3}...)
	var p1 *int = foo.Loader[*int](nil, 1)
	var p2 *int = foo.Loader[*int, int](nil, 1)
	var fn1 func(p1 *int, p2 int) *int
	fn1 = foo.Loader[*int, int]
	fn1(nil, 1)
}
`)
}

func TestTypeParamsErrorInstantiate(t *testing.T) {
	const src = `package foo

type Number interface {
	~int | float64
}

func Sum[T Number](vec []T) T {
	var sum T
	for _, elt := range vec {
		sum = sum + elt
	}
	return sum
}

var	SumInt = Sum[int]
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}

	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnSum := pkgRef.Ref("Sum")
	tyUint := types.Typ[types.Uint]

	var msg string
	switch runtime.Version()[:6] {
	case "go1.18":
		msg = `./foo.gop:5:40: uint does not implement foo.Number`
	case "go1.19":
		msg = `./foo.gop:5:40: uint does not implement foo.Number (uint missing in ~int | float64)`
	default:
		msg = `./foo.gop:5:40: uint does not satisfy foo.Number (uint missing in ~int | float64)`
	}
	codeErrorTestEx(t, pkg, msg, func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			DefineVarStart(0, "sum").Val(fnSum).Typ(tyUint).Index(1, false, source(`foo.Sum[uint]`, 5, 40)).EndInit(1).
			End()
	})
}

func TestTypeParamsErrorMatch(t *testing.T) {
	const src = `package foo

func At[T interface{ ~[]E }, E any](x T, i int) E {
	return x[i]
}

var	AtInt = At[[]int]
`

	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}

	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnAt := pkgRef.Ref("At")
	tyAtInt := pkgRef.Ref("AtInt").Type()
	tyInt := types.Typ[types.Int]

	var msg string
	switch runtime.Version()[:6] {
	case "go1.18", "go1.19":
		msg = `./foo.gop:5:40: T does not match ~[]E`
	case "go1.20":
		msg = `./foo.gop:5:40: int does not match ~[]E`
	default:
		msg = `./foo.gop:5:40: T (type int) does not satisfy interface{~[]E}`
	}
	codeErrorTestEx(t, pkg, msg, func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			NewVarStart(tyAtInt, "at").Val(fnAt).Typ(tyInt).Index(1, false, source(`foo.At[int]`, 5, 40)).EndInit(1).
			End()
	})
}

func TestTypeParamsErrInferFunc(t *testing.T) {
	const src = `package foo

func Loader[T1 any, T2 any](p1 T1, p2 T2) T1 {
	return p1
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnLoader := pkgRef.Ref("Loader")
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: cannot infer T2 (foo.go:3:21)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Val(fnLoader).Typ(tyInt).Index(1, false, source(`v1 := foo.Loader[int]`, 5, 40)).EndInit(1).
				End()
		})
}

func TestTypeParamsErrArgumentsParameters1(t *testing.T) {
	const src = `package foo

type Data[T1 any, T2 any] struct {
	v1 T1
	v2 T2
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	tyData := pkgRef.Ref("Data").Type()
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: got 1 type arguments but foo.Data[T1, T2 any] has 2 type parameters`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Typ(tyData).Typ(tyInt).Index(1, false, source(`foo.Data[int]`, 5, 40)).Star().Val(nil).Call(1).EndInit(1).
				End()
		})
}

func TestTypeParamsErrArgumentsParameters2(t *testing.T) {
	const src = `package foo

type Data[T1 any, T2 any] struct {
	v1 T1
	v2 T2
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	tyData := pkgRef.Ref("Data").Type()
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: got 3 type arguments but foo.Data[T1, T2 any] has 2 type parameters`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Typ(tyData).Typ(tyInt).Typ(tyInt).Typ(tyInt).Index(3, false, source(`foo.Data[int,int,int]`, 5, 40)).Star().Val(nil).Call(1).EndInit(1).
				End()
		})
}

func TestTypeParamsErrArgumentsParameters3(t *testing.T) {
	const src = `package foo

func Test[T1 any, T2 any](t1 T1, t2 T2) {
	println(t1,t2)
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnTest := pkgRef.Ref("Test")
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: got 3 type arguments but func[T1, T2 any](t1 T1, t2 T2) has 2 type parameters`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnTest).Typ(tyInt).Typ(tyInt).Typ(tyInt).Index(3, false, source(`foo.Test[int,int,int]`, 5, 40)).Val(1).Val(1).Call(2).EndStmt().
				End()
		})
}

func TestTypeParamsErrCallArguments1(t *testing.T) {
	const src = `package foo

func Test[T1 any, T2 any](t1 T1, t2 T2) {
	println(t1,t2)
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnTest := pkgRef.Ref("Test")
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: not enough arguments in call to foo.Test
	have (untyped int)
	want (T1, T2)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnTest).Val(1).CallWith(1, 0, source("foo.Test(1)", 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamsErrCallArguments2(t *testing.T) {
	const src = `package foo

func Test[T1 any, T2 any](t1 T1, t2 T2) {
	println(t1,t2)
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnTest := pkgRef.Ref("Test")

	codeErrorTestEx(t, pkg, `./foo.gop:5:40: too many arguments in call to foo.Test
	have (untyped int, untyped int, untyped int)
	want (T1, T2)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnTest).Val(1).Val(2).Val(3).CallWith(3, 0, source("foo.Test(1,2,3)", 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamsErrCallArguments3(t *testing.T) {
	const src = `package foo

func Test[T1 any, T2 any]() {
	var t1 T1
	var t2 T2
	println(t1,t2)
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnTest := pkgRef.Ref("Test")
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: too many arguments in call to foo.Test
	have (untyped int, untyped int)
	want ()`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnTest).Val(1).Val(2).CallWith(2, 0, source("foo.Test(1,2)", 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamsErrCallVariadicArguments1(t *testing.T) {
	const src = `package foo

func Add[T1 any, T2 ~int](v1 T1, v2 ...T2) (sum T2) {
	println(v1)
	for _, v := range v2 {
		sum += v
	}
	return sum
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnAdd := pkgRef.Ref("Add")

	codeErrorTestEx(t, pkg, `./foo.gop:5:40: not enough arguments in call to foo.Add
	have ()
	want (T1, ...T2)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnAdd).CallWith(0, 0, source("foo.Add()", 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamsErrCallVariadicArguments2(t *testing.T) {
	const src = `package foo

func Add[T1 any, T2 ~int](v1 T1, v2 ...T2) (sum T2) {
	println(v1)
	for _, v := range v2 {
		sum += v
	}
	return sum
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnAdd := pkgRef.Ref("Add")

	// not pass source to foo.Add
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: cannot infer T2 (-)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnAdd).Val(1).CallWith(1, 0, source("foo.Add(1)", 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamsErrorCall(t *testing.T) {
	const src = `package foo

type Number interface {
	~int | float64
}

func Sum[T Number](vec []T) T {
	var sum T
	for _, elt := range vec {
		sum = sum + elt
	}
	return sum
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnSum := pkgRef.Ref("Sum")
	tyUint := types.Typ[types.Uint]
	tyUintSlice := types.NewSlice(tyUint)

	var msg string
	switch runtime.Version()[:6] {
	case "go1.18":
		msg = `./foo.gop:5:40: uint does not implement foo.Number`
	case "go1.19":
		msg = `./foo.gop:5:40: uint does not implement foo.Number (uint missing in ~int | float64)`
	default:
		msg = `./foo.gop:5:40: uint does not satisfy foo.Number (uint missing in ~int | float64)`
	}
	codeErrorTestEx(t, pkg, msg, func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			Val(fnSum).Val(1).Val(2).Val(3).SliceLit(tyUintSlice, 3).CallWith(1, 0, source(`foo.Sum([]uint{1,2,3})`, 5, 40)).EndInit(1).
			End()
	})
}

func TestTypeParamsErrorInferCall(t *testing.T) {
	const src = `package foo

func Loader[T1 any, T2 any](p1 T1, p2 T2) T1 {
	return p1
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnLoader := pkgRef.Ref("Loader")
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: cannot infer T2 (foo.go:3:21)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(fnLoader).Typ(tyInt).Index(1, false, source(`foo.Loader[int]`, 5, 40)).Val(10).Val(nil).CallWith(2, 0, source(`foo.Loader[int]`, 5, 40)).EndStmt().
				End()
		})
}

func TestTypeParamErrGenericType(t *testing.T) {
	const src = `package foo

type Data struct {
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	tyData := pkgRef.Ref("Data").Type()
	tyInt := types.Typ[types.Int]
	codeErrorTestEx(t, pkg, `./foo.gop:5:40: foo.Data is not a generic type`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Typ(tyData).Typ(tyInt).Index(1, false, source(`foo.Data[int]`, 5, 40)).Star().Val(nil).Call(1).EndInit(1).
				End()
		})
}

func TestTypeParamErrGenericType2(t *testing.T) {
	gt := newGoxTest()
	pkg := gt.NewPackage("", "main")
	tyInt := types.Typ[types.Int]
	tyString := types.Typ[types.String]

	codeErrorTestEx(t, pkg, `./foo.gop:5:40: string is not a generic type`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Typ(tyString).Typ(tyInt).Index(1, false, source(`string[int]`, 5, 40)).Star().Val(nil).Call(1).EndInit(1).
				End()
		})
}

func TestTypeParamErrGenericFunc(t *testing.T) {
	const src = `package foo

func Loader(n int) string {
	return ""
}
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnLoader := pkgRef.Ref("Loader")
	tyInt := types.Typ[types.Int]

	codeErrorTestEx(t, pkg, `./foo.gop:5:40: invalid operation: cannot index foo.Loader (value of type func(n int) string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "v1").Val(fnLoader).Typ(tyInt).Index(1, false, source(`v1 := foo.Loader[int]`, 5, 40)).EndInit(1).
				End()
		})
}

func TestGenTypeParamsFunc(t *testing.T) {
	pkg := newMainPackage()
	ut1 := types.NewUnion([]*types.Term{types.NewTerm(true, types.Typ[types.Int]), types.NewTerm(false, types.Typ[types.Uint])})
	ut2 := types.NewUnion([]*types.Term{types.NewTerm(true, types.Typ[types.Int])})
	it := pkg.NewType("T").InitType(pkg, types.NewInterfaceType(nil, []types.Type{ut1}))
	tp1 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T1", nil), types.Universe.Lookup("any").Type())
	tp2 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T2", nil), ut2)
	tp3 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T3", nil), it)
	p1 := types.NewParam(token.NoPos, pkg.Types, "p1", tp1)
	p2 := types.NewParam(token.NoPos, pkg.Types, "p2", tp2)
	p3 := types.NewParam(token.NoPos, pkg.Types, "p3", tp3)
	sig := types.NewSignatureType(nil, nil, []*types.TypeParam{tp1, tp2, tp3}, types.NewTuple(p1, p2, p3), nil, false)
	fn1 := pkg.NewFuncDecl(token.NoPos, "test", sig)
	fn1.BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fn1).Val("hello").Val(100).Val(200).Call(3).EndStmt().
		Val(fn1).Typ(types.Typ[types.String]).Typ(types.Typ[types.Int]).Typ(types.Typ[types.Uint]).Index(3, false).Val("hello").Val(100).Val(200).Call(3).EndStmt().
		Val(fn1).Typ(types.Typ[types.String]).Typ(types.Typ[types.Int]).Index(2, false).Val("hello").Val(100).Val(200).Call(3).EndStmt().
		Val(fn1).Typ(types.Typ[types.String]).Index(1, false).Val("hello").Val(100).Val(200).Call(3).EndStmt().
		End()

	domTest(t, pkg, `package main

type T interface {
	~int | uint
}

func test[T1 any, T2 ~int, T3 T](p1 T1, p2 T2, p3 T3) {
}
func main() {
	test("hello", 100, 200)
	test[string, int, uint]("hello", 100, 200)
	test[string, int]("hello", 100, 200)
	test[string]("hello", 100, 200)
}
`)
}

func TestGenTypeParamsType(t *testing.T) {
	pkg := newMainPackage()
	ut := types.NewUnion([]*types.Term{types.NewTerm(true, types.Typ[types.Int]), types.NewTerm(false, types.Typ[types.Uint])})
	it := pkg.NewType("T").InitType(pkg, types.NewInterfaceType(nil, []types.Type{ut}))

	// type M
	mp1 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T", nil), ut)
	mp2 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T", nil), ut)
	mt1 := pkg.NewType("M").InitType(pkg, types.NewStruct(nil, nil), mp1)
	msig1 := types.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "m1", types.NewPointer(mt1)), []*types.TypeParam{mp2}, nil, nil, nil, false)
	mfn1 := pkg.NewFuncDecl(token.NoPos, "test", msig1)
	mfn1.BodyStart(pkg).End()

	// type S
	sp1 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T1", nil), types.Universe.Lookup("any").Type())
	sp2 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T2", nil), ut)
	sp3 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T3", nil), it)

	st := types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "f1", sp1, false),
		types.NewField(token.NoPos, pkg.Types, "f2", sp2, false),
		types.NewField(token.NoPos, pkg.Types, "f3", sp3, false),
	}, nil)
	named := pkg.NewType("S").InitType(pkg, st, sp1, sp2, sp3)

	tp1 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T1", nil), types.Universe.Lookup("any").Type())
	tp2 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T2", nil), ut)
	tp3 := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T3", nil), it)
	p1 := types.NewParam(token.NoPos, pkg.Types, "p1", tp1)
	p2 := types.NewParam(token.NoPos, pkg.Types, "p2", tp2)
	p3 := types.NewParam(token.NoPos, pkg.Types, "p3", tp3)

	sig := types.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "r1", types.NewPointer(named)), []*types.TypeParam{tp1, tp2, tp3}, nil, types.NewTuple(p1, p2, p3), nil, false)
	fn1 := pkg.NewFuncDecl(token.NoPos, "test", sig)
	fn1.BodyStart(pkg).End()

	inst, err := types.Instantiate(nil, named, []types.Type{types.Typ[types.String], types.Typ[types.Int], types.Typ[types.Uint]}, true)
	if err != nil {
		t.Fatal(err)
	}
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(token.NoPos, "s").StructLit(inst, 0, false).UnaryOp(token.AND).EndInit(1).
		Val(ctxRef(pkg, "s")).MemberVal("test").Val("hello").Val(100).Val(200).Call(3, false).EndStmt().
		Val(pkg.Builtin().Ref("println")).Val(ctxRef(pkg, "s")).MemberVal("f1").Call(1, false).EndStmt().
		End()

	domTest(t, pkg, `package main

type T interface {
	~int | uint
}
type M[T ~int | uint] struct {
}

func (m1 *M[T]) test() {
}

type S[T1 any, T2 ~int | uint, T3 T] struct {
	f1 T1
	f2 T2
	f3 T3
}

func (r1 *S[T1, T2, T3]) test(p1 T1, p2 T2, p3 T3) {
}
func main() {
	s := &S[string, int, uint]{}
	s.test("hello", 100, 200)
	println(s.f1)
}
`)
}
