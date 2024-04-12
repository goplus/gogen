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

package cache

import (
	"os"
	"testing"

	"github.com/goplus/gogen/packages"
)

func dirtyPkgHash(pkgPath string, self bool) string {
	return HashInvalid
}

func nodirtyPkgHash(pkgPath string, self bool) string {
	return "1"
}

func TestBasic(t *testing.T) {
	readFile = func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	c := New(nodirtyPkgHash)
	err := c.Load("/not-found/gopkg.cache")
	if err != nil {
		t.Fatal("Load failed:", err)
	}

	p := packages.NewImporter(nil)
	p.SetCache(c)
	pkg, err := p.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import failed:", pkg, err)
	}
	if v := c.ListTimes(); v != 1 {
		t.Fatal("ListTimes:", v)
	}
	if _, err = p.Import("not-found"); err == nil {
		t.Fatal("Import not-found: no error?")
	}
	if pkg2, err := p.Import("fmt"); err != nil || pkg2 != pkg {
		t.Fatal("Import reuse fail:", pkg, pkg2)
	}
	if v := c.ListTimes(); v != 2 {
		t.Fatal("ListTimes:", v)
	}
	pkg, err = p.Import("github.com/goplus/gogen/internal/foo")
	if err != nil {
		t.Fatal("Import failed:", pkg, err)
	}
	if v := c.ListTimes(); v != 3 {
		t.Fatal("ListTimes:", v)
	}

	var result []byte
	writeFile = func(_ string, data []byte, _ os.FileMode) error {
		result = data
		return nil
	}
	readFile = func(string) ([]byte, error) {
		return result, nil
	}
	c.Save("/not-found/gopkg.cache")

	c2 := New(nodirtyPkgHash)
	if err = c2.Load("/not-found/gopkg.cache"); err != nil {
		t.Fatal("Load failed:", err)
	}
	p2 := packages.NewImporter(nil)
	p2.SetCache(c2)
	pkg, err = p2.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import failed:", pkg, err)
	}
	if v := c2.ListTimes(); v != 0 {
		t.Fatal("ListTimes:", v)
	}

	c3 := New(dirtyPkgHash)
	if err = c3.Load("/not-found/gopkg.cache"); err != nil {
		t.Fatal("Load failed:", err)
	}
	p3 := packages.NewImporter(nil)
	p3.SetCache(c3)
	pkg, err = p3.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import failed:", pkg, err)
	}
	if v := c3.ListTimes(); v != 1 {
		t.Fatal("ListTimes:", v)
	}
}

func TestIsDirty(t *testing.T) {
	c := &pkgCache{"1.a", "a", []depPkg{
		{"b", "b"},
	}}
	if !isDirty(nil, "a", c, func(pkgPath string, self bool) (hash string) {
		if self {
			return pkgPath
		}
		return ""
	}) {
		t.Fatal("isDirty: is not dirty")
	}
}

func TestParseExport(t *testing.T) {
	if _, e := parseExports("abc"); e != errInvalidFormat {
		t.Fatal("parseExports failed:", e)
	}
}

func TestErrLoadCache(t *testing.T) {
	var c Impl
	if err := c.loadCachePkgs([]string{"abc"}); err != errInvalidFormat {
		t.Fatal("loadCachePkgs failed:", err)
	}
	if err := c.loadCachePkgs([]string{"abc\tefg\thash\t3"}); err != errInvalidFormat {
		t.Fatal("loadCachePkgs failed:", err)
	}
	if err := c.loadCachePkgs([]string{"abc\tefg\thash\t1", "x"}); err != errInvalidFormat {
		t.Fatal("loadCachePkgs failed:", err)
	}
	if err := c.loadCachePkgs([]string{"abc\tefg\thash\t1", "\tx"}); err != errInvalidFormat {
		t.Fatal("loadCachePkgs failed:", err)
	}
}
