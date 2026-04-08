//go:build genjs

/*
Copyright 2026 The XGo Authors (xgo.dev)
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
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/goplus/gogen/internal/target/util"
	"github.com/goplus/gogen/target/js"
)

// ----------------------------------------------------------------------------

type termChecker struct {
	panicCalls map[*js.CallExpr]none
}

// isTerminating reports whether s is a terminating statement.
func (c *termChecker) isTerminating(s js.Stmt, label string) bool {
	return true // TODO(xsw): implement this
}

// ----------------------------------------------------------------------------

type pkgSymbol struct {
	pkg  *ast.Ident
	name string
}

type jsImportPkg struct {
	pkgPath string
	decl    *js.ImportDecl
}

type jsVisitor struct {
	decls []*js.ImportDecl
	imps  map[*ast.Ident]*jsImportPkg
	syms  map[pkgSymbol]string
	file  *File
	this  *Package
}

func (p *jsVisitor) Visit(node js.Node) (w js.Visitor) {
	if node == nil {
		return nil
	}
	switch v := node.(type) {
	case *js.CommentGroup, *js.Ident, *js.BasicLit:
	case *js.SelectorExpr:
		x := v.X
		if fake, ok := x.(*util.FakeExpr); ok {
			if id, ok := fake.Real.(*ast.Ident); ok && id.Obj != nil {
				if pkg, ok := p.imps[id]; ok {
					if pkg.decl == nil {
						decl := &js.ImportDecl{
							Path: &js.BasicLit{
								Kind:  token.STRING,
								Value: strconv.Quote(pkg.pkgPath),
							},
						}
						p.decls = append(p.decls, decl)
						pkg.decl = decl
					}
					symName := v.Sel.Name
					symKey := pkgSymbol{id, symName}
					if newName, ok := p.syms[symKey]; ok {
						v.Sel.Name = newName
					} else {
						newName, renamed := p.this.importName(p.file.Name(), symName)
						if renamed {
							v.Sel.Name = newName
						}
						p.syms[symKey] = newName
					}
					v.X = nil // remove the package qualifier
				}
			}
		} else {
			js.Walk(p, x)
		}
	case *js.BranchStmt:
	case *js.LabeledStmt:
		js.Walk(p, v.Stmt)
	default:
		return p
	}
	return nil
}

func jsImportDecls(this *Package, file *File) []*js.ImportDecl {
	imps := make(map[*ast.Ident]*jsImportPkg)
	for pkgPath, id := range file.imps {
		imps[id] = &jsImportPkg{pkgPath: pkgPath}
	}
	p := &jsVisitor{
		imps: imps,
		syms: make(map[pkgSymbol]string),
		file: file,
		this: this,
	}
	for _, decl := range file.jsDecls {
		switch d := decl.(type) {
		case *funcDecl:
			js.Walk(p, d.Body)
		}
	}
	return p.decls
}

// ----------------------------------------------------------------------------

type jsDecl interface {
	declNode()
}

type funcDecl struct {
	ast.FuncDecl
	sig  *types.Signature
	Body *js.BlockStmt
}

func (*funcDecl) declNode() {}

type valueSpec struct {
	ast.ValueSpec
	Values []js.Expr
}

func asValueSpec(spec *valueSpec) *valueSpec {
	return spec
}

type valDecl struct {
	ast.GenDecl
	Specs []*valueSpec
}

func (*valDecl) declNode() {}

type typeDecl struct {
	ast.GenDecl
}

func (*typeDecl) declNode() {}

type fileDecls struct {
	goDecls []ast.Decl
	jsDecls []jsDecl
}

func (p *fileDecls) appendFuncDecl(decl *funcDecl, sig *types.Signature) {
	decl.sig = sig
	p.goDecls = append(p.goDecls, &decl.FuncDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

func (p *fileDecls) appendValDecl(decl *valDecl) {
	p.goDecls = append(p.goDecls, &decl.GenDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

func startValDeclStmtAt(cb *CodeBuilder, decl *valDecl) int {
	panic("todo")
}

func (p *fileDecls) appendTypeDecl(decl *typeDecl) {
	p.goDecls = append(p.goDecls, &decl.GenDecl)
	p.jsDecls = append(p.jsDecls, decl)
}

// ----------------------------------------------------------------------------

func (p *File) getJSFile(_ *Package) *js.File {
	decls := make([]js.Stmt, 0, len(p.jsDecls))
	for _, decl := range p.jsDecls {
		switch d := decl.(type) {
		case *funcDecl:
			sig := d.sig
			if sig.Recv() != nil {
				panic("todo")
			}
			in := sig.Params()
			n := in.Len()
			params := make([]*js.Ident, n)
			for i := range n {
				params[i] = &js.Ident{Name: in.At(i).Name()}
			}
			decls = append(decls, &js.FuncDecl{
				Name:   &js.Ident{Name: d.Name.Name},
				Params: params,
				Body:   d.Body,
			})
		}
	}
	return &js.File{Stmts: decls}
}

// ----------------------------------------------------------------------------
