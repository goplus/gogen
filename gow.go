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
	"io"
	"log"
	"os"
	"syscall"

	"github.com/goplus/gox/internal/go/format"
	"github.com/goplus/gox/internal/go/printer"
)

// ----------------------------------------------------------------------------

// TypeAST returns the AST of specified `typ`.
func TypeAST(pkg *Package, typ types.Type) ast.Expr {
	return toType(pkg, typ)
}

// ASTFile returns AST of a file by it `fname`.
// If `fname` is not provided, it returns AST of the default (NOT current) file.
func ASTFile(pkg *Package, fname ...string) *ast.File {
	f, ok := pkg.File(fname...)
	if !ok {
		return nil
	}
	if debugWriteFile {
		log.Println("==> ASTFile", f.Name())
	}
	decls := f.getDecls(pkg)
	return &ast.File{Name: ident(pkg.Types.Name()), Decls: decls, Imports: getImports(decls)}
}

func getImports(decls []ast.Decl) []*ast.ImportSpec {
	if len(decls) > 0 {
		if decl, ok := decls[0].(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
			n := len(decl.Specs)
			ret := make([]*ast.ImportSpec, n)
			for i, spec := range decl.Specs {
				ret[i] = spec.(*ast.ImportSpec)
			}
			return ret
		}
	}
	return nil
}

// CommentedASTFile returns commented AST of a file by it `fname`.
// If `fname` is not provided, it returns AST of the default (NOT current) file.
func CommentedASTFile(pkg *Package, fname ...string) *printer.CommentedNodes {
	f := ASTFile(pkg, fname...)
	if f == nil {
		return nil
	}
	return &printer.CommentedNodes{
		Node:           f,
		CommentedStmts: pkg.commentedStmts,
	}
}

// WriteTo writes a file named `fname` to `dst`.
// If `fname` is not provided, it writes the default (NOT current) file.
func WriteTo(dst io.Writer, pkg *Package, fname ...string) (err error) {
	file := CommentedASTFile(pkg, fname...)
	if file == nil {
		return syscall.ENOENT
	}
	fset := token.NewFileSet()
	return format.Node(dst, fset, file)
}

// WriteFile writes a `file` named `fname`.
// If `fname` is not provided, it writes the default (NOT current) file.
func WriteFile(file string, pkg *Package, fname ...string) (err error) {
	ast := CommentedASTFile(pkg, fname...)
	if ast == nil {
		return syscall.ENOENT
	}
	if debugWriteFile {
		log.Println("WriteFile", file)
	}
	f, err := os.Create(file)
	if err != nil {
		return
	}
	err = syscall.EFAULT
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(file)
		}
	}()
	fset := token.NewFileSet()
	return format.Node(f, fset, ast)
}

// ----------------------------------------------------------------------------
