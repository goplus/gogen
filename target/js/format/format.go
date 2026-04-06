// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package format implements standard formatting of JavaScript source.
package format

import (
	"go/token"
	"io"

	"github.com/goplus/gogen/target/js/printer"
)

// Keep these in sync with cmd/gofmt/gofmt.go.
const (
	tabWidth    = 8
	printerMode = printer.UseSpaces | printer.TabIndent | printerNormalizeNumbers

	// printerNormalizeNumbers means to canonicalize number literal prefixes
	// and exponents while printing. See https://golang.org/doc/go1.13#gofmt.
	//
	// This value is defined in go/printer specifically for go/format and cmd/gofmt.
	printerNormalizeNumbers = 1 << 30
)

var config = printer.Config{Mode: printerMode, Tabwidth: tabWidth}

// const parserMode = parser.ParseComments | parser.SkipObjectResolution

// Node formats node in canonical gofmt style and writes the result to dst.
//
// The function may return early (before the entire result is written)
// and return a formatting error, for instance due to an incorrect AST.
func Node(dst io.Writer, fset *token.FileSet, node any) error {
	/* TODO(xsw):
	// Determine if we have a complete source file (file != nil).
	var file *js.File
	var cnode *printer.CommentedNode
	switch n := node.(type) {
	case *js.File:
		file = n
	case *printer.CommentedNode:
		if f, ok := n.Node.(*js.File); ok {
			file = f
			cnode = n
		}
	}

	// Sort imports if necessary.
	if file != nil && hasUnsortedImports(file) {
		// Make a copy of the AST because js.SortImports is destructive.
		// TODO(gri) Do this more efficiently.
		var buf bytes.Buffer
		err := config.Fprint(&buf, fset, file)
		if err != nil {
			return err
		}
		file, err = parser.ParseFile(fset, "", buf.Bytes(), parserMode)
		if err != nil {
			// We should never get here. If we do, provide good diagnostic.
			return fmt.Errorf("format.Node internal error (%s)", err)
		}
		js.SortImports(fset, file)

		// Use new file with sorted imports.
		node = file
		if cnode != nil {
			node = &printer.CommentedNode{Node: file, Comments: cnode.Comments}
		}
	}
	*/
	return config.Fprint(dst, fset, node)
}

/* TODO(xsw):
// Source formats src in canonical gofmt style and returns the result
// or an (I/O or syntax) error. src is expected to be a syntactically
// correct Go source file, or a list of Go declarations or statements.
//
// If src is a partial source file, the leading and trailing space of src
// is applied to the result (such that it has the same leading and trailing
// space as src), and the result is indented by the same amount as the first
// line of src containing code. Imports are not sorted for partial source files.
func Source(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, sourceAdj, indentAdj, err := parse(fset, "", src, true)
	if err != nil {
		return nil, err
	}

	if sourceAdj == nil {
		// Complete source file.
		// TODO(gri) consider doing this always.
		js.SortImports(fset, file)
	}

	return format(fset, file, sourceAdj, indentAdj, src, config)
}

func hasUnsortedImports(file *js.File) bool {
	for _, d := range file.Decls {
		d, ok := d.(*js.GenDecl)
		if !ok || d.Tok != token.IMPORT {
			// Not an import declaration, so we're done.
			// Imports are always first.
			return false
		}
		if d.Lparen.IsValid() {
			// For now assume all grouped imports are unsorted.
			// TODO(gri) Should check if they are sorted already.
			return true
		}
		// Ungrouped imports are sorted by default.
	}
	return false
}
*/
