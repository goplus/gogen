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
	"runtime"
	"testing"

	"github.com/goplus/gox/packages"
	"github.com/goplus/gox/typesutil"

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

import foo "foo"

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
	sig := typesutil.NewSignatureType(nil, nil, []*types.TypeParam{tp1, tp2, tp3}, types.NewTuple(p1, p2, p3), nil, false)
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
	msig1 := typesutil.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "m1", types.NewPointer(mt1)), []*types.TypeParam{mp2}, nil, nil, nil, false)
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

	sig := typesutil.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "r1", types.NewPointer(named)), []*types.TypeParam{tp1, tp2, tp3}, nil, types.NewTuple(p1, p2, p3), nil, false)
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
