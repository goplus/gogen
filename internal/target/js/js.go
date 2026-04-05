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

package js

import (
	"go/token"
)

type Node any

type Object struct {
	Data any // object-specific data; or nil
}

type PkgRef struct {
	Name string  // package name
	Obj  *Object // denoted object, or nil.
}

// All expression nodes implement the Expr interface.
type Expr interface {
	Node
	exprNode()
}

type BasicLit struct {
	Kind  token.Token // token.INT, token.FLOAT, token.IMAG, token.CHAR, or token.STRING
	Value string      // literal string; e.g. 42, 0x7f, 3.14, 1e-9, 2.4i, 'a', '\x7f', "foo" or `\m\n\o`
}

func (p *BasicLit) exprNode() {}

// An Ident node represents an identifier.
type Ident struct {
	Name string  // identifier name
	Obj  *Object // denoted object, or nil.
}

func (p *Ident) exprNode() {}

// A UnaryExpr node represents a unary expression.
// Unary "*" expressions are represented via StarExpr nodes.
type UnaryExpr struct {
	Op token.Token // operator
	X  Expr        // operand
}

func (p *UnaryExpr) exprNode() {}

// A BinaryExpr node represents a binary expression.
type BinaryExpr struct {
	X  Expr        // left operand
	Op token.Token // operator
	Y  Expr        // right operand
}

func (p *BinaryExpr) exprNode() {}

// A SelectorExpr node represents an expression followed by a selector.
type SelectorExpr struct {
	X   Expr   // expression
	Sel *Ident // field selector
}

func (p *SelectorExpr) exprNode() {}

// A CallExpr node represents an expression followed by an argument list.
type CallExpr struct {
	Fun      Expr      // function expression
	Args     []Expr    // function arguments; or nil
	Ellipsis token.Pos // position of "..." (token.NoPos if there is no "...")
}

func (p *CallExpr) exprNode() {}

// A ParenExpr node represents a parenthesized expression.
type ParenExpr struct {
	X Expr // parenthesized expression
}

func (p *ParenExpr) exprNode() {}
