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

package printer

import (
	"go/ast"
	"go/token"
	"io"
)

type (
	astComment      = ast.Comment
	astCommentGroup = ast.CommentGroup
)

// A CommentedNode bundles an AST node and corresponding comments.
// It may be provided as argument to any of the [Fprint] functions.
type CommentedNode struct {
	Node     any // *ast.File, or ast.Expr, ast.Decl, ast.Spec, or ast.Stmt
	Comments []*ast.CommentGroup
}

// by XGo
type CommentedNodes struct {
	Node           any
	CommentedStmts map[ast.Stmt]*ast.CommentGroup
}

// Fprint "pretty-prints" an AST node to output for a given configuration cfg.
// Position information is interpreted relative to the file set fset.
// The node type must be *[ast.File], *[CommentedNode], [][ast.Decl], [][ast.Stmt],
// or assignment-compatible to [ast.Expr], [ast.Decl], [ast.Spec], or [ast.Stmt].
func (cfg *Config) Fprint(output io.Writer, fset *token.FileSet, node any) error {
	return cfg.fprint(output, fset, node, make(map[ast.Node]int))
}

// Fprint "pretty-prints" an AST node to output.
// It calls [Config.Fprint] with default settings.
// Note that gofmt uses tabs for indentation but spaces for alignment;
// use format.Node (package go/format) for output that matches gofmt.
func Fprint(output io.Writer, fset *token.FileSet, node any) error {
	return (&Config{Tabwidth: 8}).Fprint(output, fset, node)
}
