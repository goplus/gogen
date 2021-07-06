package gox

import (
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
)

// ----------------------------------------------------------------------------

// ASTFile func
func ASTFile(pkg *Package) *ast.File {
	return &ast.File{Name: ident(pkg.Types.Name()), Decls: pkg.getDecls()}
}

// WriteTo func
func WriteTo(dst io.Writer, pkg *Package) (err error) {
	fset := token.NewFileSet()
	return format.Node(dst, fset, ASTFile(pkg))
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
