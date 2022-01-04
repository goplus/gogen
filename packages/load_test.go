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

package packages

import (
	"errors"
	"go/types"
	"strings"
	"syscall"
	"testing"
)

// ----------------------------------------------------------------------------

func init() {
	SetDebug(0)
}

func TestList(t *testing.T) {
	pkgPaths, err := List(nil, ".")
	if err != nil || len(pkgPaths) != 1 {
		t.Fatal("List failed:", pkgPaths, err)
	}
}

func TestGetProgramList(t *testing.T) {
	if getProgramList(": import \"", nil) != 0 {
		t.Fatal("getProgramList failed")
	}
}

func TestIsLocal(t *testing.T) {
	if !isLocal(".") || !isLocal("/") {
		t.Fatal(`isLocal(".") || isLocal("/")`)
	}
	if !isLocal("c:/foo") {
		t.Fatal(`isLocal("c:/foo")`)
	}
	if !isLocal("C:/foo") {
		t.Fatal(`isLocal("C:/foo")`)
	}
	if isLocal("") {
		t.Fatal(`isLocal("")`)
	}
}

// ----------------------------------------------------------------------------

func TestLoadDep(t *testing.T) {
	pkgs, err := loadDeps("./.gop", "fmt")
	if err != nil {
		t.Fatal("LoadDeps failed:", pkgs, err)
	}
	if _, ok := pkgs["runtime"]; !ok {
		t.Fatal("LoadDeps failed:", pkgs)
	}

	_, err = loadDepPkgsFrom(nil, " ")
	if err != ErrWorkDirNotFound {
		t.Fatal("LoadDeps:", err)
	}
	_, err = loadDepPkgsFrom(nil, "WORK=")
	if err != ErrWorkDirNotFound {
		t.Fatal("LoadDeps:", err)
	}
	_, err = loadDepPkgsFrom(nil, "WORK=a\n ")
	if err != nil {
		t.Fatal("LoadDeps:", err)
	}
}

func TestLoadDepErr(t *testing.T) {
	_, err := loadDeps("/.gop", "fmt")
	if err == nil {
		t.Fatal("LoadDeps: no error")
	}
}

// ----------------------------------------------------------------------------

func TestLoadPkgs(t *testing.T) {
	pkgs, err := LoadPkgs("", "fmt", "strings")
	if err != nil {
		t.Fatal("Load failed:", err)
	}
	if len(pkgs) != 2 {
		t.Log(pkgs)
	}
}

func TestLoadPkgsErr(t *testing.T) {
	{
		err := &ExecCmdError{Stderr: []byte("Hi")}
		if err.Error() != "Hi" {
			t.Fatal("ExecCmdError failed:", err)
		}

		err = &ExecCmdError{Err: errors.New("Hi")}
		if err.Error() != "Hi" {
			t.Fatal("ExecCmdError failed:", err)
		}
	}
	pkgs, err := LoadPkgs("", "?")
	if err == nil || err.Error() != `malformed import path "?": invalid char '?'
` {
		t.Fatal("loadPkgs:", pkgs, err)
	}
}

func TestLoadPkgsFromErr(t *testing.T) {
	_, err := loadPkgsFrom(nil, []byte("{"))
	if err == nil {
		t.Fatal("loadPkgs no error?")
	}
	_, err = loadPkgsFrom(nil, []byte("{\n"))
	if err == nil {
		t.Fatal("loadPkgs no error?")
	}
	_, err = loadPkgsFrom(nil, []byte("{\n1\n}\n"))
	if err == nil {
		t.Fatal("loadPkgs no error?")
	}
}

// ----------------------------------------------------------------------------

func TestLoadErr(t *testing.T) {
	pkgs, err := Load(nil, "?")
	if err == nil || !strings.Contains(err.Error(), `: invalid import path: "?"
`) {
		t.Fatal("Load:", pkgs, err)
	}

	var imp Importer
	_, err = imp.loadPkgExport("/not-found", "fmt")
	if err == nil {
		t.Fatal("loadPkgExport no error?")
	}
	_, err = imp.loadPkgExport("load.go", "fmt")
	if err == nil {
		t.Fatal("loadPkgExport no error?")
	}

	err = doListPkgs(nil, "", "/not-found", false)
	if err == nil {
		t.Fatal("doListPkgs no error?")
	}
}

func TestLoadNoConf(t *testing.T) {
	pkgs, err := Load(nil, "fmt", "strings")
	if err != nil {
		t.Fatal("Load failed:", err)
	}
	if len(pkgs) != 2 {
		t.Log(pkgs)
	}
}

func TestLoadConf(t *testing.T) {
	conf := &Config{
		Loaded: make(map[string]*types.Package),
	}
	pkgs1, err := Load(conf, "fmt", "fmt", "strings")
	if err != nil {
		t.Fatal("Load failed:", err)
	}
	if len(pkgs1) != 2 {
		t.Log(pkgs1)
	}

	pkgs2, err := Load(conf, "fmt", "strconv")
	if err != nil {
		t.Fatal("Load failed:", err)
	}
	if len(pkgs2) != 2 {
		t.Log(pkgs2)
	}
}

func TestImporterNormal(t *testing.T) {
	conf := &Config{
		Loaded:  make(map[string]*types.Package),
		ModPath: "github.com/goplus/gox/packages",
	}
	p, _, err := NewImporter(conf, ".")
	if err != nil {
		t.Fatal("NewImporter failed:", err)
	}
	pkg, err := p.Import("fmt")
	if err != nil || pkg.Path() != "fmt" {
		t.Fatal("Import failed:", pkg, err)
	}
	if _, err = p.Import("not-found"); err != syscall.ENOENT {
		t.Fatal("Import not-found:", err)
	}
}

func TestImporterRecursive(t *testing.T) {
	conf := &Config{
		Loaded:  make(map[string]*types.Package),
		ModRoot: "..",
		ModPath: "github.com/goplus/gox",
	}
	p, pkgPaths, err := NewImporter(conf, "../internal/foo/...")
	if err != nil {
		t.Fatal("NewImporter failed:", err)
	}
	if len(pkgPaths) == 0 {
		t.Fatal("NewImporter pkgPaths:", pkgPaths)
	}
	pkg, err := p.Import("github.com/goplus/gox/internal/foo")
	if err != nil {
		t.Fatal("Import failed:", pkg, pkgPaths, err)
	}
}

func TestImporterRecursiveErr(t *testing.T) {
	conf := &Config{
		Loaded:  make(map[string]*types.Package),
		ModPath: "github.com/goplus/gox/packages",
	}
	p, pkgPaths, err := NewImporter(conf, "/...")
	if err == nil || err.Error() != "directory `/` outside available modules" {
		t.Fatal("NewImporter failed:", p, pkgPaths, err)
	}
}

// ----------------------------------------------------------------------------
