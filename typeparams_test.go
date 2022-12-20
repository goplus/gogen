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

func Sum[T interface{ int | float64 }](vec []T) T {
	var sum T
	for _, elt := range vec {
		sum = sum + elt
	}
	return sum
}

var (
	SumInt = Sum[int]
)
`
	gt := newGoxTest()
	_, err := gt.LoadGoPackage("foo", "foo.go", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := gt.NewPackage("", "main")
	pkgRef := pkg.Import("foo")
	fn := pkgRef.Ref("Sum")
	tyFn := pkgRef.Ref("SumInt").Type()
	tyInt := types.Typ[types.Int]
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(tyFn, "fn").Val(fn).Typ(tyInt).Index(1, false).EndInit(1).
		End()
	domTest(t, pkg, `package main

import foo "foo"

func main() {
	var fn func(vec []int) int = foo.Sum[int]
}
`)
}
