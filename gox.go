package gox

import (
	"go/ast"
	"go/token"
	"io"

	"github.com/goplus/gox/conv"
	"github.com/goplus/gox/dom"
)

// ----------------------------------------------------------------------------

// Var type
type Var = dom.Var

// Options type
type Options = dom.Options

// Package type
type Package = dom.Package

// NewPkg func
func NewPkg(name string, opts ...*Options) *Package {
	var theOpts *Options
	if opts != nil {
		theOpts = opts[0]
	} else {
		theOpts = &Options{}
	}
	return dom.NewPkg(name, theOpts)
}

// ----------------------------------------------------------------------------

// From func
func From(fset *token.FileSet, pkg *dom.Package) (file *ast.File, err error) {
	return conv.From(fset, pkg)
}

// WriteTo func
func WriteTo(dst io.Writer, pkg *dom.Package) (err error) {
	return conv.WriteTo(dst, pkg)
}

// WriteFile func
func WriteFile(file string, pkg *dom.Package) (err error) {
	return conv.WriteFile(file, pkg)
}

// ----------------------------------------------------------------------------
