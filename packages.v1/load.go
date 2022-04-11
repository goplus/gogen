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
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/tools/go/gcexportdata"
)

const (
	DbgNoRemoveTempFile = 1 << iota
)

var (
	debugRemoveTempFile = true
)

func SetDebug(dbgFlags int) {
	debugRemoveTempFile = (dbgFlags & DbgNoRemoveTempFile) == 0
}

func isLocal(ns string) bool {
	if len(ns) > 0 {
		switch c := ns[0]; c {
		case '/', '\\', '.':
			return true
		default:
			return len(ns) >= 2 && ns[1] == ':' && ('A' <= c && c <= 'Z' || 'a' <= c && c <= 'z')
		}
	}
	return false
}

// ----------------------------------------------------------------------------

type Config struct {
	// ModRoot specifies module root directory (required).
	ModRoot string

	// ModPath specifies module path (required).
	ModPath string

	// Loaded specifies all loaded packages (optional).
	Loaded map[string]*types.Package

	// Fset provides source position information for syntax trees and types (optional).
	// If Fset is nil, Load will use a new fileset, but preserve Fset's value.
	Fset *token.FileSet
}

func (p *Config) getTempDir() string {
	return filepath.Join(p.ModRoot, ".gop/_dummy")
}

type none = struct{}

func (p *Config) listPkgs(ret map[string]none, pat, modRoot string) (err error) {
	const multi = "/..."
	if isLocal(pat) {
		recursive := strings.HasSuffix(pat, multi)
		if recursive {
			pat = pat[:len(pat)-len(multi)]
			if pat == "" {
				pat = "/"
			}
		}
		patAbs, err1 := filepath.Abs(pat)
		patRel, err2 := filepath.Rel(modRoot, patAbs)
		if err1 != nil || err2 != nil || strings.HasPrefix(patRel, "..") {
			return fmt.Errorf("directory `%s` outside available modules", pat)
		}
		pkgPathBase := path.Join(p.ModPath, filepath.ToSlash(patRel))
		return doListPkgs(ret, pkgPathBase, pat, recursive)
	}
	ret[pat] = none{}
	return nil
}

func doListPkgs(ret map[string]none, pkgPathBase, pat string, recursive bool) (err error) {
	fis, err := os.ReadDir(pat)
	if err != nil {
		return
	}
	noSouceFile := true
	for _, fi := range fis {
		name := fi.Name()
		if strings.HasPrefix(name, "_") {
			continue
		}
		if fi.IsDir() {
			if recursive {
				if err = doListPkgs(ret, pkgPathBase+"/"+name, pat+"/"+name, true); err != nil {
					return
				}
			}
		} else if noSouceFile {
			ext := path.Ext(name)
			if ext == ".go" {
				noSouceFile = false
			}
		}
	}
	if !noSouceFile {
		ret[pkgPathBase] = none{}
	}
	return nil
}

func List(conf *Config, pattern ...string) (pkgPaths []string, err error) {
	if conf == nil {
		conf = new(Config)
	}
	ret := make(map[string]none)
	modRoot, _ := filepath.Abs(conf.ModRoot)
	for _, pat := range pattern {
		if err = conf.listPkgs(ret, pat, modRoot); err != nil {
			return
		}
	}
	return getKeys(ret), nil
}

func getKeys(v map[string]none) []string {
	keys := make([]string, 0, len(v))
	for key := range v {
		keys = append(keys, key)
	}
	return keys
}

func Load(conf *Config, pattern ...string) (pkgs []*types.Package, err error) {
	p, pkgPaths, err := NewImporter(conf, pattern...)
	if err != nil {
		return
	}
	pkgs = make([]*types.Package, len(pkgPaths))
	for i, pkgPath := range pkgPaths {
		if pkgs[i], err = p.Import(pkgPath); err != nil {
			return
		}
	}
	return
}

// ----------------------------------------------------------------------------

type Importer struct {
	pkgs   map[string]pkgExport
	loaded map[string]*types.Package
	fset   *token.FileSet
	wds    []string
}

func NewImporter(conf *Config, pattern ...string) (p *Importer, pkgPaths []string, err error) {
	initLoadDeps()
	if conf == nil {
		conf = new(Config)
	}
	pkgPaths, err = List(conf, pattern...)
	if err != nil {
		return
	}
	pkgs, wds, err := loadDeps(conf.getTempDir(), pkgPaths...)
	if err != nil {
		return
	}
	fset := conf.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	loaded := conf.Loaded
	if loaded == nil {
		loaded = make(map[string]*types.Package)
	}
	loaded["unsafe"] = types.Unsafe
	p = &Importer{pkgs: pkgs, loaded: loaded, fset: fset, wds: wds}
	runtime.SetFinalizer(p, (*Importer).Close)
	return
}

func (p *Importer) Close() error {
	if p.wds != nil {
		cleanWorkDirs(p.wds)
		p.wds = nil
	}
	// no need for a finalizer anymore
	runtime.SetFinalizer(p, nil)
	return nil
}

func (p *Importer) Import(pkgPath string) (*types.Package, error) {
	if ret, ok := p.loaded[pkgPath]; ok && ret.Complete() {
		return ret, nil
	}
	if expfile, ok := p.pkgs[pkgPath]; ok {
		return p.loadPkgExport(expfile, pkgPath)
	}
	return nil, syscall.ENOENT
}

func (p *Importer) loadPkgExport(expfile string, pkgPath string) (*types.Package, error) {
	f, err := os.Open(expfile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return nil, err
	}
	return gcexportdata.Read(r, p.fset, p.loaded, pkgPath)
}

// ----------------------------------------------------------------------------
