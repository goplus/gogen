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

// Config type
type Config = dom.Config

// Package type
type Package = dom.Package

// NewPackage func
func NewPackage(name string, conf *dom.Config) *dom.Package {
	return dom.NewPackage(name, conf)
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
