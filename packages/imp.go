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
	"bytes"
	"errors"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/gcexportdata"
)

// ----------------------------------------------------------------------------

type DiskCache interface {
	Find(dir, pkgPath string) (expfile string, err error)
}

type Importer struct {
	loaded map[string]*types.Package
	fset   *token.FileSet
	dir    string
	m      sync.RWMutex
	cache  DiskCache
}

// NewImporter creates an Importer object that meets types.ImporterFrom and types.Importer interface.
func NewImporter(fset *token.FileSet, workDir ...string) *Importer {
	dir := ""
	if len(workDir) > 0 {
		dir = workDir[0]
	}
	if fset == nil {
		fset = token.NewFileSet()
	}
	loaded := make(map[string]*types.Package)
	loaded["unsafe"] = types.Unsafe
	return &Importer{loaded: loaded, fset: fset, dir: dir}
}

// SetDiskCache sets an optional disk cache for the importer.
func (p *Importer) SetDiskCache(cache DiskCache) {
	p.cache = cache
}

func (p *Importer) Import(pkgPath string) (pkg *types.Package, err error) {
	return p.ImportFrom(pkgPath, p.dir, 0)
}

// ImportFrom returns the imported package for the given import
// path when imported by a package file located in dir.
// If the import failed, besides returning an error, ImportFrom
// is encouraged to cache and return a package anyway, if one
// was created. This will reduce package inconsistencies and
// follow-on type checker errors due to the missing package.
// The mode value must be 0; it is reserved for future use.
// Two calls to ImportFrom with the same path and dir must
// return the same package.
func (p *Importer) ImportFrom(pkgPath, dir string, mode types.ImportMode) (*types.Package, error) {
	p.m.RLock()
	if ret, ok := p.loaded[pkgPath]; ok && ret.Complete() {
		p.m.RUnlock()
		return ret, nil
	}
	p.m.RUnlock()
	expfile, err := p.findExport(dir, pkgPath)
	if err != nil {
		return nil, err
	}
	return p.loadByExport(expfile, pkgPath)
}

func (p *Importer) loadByExport(expfile string, pkgPath string) (ret *types.Package, err error) {
	f, err := os.Open(expfile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err == nil {
		p.m.Lock() // use mutex because Import should be multi-thread safe
		defer p.m.Unlock()
		ret, err = gcexportdata.Read(r, p.fset, p.loaded, pkgPath)
	}
	return
}

// ----------------------------------------------------------------------------

// findExport lookups export file (.a) of a package by its pkgPath.
// It returns empty if pkgPath not found.
func (p *Importer) findExport(dir, pkgPath string) (expfile string, err error) {
	if c := p.cache; c != nil {
		return c.Find(dir, pkgPath)
	}
	atomic.AddInt32(&nlist, 1)
	data, err := golistExport(dir, pkgPath)
	if err != nil {
		return
	}
	expfile = string(bytes.TrimSuffix(data, []byte{'\n'}))
	return
}

func golistExport(dir, pkgPath string) (ret []byte, err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "list", "-f={{.Export}}", "-export", pkgPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err = cmd.Run()
	if err == nil {
		ret = stdout.Bytes()
	} else if stderr.Len() > 0 {
		err = errors.New(stderr.String())
	}
	return
}

var (
	nlist int32
)

// ListTimes returns the number of times of calling `go list`.
func ListTimes() int {
	return int(atomic.LoadInt32(&nlist))
}

// ----------------------------------------------------------------------------
