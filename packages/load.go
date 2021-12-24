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
	"go/token"
	"go/types"
	"os"

	"golang.org/x/tools/go/gcexportdata"
)

// ----------------------------------------------------------------------------

type Config struct {
	Dir    string
	Loaded map[string]*types.Package
	Fset   *token.FileSet
}

func Load(conf *Config, pattern ...string) (ret []*types.Package, err error) {
	if conf == nil {
		conf = new(Config)
	}
	pkgs, err := loadPkgs(conf.Dir, pattern...)
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
	ret = make([]*types.Package, len(pkgs))
	for i, pkg := range pkgs {
		ret[i], err = loadPkgExport(pkg.Export, fset, loaded, pkg.ImportPath)
		if err != nil {
			return
		}
	}
	return
}

func loadPkgExport(expfile string, fset *token.FileSet, loaded map[string]*types.Package, path string) (pkg *types.Package, err error) {
	if ret, ok := loaded[path]; ok && ret.Complete() {
		return ret, nil
	}
	f, err := os.Open(expfile)
	if err != nil {
		return
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return
	}
	return gcexportdata.Read(r, fset, loaded, path)
}

// ----------------------------------------------------------------------------
