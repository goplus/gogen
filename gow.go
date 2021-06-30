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
func From(pkg *Package) (file *ast.File, err error) {
	return &ast.File{Name: ident(pkg.Name), Decls: pkg.decls}, nil
}

// WriteTo func
func WriteTo(dst io.Writer, pkg *Package) (err error) {
	fset := token.NewFileSet()
	file, err := From(pkg)
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
