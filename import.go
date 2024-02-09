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
	"path"
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
}

func (p PkgRef) isValid() bool {
	return p.Types != nil
}

func (p PkgRef) isNil() bool {
	return p.Types == nil
}

// Path returns the package path.
func (p PkgRef) Path() string {
	return p.Types.Path()
}

// Ref returns the object in this package with the given name if such an
// object exists; otherwise it panics.
func (p PkgRef) Ref(name string) Ref {
	if o := p.TryRef(name); o != nil {
		return o
	}
	panic(p.Path() + "." + name + " not found")
}

// TryRef returns the object in this package with the given name if such an
// object exists; otherwise it returns nil.
func (p PkgRef) TryRef(name string) Ref {
	return p.Types.Scope().Lookup(name)
}

// MarkForceUsed marks to import a package always (i.e. `import _ pkgPath`).
func (p PkgRef) MarkForceUsed(pkg *Package) {
	pkg.file.forceImport(p.Types.Path())
}

// Deprecated: EnsureImported is nothing to do now.
func (p PkgRef) EnsureImported() {
}

func isGopoConst(name string) bool {
	return strings.HasPrefix(name, gopoPrefix)
}

func isGopFunc(name string) bool {
	return isOverload(name) || strings.HasPrefix(name, goptPrefix) || strings.HasPrefix(name, gopxPrefix)
}

func isOverload(name string) bool {
	n := len(name)
	return n > 3 && name[n-3:n-1] == "__"
}

// InitThisGopPkg initializes a Go+ package.
func InitThisGopPkg(pkg *types.Package) {
	scope := pkg.Scope()
	if scope.Lookup(gopPackage) == nil { // not is a Go+ package
		return
	}
	if debugImport {
		log.Println("==> Import", pkg.Path())
	}
	gopos := make([]string, 0, 4)
	overloads := make(map[omthd][]types.Object)
	onameds := make(map[string][]*types.Named)
	names := scope.Names()
	for _, name := range names {
		if isGopoConst(name) {
			gopos = append(gopos, name)
			continue
		}
		o := scope.Lookup(name)
		if tn, ok := o.(*types.TypeName); ok && tn.IsAlias() {
			continue
		}
		if named, ok := o.Type().(*types.Named); ok {
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
				if isOverload(mName) { // overload method
					mthd := mName[:len(mName)-3]
					key := omthd{named, mthd}
					overloads[key] = append(overloads[key], m)
				}
			}
			if isOverload(name) { // overload named
				key := name[:len(name)-3]
				onameds[key] = append(onameds[key], named)
			}
		} else if isOverload(name) { // overload function
			key := omthd{nil, name[:len(name)-3]}
			overloads[key] = append(overloads[key], o)
		} else {
			checkGoptGopx(pkg, scope, name, o)
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
				fns[i] = lookupFunc(scope, name, tname)
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
	for name, items := range onameds {
		off := len(name) + 2
		nameds := overloadNameds(off, items)
		if debugImport {
			log.Println("==> NewOverloadNamed", name)
		}
		on := NewOverloadNamed(token.NoPos, pkg, name, nameds...)
		scope.Insert(on)
	}
}

// name
// .name
// (T).name
func lookupFunc(scope *types.Scope, name, tname string) types.Object {
	first := name[0]
	if first == '.' || first == '(' {
		if first == '.' {
			name = name[1:]
		} else {
			next := name[1:]
			pos := strings.Index(next, ").")
			if pos <= 0 {
				log.Panicf("lookupFunc: %v not a valid method, use `(T).method` please\n", name)
			}
			tname, name = next[:pos], next[pos+2:]
		}
		tobj := scope.Lookup(tname)
		if tobj != nil {
			if tn, ok := tobj.(*types.TypeName); ok {
				if o, ok := tn.Type().(*types.Named); ok { // TODO: interface support
					for i, n := 0, o.NumMethods(); i < n; i++ {
						method := o.Method(i)
						if method.Name() == name {
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

// Gopx_Func
// Gopt_TypeName_Method
// Gopt__TypeName__Method
func checkGoptGopx(pkg *types.Package, scope *types.Scope, name string, o types.Object) {
	if strings.HasPrefix(name, goptPrefix) { // Gopt_xxx
		name = name[len(goptPrefix):]
		if m, tname := checkTypeMethod(pkg.Scope(), name); m.typ != nil {
			if debugImport {
				log.Println("==> NewTemplateRecvMethod", tname, m.name)
			}
			NewTemplateRecvMethod(m.typ, token.NoPos, pkg, m.name, o)
		}
	} else if strings.HasPrefix(name, gopxPrefix) { // Gopx_xxx
		aname := name[len(gopxPrefix):]
		o := newFuncEx(token.NoPos, pkg, nil, aname, &tyTypeAsParams{o})
		scope.Insert(o)
		if debugImport {
			log.Println("==> AliasFunc", name, "=>", aname)
		}
	}
}

const (
	goptPrefix = "Gopt_" // template method
	gopoPrefix = "Gopo_" // overload function/method
	gopxPrefix = "Gopx_" // type as parameters function/method
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
		checkGoptGopx(pkg, scope, m.name, o)
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

func overloadNameds(off int, items []*types.Named) []*types.Named {
	nameds := make([]*types.Named, len(items))
	for _, item := range items {
		name := item.Obj().Name()
		idx := toIndex(name[off])
		if idx >= len(items) {
			log.Panicf("overload type %v out of range 0..%v\n", name, len(nameds)-1)
		}
		if nameds[idx] != nil {
			log.Panicf("overload type %v exists?\n", name)
		}
		nameds[idx] = item
	}
	return nameds
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
	stdPkg        func(pkgPath string) bool
}

func NewContext(isPkgtStandard func(pkgPath string) bool) *Context {
	if isPkgtStandard == nil {
		isPkgtStandard = isStdPkg
	}
	return &Context{
		chkGopImports: make(map[string]bool),
		stdPkg:        isPkgtStandard,
	}
}

// initGopPkg initializes a Go+ packages.
func (p *Context) initGopPkg(importer types.Importer, pkgImp *types.Package) {
	pkgPath := pkgImp.Path()
	if p.stdPkg(pkgPath) || p.chkGopImports[pkgPath] {
		return
	}
	if !pkgImp.Complete() {
		importer.Import(pkgPath)
	}
	InitThisGopPkg(pkgImp)
	p.chkGopImports[pkgPath] = true
	for _, imp := range pkgImp.Imports() {
		p.initGopPkg(importer, imp)
	}
}

func isStdPkg(pkgPath string) bool {
	return strings.IndexByte(pkgPath, '.') < 0
}

// ----------------------------------------------------------------------------

func importPkg(this *Package, pkgPath string, src ast.Node) (PkgRef, error) {
	if strings.HasPrefix(pkgPath, ".") { // canonical pkgPath
		pkgPath = path.Join(this.Path(), pkgPath)
	}
	pkgImp, err := this.imp.Import(pkgPath)
	if err != nil {
		e := &ImportError{Path: pkgPath, Err: err}
		if src != nil {
			e.Fset = this.cb.fset
			e.Pos = src.Pos()
		}
		return PkgRef{}, e
	} else {
		this.ctx.initGopPkg(this.imp, pkgImp)
	}
	return PkgRef{Types: pkgImp}, nil
}

// Import imports a package by pkgPath. It will panic if pkgPath not found.
func (p *Package) Import(pkgPath string, src ...ast.Node) PkgRef {
	ret, err := importPkg(p, pkgPath, getSrc(src))
	if err != nil {
		panic(err)
	}
	return ret
}

// ForceImport always imports a package (i.e. `import _ pkgPath`).
func (p *Package) ForceImport(pkgPath string, src ...ast.Node) {
	p.Import(pkgPath, src...)
	p.file.forceImport(pkgPath)
}

// TryImport imports a package by pkgPath. It returns nil if pkgPath not found.
func (p *Package) TryImport(pkgPath string) PkgRef {
	ret, _ := importPkg(p, pkgPath, nil)
	return ret
}

func (p *Package) big() PkgRef {
	if p.pkgBig.isNil() {
		p.pkgBig = p.Import("math/big")
	}
	return p.pkgBig
}

func (p *Package) unsafe() PkgRef {
	return PkgRef{types.Unsafe}
}

// ----------------------------------------------------------------------------

type null struct{}
type autoNames struct {
	names   map[string]null
	reqIdx  int
	autoIdx int
}

const (
	goxAutoPrefix = "_autoGo_"
)

func (p *autoNames) initAutoNames() {
	p.names = make(map[string]null)
}

func (p *autoNames) autoName() string {
	p.autoIdx++
	return goxAutoPrefix + strconv.Itoa(p.autoIdx)
}

func (p *autoNames) useName(name string) {
	p.names[name] = null{}
}

func (p *autoNames) hasName(name string) bool {
	_, ok := p.names[name]
	return ok
}

func (p *autoNames) requireName(name string) (ret string, renamed bool) {
	ret = name
	for p.hasName(ret) {
		p.reqIdx++
		ret = name + strconv.Itoa(p.reqIdx)
		renamed = true
	}
	p.useName(ret)
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
