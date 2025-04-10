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

package packages

import (
	"io"
	"os"
	"testing"
)

func TestImporterNormal(t *testing.T) {
	p := NewImporter(nil)
	pkg, err := p.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import failed:", pkg, err)
	}
	if _, err = p.Import("not-found"); err == nil {
		t.Fatal("Import not-found: no error?")
	}
	if pkg2, err := p.Import("fmt"); err != nil || pkg2 != pkg {
		t.Fatal("Import reuse fail:", pkg, pkg2)
	}
}

func TestImporterRecursive(t *testing.T) {
	p := NewImporter(nil, "..")
	pkg, err := p.Import("github.com/goplus/gogen/internal/foo")
	if err != nil {
		t.Fatal("Import failed:", pkg, err)
	}
}

func TestImportBuiltin(t *testing.T) {
	p := NewImporter(nil, "..")
	pkg, err := p.Import("github.com/goplus/gogen/internal/builtin")
	if err != nil {
		t.Fatal("Import failed:", pkg, err)
	}
}

func TestImportUnsafe(t *testing.T) {
	p := NewImporter(nil)
	pkg, err := p.Import("unsafe")
	if err != nil {
		t.Fatal("Import failed", pkg, err)
	}
}

// ----------------------------------------------------------------------------

type diskCache struct {
	imp *Importer
}

func (p diskCache) Find(dir, pkgPath string) (f io.ReadCloser, err error) {
	if p.imp != nil {
		return p.imp.findExport(dir, pkgPath)
	}
	return nil, os.ErrNotExist
}

func TestCache(t *testing.T) {
	nlist = 0
	p := NewImporter(nil)
	p.SetCache(diskCache{})
	_, err := p.Import("fmt")
	if err != os.ErrNotExist {
		t.Fatal("Import:", err)
	}
	p.SetCache(diskCache{imp: NewImporter(nil)})
	if p.Cache() == nil {
		t.Fatal("Cache nil")
	}
	pkg, err := p.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import fmt:", pkg, err)
	}
	if v := ListTimes(); v != 1 {
		t.Fatal("ListTimes:", v)
	}
}

// ----------------------------------------------------------------------------
