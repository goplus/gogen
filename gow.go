package gox

import (
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
)

// ----------------------------------------------------------------------------

// From func
func From(fset *token.FileSet, pkg *Package) (file *ast.File, err error) {
	return
}

// WriteTo func
func WriteTo(dst io.Writer, pkg *Package) (err error) {
	fset := token.NewFileSet()
	file, err := From(fset, pkg)
	if err != nil {
		return
	}
	return format.Node(dst, fset, file)
}

// WriteFile func
func WriteFile(file string, pkg *Package) (err error) {
	f, err := os.Create(file)
	if err != nil {
		return
	}
	defer f.Close()
	return WriteTo(f, pkg)
}

// ----------------------------------------------------------------------------
