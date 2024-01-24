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

package cpackages

import (
	"go/types"

	"github.com/goplus/gox"
)

// ----------------------------------------------------------------------------

type PkgRef struct {
	pkg    gox.PkgRef
	public map[string]string
}

func (p *PkgRef) Pkg() gox.PkgRef {
	return p.pkg
}

func (p *PkgRef) Lookup(name string) types.Object {
	if goName, ok := p.public[name]; ok {
		if goName == "" {
			goName = gox.CPubName(name)
		}
		return p.pkg.TryRef(goName)
	}
	return nil
}

func PubName(name string) string {
	return gox.CPubName(name)
}

// ----------------------------------------------------------------------------

type Config struct {
	Pkg       *gox.Package
	LookupPub func(pkgPath string) (pubfile string, err error)
}

type Importer struct {
	loaded    map[string]PkgRef
	lookupPub func(pkgPath string) (pubfile string, err error)
	pkg       *gox.Package
}

func NewImporter(conf *Config) *Importer {
	return &Importer{
		loaded:    make(map[string]PkgRef),
		lookupPub: conf.LookupPub,
		pkg:       conf.Pkg,
	}
}

func (p *Importer) Import(pkgPath string) (pkg PkgRef, err error) {
	if ret, ok := p.loaded[pkgPath]; ok {
		return ret, nil
	}
	pubfile, err := p.lookupPub(pkgPath)
	if err != nil {
		return
	}
	public, err := ReadPubFile(pubfile)
	if err != nil {
		return
	}
	pkgImp := p.pkg.Import(pkgPath)
	pkg = PkgRef{pkg: pkgImp, public: public}
	p.loaded[pkgPath] = pkg
	return
}

// ----------------------------------------------------------------------------
