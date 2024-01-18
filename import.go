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
	"fmt"
	"go/ast"
	"go/constant"
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
	// Types provides type information for the package.
	// The NeedTypes LoadMode bit sets this field for packages matching the
	// patterns; type information for dependencies may be missing or incomplete,
	// unless NeedDeps and NeedImports are also set.
	Types *types.Package

	nameRefs []*ast.Ident // for internal use

	isForceUsed bool // this package is force-used
	isUsed      bool
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
	return p.Types.Path()
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
	return p.Types.Scope().Lookup(name)
}

// MarkForceUsed marks this package is force-used.
func (p *PkgRef) MarkForceUsed() {
	p.isForceUsed = true
}

// EnsureImported ensures this package is imported.
func (p *PkgRef) EnsureImported() {
}

func shouldAddGopPkg(pkg *Package) bool {
	return pkg.isGopPkg && pkg.Types.Scope().Lookup(gopPackage) == nil
}

func isGopFunc(name string) bool {
	return isOverloadFunc(name) || isGoptFunc(name)
}

func isGoptFunc(name string) bool {
	return strings.HasPrefix(name, goptPrefix)
}

func isGopoFunc(name string) bool {
	return strings.HasPrefix(name, gopoPrefix)
}

func isOverloadFunc(name string) bool {
	n := len(name)
	return n > 3 && name[n-3:n-1] == "__"
}

func initThisGopPkg(pkg *types.Package) {
	scope := pkg.Scope()
	if scope.Lookup(gopPackage) == nil { // not is a Go+ package
		return
	}
	if debugImport {
		log.Println("==> Import", pkg.Path())
	}
	gopos := make([]string, 0, 4)
	overloads := make(map[omthd][]types.Object)
	names := scope.Names()
	for _, name := range names {
		o := scope.Lookup(name)
		if tn, ok := o.(*types.TypeName); ok && tn.IsAlias() {
			continue
		}
		if isGopoFunc(name) {
			gopos = append(gopos, name)
		} else if isOverloadFunc(name) { // overload function
			key := omthd{nil, name[:len(name)-3]}
			overloads[key] = append(overloads[key], o)
		} else if named, ok := o.Type().(*types.Named); ok {
			var list methodList
			switch t := named.Underlying().(type) {
			case *types.Interface:
				list = t // add interface overload method to named
			default:
				list = named
			}
			for i, n := 0, list.NumMethods(); i < n; i++ {
				m := list.Method(i)
				mName := m.Name()
				if isOverloadFunc(mName) { // overload method
					mthd := mName[:len(mName)-3]
					key := omthd{named, mthd}
					overloads[key] = append(overloads[key], m)
				}
			}
		} else {
			checkTemplateMethod(pkg, name, o)
		}
	}
	for _, gopoName := range gopos {
		if names, ok := checkOverloads(scope, gopoName); ok {
			key := gopoName[len(gopoPrefix):]
			m, tname := checkTypeMethod(scope, key)
			fns := make([]types.Object, len(names))
			for i, name := range names {
				if name == "" {
					if m.typ != nil {
						name = "(" + tname + ")."
					}
					name += m.name + "__" + indexTable[i:i+1]
				}
				fns[i] = lookupFunc(scope, name)
			}
			newOverload(pkg, scope, m, fns)
			delete(overloads, m)
		}
	}
	for key, items := range overloads {
		off := len(key.name) + 2
		fns := overloadFuncs(off, items)
		newOverload(pkg, scope, key, fns)
	}
}

// name
// (T).name
func lookupFunc(scope *types.Scope, name string) types.Object {
	if strings.HasPrefix(name, "(") {
		next := name[1:]
		pos := strings.Index(next, ").")
		if pos <= 0 {
			log.Panicf("lookupFunc: %v not a valid method, use `(T).method` please\n", name)
		}
		tname, mname := next[:pos], next[pos+2:]
		log.Println("lookupFunc:", tname, mname)
		tobj := scope.Lookup(tname)
		if tobj != nil {
			if tn, ok := tobj.(*types.TypeName); ok {
				if o, ok := tn.Type().(*types.Named); ok { // TODO: interface support
					for i, n := 0, o.NumMethods(); i < n; i++ {
						method := o.Method(i)
						if method.Name() == mname {
							return method
						}
					}
				}
			}
		}
	} else if o := scope.Lookup(name); o != nil {
		return o
	}
	log.Panicf("lookupFunc: %v not found\n", name)
	return nil
}

type omthd struct {
	typ  *types.Named
	name string
}

// TypeName_Method
// _TypeName__Method
func checkTypeMethod(scope *types.Scope, name string) (omthd, string) {
	if pos := strings.IndexByte(name, '_'); pos >= 0 {
		nsep := 1
		if pos == 0 {
			t := name[1:]
			if pos = strings.Index(t, "__"); pos <= 0 {
				return omthd{nil, t}, ""
			}
			name, nsep = t, 2
		}
		tname, mname := name[:pos], name[pos+nsep:]
		tobj := scope.Lookup(tname)
		if tobj != nil {
			if tn, ok := tobj.(*types.TypeName); ok {
				if t, ok := tn.Type().(*types.Named); ok {
					return omthd{t, mname}, tname
				}
			}
		}
		if tobj != nil || nsep == 2 {
			log.Panicf("checkTypeMethod: %v not found or not a named type\n", tname)
		}
	}
	return omthd{nil, name}, ""
}

// Gopt_TypeName_Method
// Gopt__TypeName__Method
func checkTemplateMethod(pkg *types.Package, name string, o types.Object) {
	if strings.HasPrefix(name, goptPrefix) {
		name = name[len(goptPrefix):]
		if m, tname := checkTypeMethod(pkg.Scope(), name); m.typ != nil {
			if debugImport {
				log.Println("==> NewTemplateRecvMethod", tname, m.name)
			}
			NewTemplateRecvMethod(m.typ, token.NoPos, pkg, m.name, o)
		}
	}
}

const (
	goptPrefix = "Gopt_" // template method
	gopoPrefix = "Gopo_" // overload function/method
	gopPackage = "GopPackage"
)

/*
const (
	Gopo_FuncName = "Func0,Func1,,,Func4"
	Gopo_TypeName_Method = "Func0,,,,Func4"
	Gopo__TypeName__Method = "Func0,,,,Func4"
)
*/

func checkOverloads(scope *types.Scope, gopoName string) (ret []string, exists bool) {
	if o := scope.Lookup(gopoName); o != nil {
		if c, ok := o.(*types.Const); ok {
			if v := c.Val(); v.Kind() == constant.String {
				return strings.Split(constant.StringVal(v), ","), true
			}
		}
		panic("checkOverloads TODO: should be string constant - " + gopoName)
	}
	return
}

func newOverload(pkg *types.Package, scope *types.Scope, m omthd, fns []types.Object) {
	if m.typ == nil {
		if debugImport {
			log.Println("==> NewOverloadFunc", m.name)
		}
		o := NewOverloadFunc(token.NoPos, pkg, m.name, fns...)
		scope.Insert(o)
		checkTemplateMethod(pkg, m.name, o)
	} else {
		if debugImport {
			log.Println("==> NewOverloadMethod", m.typ.Obj().Name(), m.name)
		}
		NewOverloadMethod(m.typ, token.NoPos, pkg, m.name, fns...)
	}
}

func overloadFuncs(off int, items []types.Object) []types.Object {
	fns := make([]types.Object, len(items))
	for _, item := range items {
		idx := toIndex(item.Name()[off])
		if idx >= len(items) {
			log.Panicf("overload func %v out of range 0..%v\n", item.Name(), len(fns)-1)
		}
		if fns[idx] != nil {
			log.Panicf("overload func %v exists?\n", item.Name())
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

const (
	indexTable = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// ----------------------------------------------------------------------------

// Context represents all things between packages.
type Context struct {
	chkGopImports map[string]bool
}

func NewContext() *Context {
	return &Context{
		chkGopImports: make(map[string]bool),
	}
}

// InitGopPkg initializes a Go+ packages.
func (p *Context) InitGopPkg(importer types.Importer, pkgImp *types.Package) {
	pkgPath := pkgImp.Path()
	if stdPkg(pkgPath) || p.chkGopImports[pkgPath] {
		return
	}
	if !pkgImp.Complete() {
		importer.Import(pkgPath)
	}
	initThisGopPkg(pkgImp)
	p.chkGopImports[pkgPath] = true
	for _, imp := range pkgImp.Imports() {
		p.InitGopPkg(importer, imp)
	}
}

func stdPkg(pkgPath string) bool {
	return strings.IndexByte(pkgPath, '.') < 0
}

// ----------------------------------------------------------------------------

// Import imports a package by pkgPath. It will panic if pkgPath not found.
func (p *Package) Import(pkgPath string, src ...ast.Node) *PkgRef {
	return p.file.importPkg(p, pkgPath, getSrc(src))
}

// TryImport imports a package by pkgPath. It returns nil if pkgPath not found.
func (p *Package) TryImport(pkgPath string) *PkgRef {
	defer func() {
		recover()
	}()
	return p.file.importPkg(p, pkgPath, nil)
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

type ImportError struct {
	Fset dbgPositioner
	Pos  token.Pos
	Path string
	Err  error
}

func (p *ImportError) Unwrap() error {
	return p.Err
}

func (p *ImportError) Error() string {
	if p.Pos == token.NoPos {
		return fmt.Sprintf("%v", p.Err)
	}
	pos := p.Fset.Position(p.Pos)
	return fmt.Sprintf("%v: %v", pos, p.Err)
}

// ----------------------------------------------------------------------------
