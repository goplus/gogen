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
	"path"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/goplus/gox/packages"
)

type LoadNamedFunc = func(at *Package, typ *types.Named)

const (
	DbgFlagInstruction = 1 << iota
	DbgFlagImport
	DbgFlagMatch
	DbgFlagComments
	DbgFlagWriteFile
	DbgFlagSetDebug
	DbgFlagPersistCache
	DbgFlagAll = DbgFlagInstruction | DbgFlagImport | DbgFlagMatch |
		DbgFlagComments | DbgFlagWriteFile | DbgFlagSetDebug | DbgFlagPersistCache
)

var (
	debugInstr     bool
	debugMatch     bool
	debugImport    bool
	debugComments  bool
	debugWriteFile bool
	debugImportIox bool
)

func SetDebug(dbgFlags int) {
	debugInstr = (dbgFlags & DbgFlagInstruction) != 0
	debugImport = (dbgFlags & DbgFlagImport) != 0
	debugMatch = (dbgFlags & DbgFlagMatch) != 0
	debugComments = (dbgFlags & DbgFlagComments) != 0
	debugWriteFile = (dbgFlags & DbgFlagWriteFile) != 0
	if (dbgFlags & DbgFlagSetDebug) != 0 {
		log.Printf("SetDebug: import=%v, match=%v, instr=%v\n", debugImport, debugMatch, debugInstr)
	}
}

type fatalMsg string

func fatal(msg string) {
	panic(fatalMsg(msg))
}

// ----------------------------------------------------------------------------

// Recorder represents a gox event recorder.
type Recorder interface {
	// Member maps identifiers to the objects they denote.
	Member(id ast.Node, obj types.Object)
}

// ----------------------------------------------------------------------------

type dbgPositioner interface {
	// Position gets position of a Pos.
	Position(p token.Pos) token.Position
}

type NodeInterpreter interface {
	// LoadExpr is called to load an expr code.
	LoadExpr(expr ast.Node) string
}

// Config type
type Config struct {
	// Types provides type information for the package (optional).
	Types *types.Package

	// Fset provides source position information for syntax trees and types (optional).
	// If Fset is nil, Load will use a new fileset, but preserve Fset's value.
	Fset *token.FileSet

	// Context represents all things between packages (optional).
	Context *Context

	// HandleErr is called to handle errors (optional).
	HandleErr func(err error)

	// NodeInterpreter is to interpret an ast.Node (optional).
	NodeInterpreter NodeInterpreter

	// LoadNamed is called to load a delay-loaded named type (optional).
	LoadNamed LoadNamedFunc

	// An Importer resolves import paths to Packages (optional).
	Importer types.Importer

	// DefaultGoFile specifies default file name. It can be empty.
	DefaultGoFile string

	// PkgPathIox specifies package path of github.com/goplus/gop/builtin/iox
	PkgPathIox string

	// NewBuiltin is to create the builin package (optional).
	NewBuiltin func(pkg *Package, conf *Config) *types.Package

	// CanImplicitCast checkes can cast V to T implicitly (optional).
	CanImplicitCast func(pkg *Package, V, T types.Type, pv *Element) bool

	// untyped bigint, untyped bigrat, untyped bigfloat (optional).
	UntypedBigInt, UntypedBigRat, UntypedBigFloat *types.Named

	// A Recorder records selected objects such as methods, etc (optional).
	Recorder Recorder

	// NoSkipConstant is to disable optimization of skipping constant (optional).
	NoSkipConstant bool

	// (internal) only for testing
	DbgPositioner dbgPositioner
}

// ----------------------------------------------------------------------------

type File struct {
	decls        []ast.Decl
	allPkgPaths  []string
	importPkgs   map[string]*PkgRef
	pkgBig       *PkgRef
	pkgUnsafe    *PkgRef
	fname        string
	removedExprs bool
	defaultFile  bool
}

// Name returns the name of this file.
func (p *File) Name() string {
	return p.fname
}

func (p *File) importPkg(this *Package, pkgPath string, src ast.Node) *PkgRef {
	if strings.HasPrefix(pkgPath, ".") { // canonical pkgPath
		pkgPath = path.Join(this.Path(), pkgPath)
	}
	pkgImport, ok := p.importPkgs[pkgPath]
	if !ok {
		pkgImp, err := this.imp.Import(pkgPath)
		if err != nil {
			e := &ImportError{Path: pkgPath, Err: err}
			if src != nil {
				e.Fset = this.cb.fset
				e.Pos = src.Pos()
			}
			panic(e)
		} else {
			this.ctx.InitGopPkg(this, this.imp, pkgImp)
		}
		pkgImport = &PkgRef{Types: pkgImp}
		pkgImport.overloadFuncs = this.overloadFuncs
		pkgImport.overloadMethods = this.overloadMethods
		p.importPkgs[pkgPath] = pkgImport
		p.allPkgPaths = append(p.allPkgPaths, pkgPath)
	}
	return pkgImport
}

func (p *File) markUsed(this *Package) {
	if p.removedExprs {
		// travel all ast nodes to mark used
		p.markUsedBy(this, reflect.ValueOf(p.decls))
		return
	}
	// no removed exprs, mark used simplely
	for _, pkgImport := range p.importPkgs {
		if pkgImport.nameRefs != nil {
			pkgImport.isUsed = true
		}
	}
}

var (
	tyAstNode    = reflect.TypeOf((*ast.Node)(nil)).Elem()
	tySelExprPtr = reflect.TypeOf((*ast.SelectorExpr)(nil))
)

func (p *File) markUsedBy(this *Package, val reflect.Value) {
retry:
	switch val.Kind() {
	case reflect.Slice:
		for i, n := 0, val.Len(); i < n; i++ {
			p.markUsedBy(this, val.Index(i))
		}
	case reflect.Ptr:
		if val.IsNil() {
			return
		}
		t := val.Type()
		if t == tySelExprPtr {
			x := val.Interface().(*ast.SelectorExpr).X
			if sym, ok := x.(*ast.Ident); ok {
				name := sym.Name
				for _, at := range p.importPkgs {
					if at.Types != nil && at.Types.Name() == name { // pkg.Object
						at.markUsed(sym)
					}
				}
			} else {
				p.markUsedBy(this, reflect.ValueOf(x))
			}
		} else if t.Implements(tyAstNode) { // ast.Node
			elem := val.Elem()
			for i, n := 0, elem.NumField(); i < n; i++ {
				p.markUsedBy(this, elem.Field(i))
			}
		}
	case reflect.Interface:
		val = val.Elem()
		goto retry
	}
}

func (p *File) getDecls(this *Package) (decls []ast.Decl) {
	p.markUsed(this)
	n := len(p.allPkgPaths)
	if n == 0 {
		return p.decls
	}
	specs := make([]ast.Spec, 0, n)
	names := this.newAutoNames()
	for _, pkgPath := range p.allPkgPaths {
		pkgImport := p.importPkgs[pkgPath]
		if !pkgImport.isUsed { // unused
			if pkgImport.isForceUsed { // force-used
				specs = append(specs, &ast.ImportSpec{
					Name: underscore, // _
					Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(pkgPath)},
				})
			}
			continue
		}
		pkgName, renamed := names.RequireName(pkgImport.Types.Name())
		var name *ast.Ident
		if renamed {
			for _, nameRef := range pkgImport.nameRefs {
				nameRef.Name = pkgName
			}
			name = ident(pkgName)
		} else {
			if pkgName != path.Base(pkgImport.Types.Path()) {
				name = ident(pkgName)
			}
		}
		specs = append(specs, &ast.ImportSpec{
			Name: name,
			Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(pkgPath)},
		})
	}
	addGopPkg := p.defaultFile && shouldAddGopPkg(this)
	if len(specs) == 0 && !addGopPkg {
		return p.decls
	}
	decls = make([]ast.Decl, 0, len(p.decls)+2)
	decls = append(decls, &ast.GenDecl{Tok: token.IMPORT, Specs: specs})
	if addGopPkg {
		decls = append(decls, &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{{Name: gopPackage}},
				Values: []ast.Expr{
					&ast.Ident{Name: "true"},
				},
			},
		}})
	}
	decls = append(decls, p.decls...)
	return
}

func (p *File) big(this *Package) *PkgRef {
	if p.pkgBig == nil {
		p.pkgBig = p.importPkg(this, "math/big", nil)
	}
	return p.pkgBig
}

func (p *File) unsafe(this *Package) *PkgRef {
	if p.pkgUnsafe == nil {
		p.pkgUnsafe = p.importPkg(this, "unsafe", nil)
	}
	return p.pkgUnsafe
}

// ----------------------------------------------------------------------------

// ObjectDocs maps an object to its document.
type ObjectDocs = map[types.Object]*ast.CommentGroup

// Package type
type Package struct {
	PkgRef
	Docs ObjectDocs
	Fset *token.FileSet

	cb             CodeBuilder
	imp            types.Importer
	files          map[string]*File
	file           *File
	conf           *Config
	ctx            *Context
	builtin        *types.Package
	utBigInt       *types.Named
	utBigRat       *types.Named
	utBigFlt       *types.Named
	autoIdx        int
	commentedStmts map[ast.Stmt]*ast.CommentGroup
	implicitCast   func(pkg *Package, V, T types.Type, pv *Element) bool
	allowRedecl    bool // for c2go
	isGopPkg       bool

	overloadFuncs   map[*types.Package][]*types.Func
	overloadMethods map[types.Type][]*types.Func
}

const (
	goxPrefix = "Gop_"
)

// NewPackage creates a new package.
func NewPackage(pkgPath, name string, conf *Config) *Package {
	if conf == nil {
		conf = new(Config)
	}
	fset := conf.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	ctx := conf.Context
	if ctx == nil {
		ctx = NewContext()
	}
	imp := conf.Importer
	if imp == nil {
		imp = packages.NewImporter(fset)
	}
	newBuiltin := conf.NewBuiltin
	if newBuiltin == nil {
		newBuiltin = newBuiltinDefault
	}
	fname := conf.DefaultGoFile
	file := &File{importPkgs: make(map[string]*PkgRef), fname: fname, defaultFile: true}
	files := map[string]*File{fname: file}
	pkg := &Package{
		Fset:  fset,
		file:  file,
		files: files,
		conf:  conf,
		ctx:   ctx,
	}
	pkg.imp = imp
	pkg.Types = conf.Types
	if pkg.Types == nil {
		pkg.Types = types.NewPackage(pkgPath, name)
	}
	pkg.overloadFuncs = make(map[*types.Package][]*types.Func)
	pkg.overloadMethods = make(map[types.Type][]*types.Func)
	pkg.builtin = newBuiltin(pkg, conf)
	pkg.implicitCast = conf.CanImplicitCast
	pkg.utBigInt = conf.UntypedBigInt
	pkg.utBigRat = conf.UntypedBigRat
	pkg.utBigFlt = conf.UntypedBigFloat
	pkg.cb.init(pkg)
	return pkg
}

func (p *Package) FindOverload(pkg *types.Package, name string) types.Object {
	if funcs, ok := p.overloadFuncs[pkg]; ok {
		for _, fn := range funcs {
			if fn.Name() == name {
				return fn
			}
		}
	}
	return nil
}

func (p *Package) setDoc(o types.Object, doc *ast.CommentGroup) {
	if p.Docs == nil {
		p.Docs = make(ObjectDocs)
	}
	p.Docs[o] = doc
}

func (p *Package) setStmtComments(stmt ast.Stmt, comments *ast.CommentGroup) {
	if p.commentedStmts == nil {
		p.commentedStmts = make(map[ast.Stmt]*ast.CommentGroup)
	}
	p.commentedStmts[stmt] = comments
}

// SetRedeclarable sets to allow redeclaration of variables/functions or not.
func (p *Package) SetRedeclarable(allowRedecl bool) {
	p.allowRedecl = allowRedecl
}

// Sizeof returns sizeof typ in bytes.
func (p *Package) Sizeof(typ types.Type) int64 {
	return align(std.Sizeof(typ), std.Alignof(typ))
}

// align returns the smallest y >= x such that y % a == 0.
func align(x, a int64) int64 {
	y := x + a - 1
	return y - y%a
}

func (p *Package) Offsetsof(fields []*types.Var) []int64 {
	return std.Offsetsof(fields)
}

// Builtin returns the buitlin package.
func (p *Package) Builtin() *PkgRef {
	return &PkgRef{Types: p.builtin}
}

// CB returns the code builder.
func (p *Package) CB() *CodeBuilder {
	return &p.cb
}

// SetCurFile sets new current file to write.
// If createIfNotExists is true, then create a new file named `fname` if it not exists.
// It returns an `old` file to restore in the future (by calling `RestoreCurFile`).
func (p *Package) SetCurFile(fname string, createIfNotExists bool) (old *File, err error) {
	old = p.file
	f, ok := p.files[fname]
	if !ok {
		if createIfNotExists {
			f = &File{importPkgs: make(map[string]*PkgRef), fname: fname}
			p.files[fname] = f
		} else {
			return nil, syscall.ENOENT
		}
	}
	p.file = f
	return
}

// CurFile returns the current file.
func (p *Package) CurFile() *File {
	return p.file
}

// RestoreCurFile sets current file to an `old` file that was returned by `SetCurFile`.
func (p *Package) RestoreCurFile(file *File) {
	p.file = file
}

// File returns a file by its name.
// If `fname` is not provided, it returns the default (NOT current) file.
func (p *Package) File(fname ...string) (file *File, ok bool) {
	var name string
	if len(fname) == 1 {
		name = fname[0]
	} else {
		name = p.conf.DefaultGoFile
	}
	file, ok = p.files[name]
	return
}

// ForEachFile walks all files to `doSth`.
func (p *Package) ForEachFile(doSth func(fname string, file *File)) {
	for fname, file := range p.files {
		doSth(fname, file)
	}
}

// ----------------------------------------------------------------------------
