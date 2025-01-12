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
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/gcexportdata"
)

// ----------------------------------------------------------------------------

// Cache represents a cache for the importer.
type Cache interface {
	Find(dir, pkgPath string) (f io.ReadCloser, err error)
}

// Importer represents a Go package importer.
type Importer struct {
	loaded map[string]*types.Package
	fset   *token.FileSet
	dir    string
	m      sync.RWMutex
	cache  Cache
	tags   string
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

// SetCache sets an optional cache for the importer.
func (p *Importer) SetCache(cache Cache) {
	p.cache = cache
}

// Cache returns the cache of the importer.
func (p *Importer) Cache() Cache {
	return p.cache
}

func (p *Importer) SetTags(tags string) {
	p.tags = tags
}

func (p *Importer) Tags() string {
	return p.tags
}

// Import returns the imported package for the given import path.
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
	f, err := p.findExport(dir, pkgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return p.loadByExport(f, pkgPath)
}

func (p *Importer) loadByExport(f io.ReadCloser, pkgPath string) (ret *types.Package, err error) {
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
func (p *Importer) findExport(dir, pkgPath string) (f io.ReadCloser, err error) {
	if c := p.cache; c != nil {
		return c.Find(dir, pkgPath)
	}
	atomic.AddInt32(&nlist, 1)
	data, err := golistExport(dir, pkgPath, p.tags)
	if err != nil {
		return
	}
	expfile := string(bytes.TrimSuffix(data, []byte{'\n'}))
	return os.Open(expfile)
}

func golistExport(dir, pkgPath string, tags string) (ret []byte, err error) {
	var stdout, stderr bytes.Buffer
	args := []string{"list", "-f={{.Export}}"}
	if tags != "" {
		args = append(args, "-tags=", tags)
	}
	args = append(args, "-export", pkgPath)
	cmd := exec.Command("go", args...)
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
