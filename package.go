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
	"sort"
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
	// Call maps func to the the objects they denote.
	Call(fn ast.Node, obj types.Object)
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

	// (internal) only for testing
	DbgPositioner dbgPositioner

	// NoSkipConstant is to disable optimization of skipping constant (optional).
	NoSkipConstant bool
}

// ----------------------------------------------------------------------------

type importUsed bool

type File struct {
	decls []ast.Decl
	fname string
	imps  map[string]*ast.Ident // importPath => impRef
}

func newFile(fname string) *File {
	return &File{fname: fname, imps: make(map[string]*ast.Ident)}
}

func (p *File) newImport(name, pkgPath string) *ast.Ident {
	id := p.imps[pkgPath]
	if id == nil {
		id = &ast.Ident{Name: name, Obj: &ast.Object{Data: importUsed(false)}}
		p.imps[pkgPath] = id
	}
	return id
}

func (p *File) forceImport(pkgPath string) {
	if _, ok := p.imps[pkgPath]; !ok {
		p.imps[pkgPath] = nil
	}
}

// Name returns the name of this file.
func (p *File) Name() string {
	return p.fname
}

type astVisitor struct {
	this *Package
}

func (p astVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return nil
	}
	switch v := node.(type) {
	case *ast.CommentGroup, *ast.Ident, *ast.BasicLit:
	case *ast.SelectorExpr:
		x := v.X
		if id, ok := x.(*ast.Ident); ok && id.Obj != nil {
			if used, ok := id.Obj.Data.(importUsed); ok && bool(!used) {
				id.Obj.Data = importUsed(true)
				if name, renamed := p.this.requireName(id.Name); renamed {
					id.Name = name
					id.Obj.Name = name
				}
			}
		} else {
			ast.Walk(p, x)
		}
	case *ast.FuncDecl:
		ast.Walk(p, v.Type)
		if v.Body != nil {
			ast.Walk(p, v.Body)
		}
	case *ast.ValueSpec:
		if v.Type != nil {
			ast.Walk(p, v.Type)
		}
		for _, val := range v.Values {
			ast.Walk(p, val)
		}
	case *ast.TypeSpec:
		ast.Walk(p, v.Type)
	case *ast.BranchStmt:
	case *ast.LabeledStmt:
		ast.Walk(p, v.Stmt)
	default:
		return p
	}
	return nil
}

func (p astVisitor) markUsed(decls []ast.Decl) {
	for _, decl := range decls {
		ast.Walk(p, decl)
	}
}

func (p *File) getDecls(this *Package) (decls []ast.Decl) {
	astVisitor{this}.markUsed(p.decls)

	specs := make([]ast.Spec, 0, len(p.imps))
	for pkgPath, id := range p.imps {
		if id == nil { // force-used
			specs = append(specs, &ast.ImportSpec{
				Name: underscore, // _
				Path: stringLit(pkgPath),
			})
		} else if id.Obj.Data.(importUsed) {
			var name *ast.Ident
			if id.Obj.Name != "" {
				name = ident(id.Obj.Name)
			}
			specs = append(specs, &ast.ImportSpec{
				Name: name,
				Path: stringLit(pkgPath),
			})
		}
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].(*ast.ImportSpec).Path.Value < specs[j].(*ast.ImportSpec).Path.Value
	})

	var valGopPkg ast.Expr
	var addGopPkg bool
	if p.fname == this.conf.DefaultGoFile {
		valGopPkg, addGopPkg = checkGopPkg(this)
	}
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
					valGopPkg,
				},
			},
		}})
	}
	return append(decls, p.decls...)
}

// ----------------------------------------------------------------------------

// ObjectDocs maps an object to its document.
type ObjectDocs = map[types.Object]*ast.CommentGroup

// Package type
type Package struct {
	PkgRef
	Docs ObjectDocs
	Fset *token.FileSet

	autoNames
	cb             CodeBuilder
	imp            types.Importer
	files          map[string]*File
	file           *File
	conf           *Config
	builtin        PkgRef
	pkgBig         PkgRef
	utBigInt       *types.Named
	utBigRat       *types.Named
	utBigFlt       *types.Named
	commentedStmts map[ast.Stmt]*ast.CommentGroup
	implicitCast   func(pkg *Package, V, T types.Type, pv *Element) bool

	expObjTypes []types.Type // types of export objects
	isGopPkg    bool
	allowRedecl bool // for c2go
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
	imp := conf.Importer
	if imp == nil {
		imp = packages.NewImporter(fset)
	}
	newBuiltin := conf.NewBuiltin
	if newBuiltin == nil {
		newBuiltin = newBuiltinDefault
	}
	fname := conf.DefaultGoFile
	file := newFile(fname)
	files := map[string]*File{fname: file}
	pkg := &Package{
		Fset:  fset,
		file:  file,
		files: files,
		conf:  conf,
	}
	pkg.initAutoNames()
	pkg.imp = imp
	pkg.Types = conf.Types
	if pkg.Types == nil {
		pkg.Types = types.NewPackage(pkgPath, name)
	}
	pkg.builtin.Types = newBuiltin(pkg, conf)
	pkg.implicitCast = conf.CanImplicitCast
	pkg.utBigInt = conf.UntypedBigInt
	pkg.utBigRat = conf.UntypedBigRat
	pkg.utBigFlt = conf.UntypedBigFloat
	pkg.cb.init(pkg)
	return pkg
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
func (p *Package) Builtin() PkgRef {
	return p.builtin
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
			f = newFile(fname)
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
func (p *Package) RestoreCurFile(file *File) (old *File) {
	old = p.file
	p.file = file
	return
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
