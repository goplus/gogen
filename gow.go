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
	"io"
	"log"
	"os"
	"syscall"

	"github.com/goplus/gox/internal/go/format"
	"github.com/goplus/gox/internal/go/printer"
)

// ----------------------------------------------------------------------------

// ASTFile func
func ASTFile(pkg *Package, testingFile bool) *ast.File {
	idx := getInTestingFile(testingFile)
	return &ast.File{Name: ident(pkg.Types.Name()), Decls: pkg.files[idx].getDecls(pkg)}
}

// CommentedASTFile func
func CommentedASTFile(pkg *Package, testingFile bool) *printer.CommentedNodes {
	return &printer.CommentedNodes{
		Node:           ASTFile(pkg, testingFile),
		CommentedStmts: pkg.commentedStmts,
	}
}

// WriteTo func
func WriteTo(dst io.Writer, pkg *Package, testingFile bool) (err error) {
	fset := token.NewFileSet()
	return format.Node(dst, fset, CommentedASTFile(pkg, testingFile))
}

// WriteFile func
func WriteFile(file string, pkg *Package, testingFile bool) (err error) {
	if debugWriteFile {
		log.Println("WriteFile", file, "testing:", testingFile)
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
	return WriteTo(f, pkg, testingFile)
}

// ----------------------------------------------------------------------------
