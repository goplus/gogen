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
	"go/types"

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
