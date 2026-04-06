//go:build genjs
// +build genjs

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
	"go/token"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/goplus/gogen/target/js"
	"github.com/goplus/gogen/target/js/format"
	"github.com/goplus/gogen/target/js/printer"
)

// ----------------------------------------------------------------------------

// JSFile returns AST of a JavaScript file by its fname.
// If fname is not provided, it returns AST of the default (NOT current) file.
func (p *Package) JSFile(fname ...string) *js.File {
	f, ok := p.File(fname...)
	if !ok {
		return nil
	}
	if debugWriteFile {
		log.Println("==> JSFile", f.Name())
	}
	return f.getJSFile(p)
}

// CommentedJSFile returns commented AST of a JavaScript file by its fname.
// If fname is not provided, it returns AST of the default (NOT current) file.
func (p *Package) CommentedJSFile(fname ...string) *printer.CommentedNodes {
	f := p.ASTFile(fname...)
	if f == nil {
		return nil
	}
	return &printer.CommentedNodes{
		Node:           f,
		CommentedStmts: p.commentedStmts,
	}
}

// WriteJSTo writes a JavaScript file named fname to dst.
// If fname is not provided, it writes the default (NOT current) file.
func (p *Package) WriteJSTo(dst io.Writer, fname ...string) (err error) {
	file := p.CommentedJSFile(fname...)
	if file == nil {
		return syscall.ENOENT
	}
	fset := token.NewFileSet()
	return format.Node(dst, fset, file)
}

// WriteJSFile writes a JavaScript file named fname.
// If fname is not provided, it writes the default (NOT current) file.
func (p *Package) WriteJSFile(file string, fname ...string) (err error) {
	ast := p.CommentedJSFile(fname...)
	if ast == nil {
		return syscall.ENOENT
	}
	if debugWriteFile {
		log.Println("WriteJSFile", file)
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
	if GeneratedHeader != "" {
		f.WriteString(GeneratedHeader)
	}
	fset := token.NewFileSet()
	return format.Node(f, fset, ast)
}

// ----------------------------------------------------------------------------
