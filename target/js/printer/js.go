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

	"github.com/goplus/gogen/target/js"
)

type (
	astNode         = ast.Node
	astComment      = ast.Comment
	astCommentGroup = ast.CommentGroup
)

// A CommentedNode bundles an AST node and corresponding comments.
// It may be provided as argument to any of the [Fprint] functions.
type CommentedNode struct {
	Node     any
	Comments []*ast.CommentGroup
}

// by XGo
type CommentedNodes struct {
	Node           any
	CommentedStmts map[js.Stmt]*ast.CommentGroup
}

// Fprint "pretty-prints" an AST node to output for a given configuration cfg.
// Position information is interpreted relative to the file set fset.
func (cfg *Config) Fprint(output io.Writer, fset *token.FileSet, node any) error {
	return cfg.fprint(output, fset, node, make(map[astNode]int))
}

// Fprint "pretty-prints" an AST node to output.
// It calls [Config.Fprint] with default settings.
func Fprint(output io.Writer, fset *token.FileSet, node any) error {
	return (&Config{Tabwidth: 8}).Fprint(output, fset, node)
}
