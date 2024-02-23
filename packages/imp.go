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
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
)

// pkgPath Caches
var (
	dirCache          = map[string]bool{}
	dirCacheMutex     = sync.RWMutex{}
	packageCacheMap   = map[string]string{}
	packageCacheMutex = sync.RWMutex{}
	waitCache         = sync.WaitGroup{}
)

// ----------------------------------------------------------------------------

type Importer struct {
	loaded map[string]*types.Package
	fset   *token.FileSet
	dir    string
	m      sync.RWMutex
}

// NewImporter creates an Importer object that meets types.Importer interface.
func NewImporter(fset *token.FileSet, workDir ...string) *Importer {
	dir := ""
	if len(workDir) > 0 {
		dir = workDir[0]
	}
	if len(packageCacheMap) == 0 {
		initGoListCache(dir)
	}
	if fset == nil {
		fset = token.NewFileSet()
	}
	loaded := make(map[string]*types.Package)
	loaded["unsafe"] = types.Unsafe
	return &Importer{loaded: loaded, fset: fset, dir: dir}
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
	expfile, err := FindExport(dir, pkgPath)
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

// FindExport lookups export file (.a) of a package by its pkgPath.
// It returns empty if pkgPath not found.
func FindExport(dir, pkgPath string) (expfile string, err error) {
	waitCache.Wait()
	expfile = packageCacheMap[pkgPath]
	if len(expfile) > 0 {
		return expfile, nil
	}
	data, err := golistExport(dir, pkgPath)
	if err != nil {
		return
	}
	expfile = string(bytes.TrimSuffix(data, []byte{'\n'}))
	return
}

// https://github.com/goplus/gop/issues/1710
// Not fully optimized
// Retrieve all imports in the specified directory and cache them
func goListExportCache(dir string, pkgs ...string) {
	dirCacheMutex.Lock()
	if dirCache[dir] {
		dirCacheMutex.Unlock()
		return
	}
	dirCache[dir] = true
	dirCacheMutex.Unlock()
	var stdout, stderr bytes.Buffer
	commandStr := []string{"list", "-f", "{{.ImportPath}},{{.Export}}", "-export", "-e"}
	commandStr = append(commandStr, pkgs...)
	commandStr = append(commandStr, "all")
	cmd := exec.Command("go", commandStr...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		ret := stdout.String()
		for _, v := range strings.Split(ret, "\n") {
			s := strings.Split(v, ",")
			if len(s) != 2 {
				continue
			}
			packageCacheMutex.Lock()
			packageCacheMap[s[0]] = s[1]
			packageCacheMutex.Unlock()
		}
	}
}
func GoListExportCacheSync(dir string, pkgs ...string) {
	waitCache.Add(1)
	go func() {
		defer waitCache.Done()
		goListExportCache(dir, pkgs...)
	}()
}
func initGoListCache(dir string) {
	goListExportCache(dir)
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

// ----------------------------------------------------------------------------
