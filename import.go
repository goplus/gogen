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

package gogen

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
	return isOverload(name) || isGopCommon(name)
}

func isOverload(name string) bool {
	n := len(name)
	return n > 3 && name[n-3:n-1] == "__"
}

// Gop?_xxx
func isGopCommon(name string) bool {
	const n = len(commonPrefix)
	return len(name) > n+2 && name[n+1] == '_' && name[:n] == commonPrefix
}

// InitThisGopPkg initializes a Go+ package.
func InitThisGopPkg(pkg *types.Package) {
	InitThisGopPkgEx(pkg, nil)
}

// InitThisGopPkg initializes a Go+ package. pos map overload name to postion.
func InitThisGopPkgEx(pkg *types.Package, pos map[string]token.Pos) {
	scope := pkg.Scope()
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
			checkGoptsx(pkg, scope, name, o)
		}
	}
	for _, gopoName := range gopos {
		if names, ok := checkOverloads(scope, gopoName); ok {
			key := gopoName[len(gopoPrefix):]
			m, tname := checkTypeMethod(scope, key)
			fns := make([]types.Object, 0, len(names))
			for i, name := range names {
				if name == "" {
					if m.typ != nil {
						name = "."
					}
					name += m.name + "__" + indexTable[i:i+1]
				}
				if obj := lookupFunc(scope, name, tname); obj != nil {
					fns = append(fns, obj)
				}
			}
			if len(fns) > 0 {
				newOverload(pkg, scope, m, fns, pos)
			}
			delete(overloads, m)
		}
	}
	for key, items := range overloads {
		off := len(key.name) + 2
		fns := overloadFuncs(off, items)
		newOverload(pkg, scope, key, fns, pos)
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
func lookupFunc(scope *types.Scope, name, tname string) types.Object {
	if name[0] == '.' {
		name = name[1:]
		tobj := scope.Lookup(tname)
		if tobj != nil {
			if tn, ok := tobj.(*types.TypeName); ok {
				if o, ok := tn.Type().(*types.Named); ok { // TODO(xsw): interface support
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
		if _, ok := o.Type().(*types.Signature); ok {
			return o
		}
	}
	return nil
}

type omthd struct {
	typ  *types.Named
	name string
}

// Func (no _ func name)
// _Func (with _ func name)
// TypeName_Method (no _ method name)
// _TypeName__Method (with _ method name)
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
// Gops_TypeName_Method
// Gops__TypeName__Method
func checkGoptsx(pkg *types.Package, scope *types.Scope, name string, o types.Object) {
	const n = len(commonPrefix)
	const n2 = n + 2
	if isGopCommon(name) {
		switch ch := name[n]; ch {
		case gopsCh, goptCh: // Gops_xxx, Gopt_xxx
			name = name[n2:]
			if m, tname := checkTypeMethod(pkg.Scope(), name); m.typ != nil {
				if ch == goptCh {
					if debugImport {
						log.Println("==> NewTemplateRecvMethod", tname, m.name)
					}
					NewTemplateRecvMethod(m.typ, token.NoPos, pkg, m.name, o)
				} else {
					if debugImport {
						log.Println("==> NewStaticMethod", tname, m.name)
					}
					NewStaticMethod(m.typ, token.NoPos, pkg, m.name, o)
				}
			}
		case gopxCh: // Gopx_xxx
			aname := name[n2:]
			o := newFuncEx(token.NoPos, pkg, nil, aname, &tyTypeAsParams{o})
			scope.Insert(o)
			if debugImport {
				log.Println("==> AliasFunc", name, "=>", aname)
			}
		}
	}
}

const (
	commonPrefix = "Gop"

	goptCh = 't' // template method
	gopsCh = 's' // static method
	gopxCh = 'x' // type as parameters function/method

	goptPrefix = "Gopt_" // template method
	gopsPrefix = "Gops_" // static method
	gopxPrefix = "Gopx_" // type as parameters function/method
	gopoPrefix = "Gopo_" // overload function/method

	gopPackage = "GopPackage"
	gopPkgInit = "__gop_inited"
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
		panic("checkOverloads: should be string constant - " + gopoName)
	}
	return
}

func newOverload(pkg *types.Package, scope *types.Scope, m omthd, fns []types.Object, pos map[string]token.Pos) {
	if m.typ == nil {
		if debugImport {
			log.Println("==> NewOverloadFunc", m.name)
		}
		o := NewOverloadFunc(pos[m.name], pkg, m.name, fns...)
		scope.Insert(o)
		checkGoptsx(pkg, scope, m.name, o)
	} else {
		if debugImport {
			log.Println("==> NewOverloadMethod", m.typ.Obj().Name(), m.name)
		}
		NewOverloadMethod(m.typ, pos[m.typ.Obj().Name()+"."+m.name], pkg, m.name, fns...)
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

type none = struct{}

type expDeps struct {
	this   *types.Package
	ret    map[*types.Package]none
	exists map[types.Type]none
}

func checkGopPkg(pkg *Package) (val ast.Expr, ok bool) {
	if pkg.Types.Name() == "main" || pkg.Types.Scope().Lookup(gopPackage) != nil {
		return
	}
	ed := expDeps{pkg.Types, make(map[*types.Package]none), make(map[types.Type]none)}
	for _, t := range pkg.expObjTypes {
		ed.typ(t)
	}
	var deps []string
	for depPkg := range ed.ret {
		if depPkg.Scope().Lookup(gopPackage) != nil {
			deps = append(deps, depPkg.Path())
		}
	}
	if len(deps) > 0 {
		return stringLit(strings.Join(deps, ",")), true
	}
	if ok = pkg.isGopPkg; ok {
		return identTrue, true
	}
	return
}

func (p expDeps) typ(typ types.Type) {
retry:
	switch t := typ.(type) {
	case *types.Basic: // bool, int, etc
	case *types.Pointer:
		typ = t.Elem()
		goto retry
	case *types.Slice:
		typ = t.Elem()
		goto retry
	case *types.Map:
		p.typ(t.Key())
		typ = t.Elem()
		goto retry
	case *types.Named:
		p.named(t)
	case *types.Signature:
		p.sig(t)
	case *types.Struct:
		p.struc(t)
	case *types.Interface:
		p.interf(t)
	case *types.Chan:
		typ = t.Elem()
		goto retry
	case *types.Array:
		typ = t.Elem()
		goto retry
	case *types.TypeParam, *types.Union:
	default:
		log.Panicf("expDeps: unknown type - %T\n", typ)
	}
}

func (p expDeps) sig(sig *types.Signature) {
	p.tuple(sig.Params())
	p.tuple(sig.Results())
}

func (p expDeps) tuple(v *types.Tuple) {
	for i, n := 0, v.Len(); i < n; i++ {
		p.typ(v.At(i).Type())
	}
}

func (p expDeps) named(t *types.Named) {
	o := t.Obj()
	if at := o.Pkg(); at != nil && at != p.this {
		if _, ok := p.exists[t]; ok {
			return
		}
		p.exists[t] = none{}
		p.ret[at] = none{}
		for i, n := 0, t.NumMethods(); i < n; i++ {
			m := t.Method(i)
			p.method(m)
		}
		p.typ(t.Underlying())
	}
}

func (p expDeps) interf(t *types.Interface) {
	for i, n := 0, t.NumEmbeddeds(); i < n; i++ {
		p.typ(t.EmbeddedType(i))
	}
	for i, n := 0, t.NumExplicitMethods(); i < n; i++ {
		m := t.ExplicitMethod(i)
		p.method(m)
	}
}

func (p expDeps) method(m *types.Func) {
	if m.Exported() {
		if sig := m.Type().(*types.Signature); !isSigFuncEx(sig) {
			p.sig(sig)
		}
	}
}

func (p expDeps) struc(t *types.Struct) {
	for i, n := 0, t.NumFields(); i < n; i++ {
		fld := t.Field(i)
		if fld.Embedded() || fld.Exported() {
			p.typ(fld.Type())
		}
	}
}

// initGopPkg initializes a Go+ packages.
func (p *Package) initGopPkg(importer types.Importer, pkgImp *types.Package) {
	scope := pkgImp.Scope()
	objGopPkg := scope.Lookup(gopPackage)
	if objGopPkg == nil { // not is a Go+ package
		return
	}

	if scope.Lookup(gopPkgInit) != nil { // initialized
		return
	}
	scope.Insert(types.NewConst(
		token.NoPos, pkgImp, gopPkgInit, types.Typ[types.UntypedBool], constant.MakeBool(true),
	))

	pkgDeps, ok := objGopPkg.(*types.Const)
	if !ok {
		return
	}

	var gopDeps []string
	if v := pkgDeps.Val(); v.Kind() == constant.String {
		gopDeps = strings.Split(constant.StringVal(v), ",")
	}

	if debugImport {
		log.Println("==> Import", pkgImp.Path())
	}
	InitThisGopPkg(pkgImp)
	for _, depPath := range gopDeps {
		imp, _ := importer.Import(depPath)
		p.initGopPkg(importer, imp)
	}
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
		this.initGopPkg(this.imp, pkgImp)
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

// ----------------------------------------------------------------------------

type null struct{}

type importName struct {
	name string
	file string
}

type autoNames struct {
	names       map[string]null
	importNames map[importName]null
	autoIdx     int
}

const (
	goxAutoPrefix = "_autoGo_"
)

func (p *autoNames) init() {
	p.names = make(map[string]null)
	p.importNames = make(map[importName]null)
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

func (p *autoNames) useImportName(file, name string) {
	p.importNames[importName{name, file}] = null{}
}

func (p *autoNames) hasImportName(file, name string) bool {
	_, ok := p.importNames[importName{name, file}]
	return ok
}

func (p *autoNames) importName(file, name string) (ret string, renamed bool) {
	ret = name
	var idx int
	for p.hasName(ret) || p.hasImportName(file, ret) {
		idx++
		ret = name + strconv.Itoa(idx)
		renamed = true
	}
	p.useImportName(file, ret)
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
