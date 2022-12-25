//go:build go1.18
// +build go1.18

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
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox/packages"

	"github.com/goplus/gox"
)

type importer struct {
	packages map[string]*types.Package
	imp      types.Importer
}

func (i *importer) Import(path string) (*types.Package, error) {
	if pkg, ok := i.packages[path]; ok {
		return pkg, nil
	}
	return i.imp.Import(path)
}

type goxTest struct {
	fset *token.FileSet
	imp  *importer
}

func newGoxTest() *goxTest {
	fset := token.NewFileSet()
	return &goxTest{
		fset: fset,
		imp: &importer{
			packages: make(map[string]*types.Package),
			imp:      packages.NewImporter(fset),
		},
	}
}

func (p *goxTest) LoadGoPackage(pkgPath string, filename string, src string) (*types.Package, error) {
	if src == "" {
		return p.imp.imp.Import(pkgPath)
	}
	f, err := parser.ParseFile(p.fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	conf := types.Config{Importer: p.imp}
	pkg, err := conf.Check(pkgPath, p.fset, []*ast.File{f}, nil)
	if err != nil {
		return nil, err
	}
	p.imp.packages[pkgPath] = pkg
	return pkg, nil
}

func (p *goxTest) NewPackage(pkgPath string, name string) *gox.Package {
	conf := &gox.Config{
		Fset:            p.fset,
		Importer:        p.imp,
		NodeInterpreter: nodeInterp{},
	}
	return gox.NewPackage(pkgPath, name, conf)
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

var (
	SumInt = Sum[int]
	AtInt = At[[]int,int]
)
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fnSum := pkgRef.Ref("Sum")
	tySumInt := pkgRef.Ref("SumInt").Type()
	tyInt := types.Typ[types.Int]
	tyIntSlice := types.NewSlice(tyInt)
	fnAt := pkgRef.Ref("At")
	tyAtInt := pkgRef.Ref("AtInt").Type()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tySumInt, "sum").Val(fnSum).Typ(tyInt).Index(1, false).EndInit(1).
		NewVarStart(tyAtInt, "at").Val(fnAt).Typ(tyIntSlice).Typ(tyInt).Index(2, false).EndInit(1).
		End()
	domTest(t, pkg, `package main

import foo "foo"

func main() {
	var sum func(vec []int) int = foo.Sum[int]
	var at func(x []int, i int) int = foo.At[[]int, int]
}
`)
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

import foo "foo"

func main() {
	var data *foo.Data[int] = (*foo.Data[int])(nil)
	var slice *foo.Slice[[]int, int] = (*foo.Slice[[]int, int])(nil)
}
`)
}

func TestTypeParamsInfer(t *testing.T) {
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

var	AtInt = At[[]int]
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
	tyAtInt := pkgRef.Ref("AtInt").Type()
	tyInt := types.Typ[types.Int]
	tyIntSlice := types.NewSlice(tyInt)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		VarRef(nil).Val(fnAt).Typ(tyIntSlice).Index(1, false).Assign(1, 1).
		NewVarStart(tyAtInt, "at").Val(fnAt).Typ(tyIntSlice).Index(1, false).EndInit(1).
		NewVarStart(tyInt, "v1").Val(fnSum).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Call(1).EndInit(1).
		NewVarStart(tyInt, "v2").Val(fnAt).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		End()
	domTest(t, pkg, `package main

import foo "foo"

func main() {
	_ = foo.At[[]int]
	var at func(x []int, i int) int = foo.At[[]int]
	var v1 int = foo.Sum([]int{1, 2, 3})
	var v2 int = foo.At([]int{1, 2, 3}, 1)
}
`)
}

func TestTypeParamsCall(t *testing.T) {
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
	tyInt := types.Typ[types.Int]
	tyIntSlice := types.NewSlice(tyInt)
	tyIntPointer := types.NewPointer(tyInt)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyInt, "v1").Val(fnSum).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Call(1).EndInit(1).
		NewVarStart(tyInt, "v2").Val(fnSum).Typ(tyInt).Index(1, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Call(1).EndInit(1).
		NewVarStart(tyInt, "v3").Val(fnAt).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyInt, "v4").Val(fnAt).Typ(tyIntSlice).Index(1, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyInt, "v5").Val(fnAt).Typ(tyIntSlice).Typ(tyInt).Index(2, false).Val(1).Val(2).Val(3).SliceLit(tyIntSlice, 3).Val(1).Call(2).EndInit(1).
		NewVarStart(tyIntPointer, "p1").Val(fnLoader).Typ(tyIntPointer).Index(1, false).Val(nil).Val(1).Call(2).EndInit(1).
		NewVarStart(tyIntPointer, "p2").Val(fnLoader).Typ(tyIntPointer).Typ(tyInt).Index(2, false).Val(nil).Val(1).Call(2).EndInit(1).
		End()
	domTest(t, pkg, `package main

import foo "foo"

func main() {
	var v1 int = foo.Sum([]int{1, 2, 3})
	var v2 int = foo.Sum[int]([]int{1, 2, 3})
	var v3 int = foo.At([]int{1, 2, 3}, 1)
	var v4 int = foo.At[[]int]([]int{1, 2, 3}, 1)
	var v5 int = foo.At[[]int, int]([]int{1, 2, 3}, 1)
	var p1 *int = foo.Loader[*int](nil, 1)
	var p2 *int = foo.Loader[*int, int](nil, 1)
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

	defer checkErrorMessage(pkg, t, `./foo.gop:5:40: uint does not implement foo.Number`,
		`./foo.gop:5:40: uint does not implement foo.Number (uint missing in ~int | float64)`)()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "sum").Val(fnSum).Typ(tyUint).Index(1, false, source(`foo.Sum[uint]`, 5, 40)).EndInit(1).
		End()
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

	defer checkErrorMessage(pkg, t, `./foo.gop:5:40: T does not match ~[]E`,
		`./foo.gop:5:40: int does not match ~[]E`)()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyAtInt, "at").Val(fnAt).Typ(tyInt).Index(1, false, source(`foo.At[int]`, 5, 40)).EndInit(1).
		End()
}

func TestTypeParamsErrInfer(t *testing.T) {
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
	defer checkErrorMessage(pkg, t, `./foo.gop:5:40: cannot infer T2 (foo.go:3:21)`)()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(0, "v1").Val(fnLoader).Typ(tyInt).Index(1, false, source(`v1 := foo.Loader[int]`, 5, 40)).EndInit(1).
		End()
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

	defer checkErrorMessage(pkg, t, `./foo.gop:5:40: uint does not implement foo.Number (uint missing in ~int | float64)`,
		`./foo.gop:5:40: uint does not implement foo.Number`)()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fnSum).Val(1).Val(2).Val(3).SliceLit(tyUintSlice, 3).CallWith(1, 0, source(`foo.Sum([]uint{1,2,3})`, 5, 40)).EndInit(1).
		End()
}

func checkErrorMessage(pkg *gox.Package, t *testing.T, msgs ...string) func() {
	return func() {
		if e := recover(); e != nil {
			switch err := e.(type) {
			case *gox.CodeError, *gox.MatchError:
				defer recover()
				pkg.CB().ResetStmt()
				ret := err.(error).Error()
				for _, msg := range msgs {
					if ret == msg {
						return
					}
				}
				t.Fatalf("\nError: \"%s\"\nExpected: \"%s\"\n", ret, msgs)
			case *gox.ImportError:
				ret := err.Error()
				for _, msg := range msgs {
					if ret == msg {
						return
					}
				}
				t.Fatalf("\nError: \"%s\"\nExpected: \"%s\"\n", ret, msgs)
			default:
				t.Fatal("Unexpected error:", e)
			}
		} else {
			t.Fatal("no error?")
		}
	}
}
