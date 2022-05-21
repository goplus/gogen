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

package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------------

// Ref type
type Ref = types.Object

// A PkgRef describes a Go package imported by others.
type PkgRef struct {
	// pkgPath is the package path.
	pkgPath string

	// Types provides type information for the package.
	// The NeedTypes LoadMode bit sets this field for packages matching the
	// patterns; type information for dependencies may be missing or incomplete,
	// unless NeedDeps and NeedImports are also set.
	Types *types.Package

	nameRefs []*ast.Ident // for internal use

	imp types.Importer // to import packages anywhere

	isForceUsed bool // this package is force-used
	isUsed      bool

	// IllTyped indicates whether the package or any dependency contains errors.
	// It is set only when Types is set.
	IllTyped bool
}

func (p *PkgRef) markUsed(v *ast.Ident) {
	if p.isUsed {
		return
	}
	for _, ref := range p.nameRefs {
		if ref == v {
			p.isUsed = true
			return
		}
	}
}

// Path returns the package path.
func (p *PkgRef) Path() string {
	return p.pkgPath
}

// Ref returns the object in this package with the given name if such an
// object exists; otherwise it panics.
func (p *PkgRef) Ref(name string) Ref {
	if o := p.TryRef(name); o != nil {
		return o
	}
	panic(p.Path() + "." + name + " not found")
}

// TryRef returns the object in this package with the given name if such an
// object exists; otherwise it returns nil.
func (p *PkgRef) TryRef(name string) Ref {
	p.EnsureImported()
	return p.Types.Scope().Lookup(name)
}

// MarkForceUsed marks this package is force-used.
func (p *PkgRef) MarkForceUsed() {
	p.isForceUsed = true
}

// EnsureImported ensures this package is imported.
func (p *PkgRef) EnsureImported() {
	if p.Types == nil {
		var err error
		if p.Types, err = p.imp.Import(p.pkgPath); err != nil {
			log.Panicln("Import package not found:", p.pkgPath)
		} else {
			initGopPkg(p.Types)
		}
	}
}

func initGopPkg(pkg *types.Package) {
	if pkg.Scope().Lookup("GopPackage") == nil { // not is a Go+ package
		return
	}
	type omthd struct {
		named *types.Named
		mthd  string
	}
	scope := pkg.Scope()
	overloads := make(map[string][]types.Object)
	moverloads := make(map[omthd][]types.Object)
	names := scope.Names()
	for _, name := range names {
		o := scope.Lookup(name)
		if tn, ok := o.(*types.TypeName); ok && tn.IsAlias() {
			continue
		}
		if n := len(name); n > 3 && name[n-3:n-1] == "__" { // overload function
			key := name[:n-3]
			overloads[key] = append(overloads[key], o)
		} else if named, ok := o.Type().(*types.Named); ok {
			for i, n := 0, named.NumMethods(); i < n; i++ {
				m := named.Method(i)
				mName := m.Name()
				if n := len(mName); n > 3 && mName[n-3:n-1] == "__" { // overload method
					mthd := mName[:n-3]
					key := omthd{named, mthd}
					moverloads[key] = append(moverloads[key], m)
				}
			}
		} else {
			checkTemplateMethod(pkg, name, o)
		}
	}
	for key, items := range overloads {
		off := len(key) + 2
		fns := overloadFuncs(off, items)
		if debugImport {
			log.Println("==> NewOverloadFunc", key)
		}
		o := NewOverloadFunc(token.NoPos, pkg, key, fns...)
		scope.Insert(o)
		checkTemplateMethod(pkg, key, o)
	}
	for key, items := range moverloads {
		off := len(key.mthd) + 2
		fns := overloadFuncs(off, items)
		if debugImport {
			log.Println("==> NewOverloadMethod", key.named.Obj().Name(), key.mthd)
		}
		NewOverloadMethod(key.named, token.NoPos, pkg, key.mthd, fns...)
	}
}

func checkTemplateMethod(pkg *types.Package, name string, o types.Object) {
	const (
		goptPrefix = "Gopt_"
	)
	if strings.HasPrefix(name, goptPrefix) {
		name = name[len(goptPrefix):]
		if pos := strings.Index(name, "_"); pos > 0 {
			tname, mname := name[:pos], name[pos+1:]
			if tobj := pkg.Scope().Lookup(tname); tobj != nil {
				if tn, ok := tobj.(*types.TypeName); ok {
					if t, ok := tn.Type().(*types.Named); ok {
						if debugImport {
							log.Println("==> NewTemplateRecvMethod", tname, mname)
						}
						NewTemplateRecvMethod(t, token.NoPos, pkg, mname, o)
					}
				}
			}
		}
	}
}

func overloadFuncs(off int, items []types.Object) []types.Object {
	fns := make([]types.Object, len(items))
	for _, item := range items {
		idx := toIndex(item.Name()[off])
		if idx >= len(items) {
			log.Panicln("overload function must be from 0 to N:", item.Name(), len(fns))
		}
		if fns[idx] != nil {
			panic("overload function exists?")
		}
		fns[idx] = item
	}
	return fns
}

func toIndex(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'z' {
		return int(c - ('a' - 10))
	}
	panic("invalid character out of [0-9,a-z]")
}

// ----------------------------------------------------------------------------

// Import func
func (p *Package) Import(pkgPath string) *PkgRef {
	return p.file.importPkg(p, pkgPath)
}

func (p *Package) big() *PkgRef {
	return p.file.big(p)
}

func (p *Package) unsafe() *PkgRef {
	return p.file.unsafe(p)
}

// ----------------------------------------------------------------------------

type null struct{}
type autoNames struct {
	gbl     *types.Scope
	builtin *types.Scope
	names   map[string]null
	idx     int
}

const (
	goxAutoPrefix = "_autoGo_"
)

func (p *Package) autoName() string {
	p.autoIdx++
	return goxAutoPrefix + strconv.Itoa(p.autoIdx)
}

func (p *Package) newAutoNames() *autoNames {
	return &autoNames{
		gbl:     p.Types.Scope(),
		builtin: p.builtin.Scope(),
		names:   make(map[string]null),
	}
}

func scopeHasName(at *types.Scope, name string) bool {
	if at.Lookup(name) != nil {
		return true
	}
	for i := at.NumChildren(); i > 0; {
		i--
		if scopeHasName(at.Child(i), name) {
			return true
		}
	}
	return false
}

func (p *autoNames) importHasName(name string) bool {
	_, ok := p.names[name]
	return ok
}

func (p *autoNames) hasName(name string) bool {
	return scopeHasName(p.gbl, name) || p.importHasName(name) ||
		p.builtin.Lookup(name) != nil || types.Universe.Lookup(name) != nil
}

func (p *autoNames) RequireName(name string) (ret string, renamed bool) {
	ret = name
	for p.hasName(ret) {
		p.idx++
		ret = name + strconv.Itoa(p.idx)
		renamed = true
	}
	p.names[name] = null{}
	return
}

// ----------------------------------------------------------------------------
