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

// ----------------------------------------------------------------------------

type Node interface {
	Pos() token.Pos
	End() token.Pos
}

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

// All statement nodes implement the Stmt interface.
type Stmt interface {
	Node
	stmtNode()
}

// ----------------------------------------------------------------------------

type BasicLit struct {
	ValuePos token.Pos   // literal position
	ValueEnd token.Pos   // position immediately after the literal
	Kind     token.Token // token.INT, token.FLOAT, token.IMAG, token.CHAR, or token.STRING
	Value    string      // literal string; e.g. 42, 0x7f, 3.14, 1e-9, 2.4i, 'a', '\x7f', "foo" or `\m\n\o`
}

func (x *BasicLit) Pos() token.Pos { return x.ValuePos }
func (x *BasicLit) End() token.Pos {
	if !x.ValueEnd.IsValid() {
		// Not from parser; use a heuristic.
		// (Incorrect for `...` containing \r\n;
		// see https://go.dev/issue/76031.)
		return token.Pos(int(x.ValuePos) + len(x.Value))
	}
	return x.ValueEnd
}
func (p *BasicLit) exprNode() {}

// An Ident node represents an identifier.
type Ident struct {
	NamePos token.Pos // identifier position
	Name    string    // identifier name
	Obj     *Object   // denoted object, or nil.
}

func (x *Ident) Pos() token.Pos { return x.NamePos }
func (x *Ident) End() token.Pos { return token.Pos(int(x.NamePos) + len(x.Name)) }
func (p *Ident) exprNode()      {}

// A UnaryExpr node represents a unary expression.
// Unary "*" expressions are represented via StarExpr nodes.
type UnaryExpr struct {
	OpPos token.Pos   // position of Op
	Op    token.Token // operator
	X     Expr        // operand
}

func (x *UnaryExpr) Pos() token.Pos { return x.OpPos }
func (x *UnaryExpr) End() token.Pos { return x.X.End() }
func (p *UnaryExpr) exprNode()      {}

// A BinaryExpr node represents a binary expression.
type BinaryExpr struct {
	X     Expr        // left operand
	OpPos token.Pos   // position of Op
	Op    token.Token // operator
	Y     Expr        // right operand
}

func (x *BinaryExpr) Pos() token.Pos { return x.X.Pos() }
func (x *BinaryExpr) End() token.Pos { return x.Y.End() }
func (p *BinaryExpr) exprNode()      {}

// A SelectorExpr node represents an expression followed by a selector.
type SelectorExpr struct {
	X   Expr   // expression
	Sel *Ident // field selector
}

func (x *SelectorExpr) Pos() token.Pos { return x.X.Pos() }
func (x *SelectorExpr) End() token.Pos { return x.Sel.End() }
func (p *SelectorExpr) exprNode()      {}

// A CallExpr node represents an expression followed by an argument list.
type CallExpr struct {
	Fun      Expr      // function expression
	Lparen   token.Pos // position of "("
	Args     []Expr    // function arguments; or nil
	Ellipsis token.Pos // position of "..." (token.NoPos if there is no "...")
	Rparen   token.Pos // position of ")"
}

func (x *CallExpr) Pos() token.Pos { return x.Fun.Pos() }
func (x *CallExpr) End() token.Pos { return x.Rparen + 1 }
func (p *CallExpr) exprNode()      {}

// A ParenExpr node represents a parenthesized expression.
type ParenExpr struct {
	Lparen token.Pos // position of "("
	X      Expr      // parenthesized expression
	Rparen token.Pos // position of ")"
}

func (x *ParenExpr) Pos() token.Pos { return x.Lparen }
func (x *ParenExpr) End() token.Pos { return x.Rparen + 1 }
func (p *ParenExpr) exprNode()      {}

/*
func (x *BadExpr) Pos() token.Pos  { return x.From }
func (x *Ellipsis) Pos() token.Pos { return x.Ellipsis }
func (x *FuncLit) Pos() token.Pos  { return x.Type.Pos() }
func (x *CompositeLit) Pos() token.Pos {
	if x.Type != nil {
		return x.Type.Pos()
	}
	return x.Lbrace
}
func (x *IndexExpr) Pos() token.Pos      { return x.X.Pos() }
func (x *IndexListExpr) Pos() token.Pos  { return x.X.Pos() }
func (x *SliceExpr) Pos() token.Pos      { return x.X.Pos() }
func (x *TypeAssertExpr) Pos() token.Pos { return x.X.Pos() }
func (x *StarExpr) Pos() token.Pos       { return x.Star }
func (x *KeyValueExpr) Pos() token.Pos   { return x.Key.Pos() }
func (x *ArrayType) Pos() token.Pos      { return x.Lbrack }
func (x *StructType) Pos() token.Pos     { return x.Struct }
func (x *FuncType) Pos() token.Pos {
	if x.Func.IsValid() || x.Params == nil { // see issue 3870
		return x.Func
	}
	return x.Params.Pos() // interface method declarations have no "func" keyword
}
func (x *InterfaceType) Pos() token.Pos { return x.Interface }
func (x *MapType) Pos() token.Pos       { return x.Map }
func (x *ChanType) Pos() token.Pos      { return x.Begin }

func (x *BadExpr) End() token.Pos { return x.To }
func (x *Ellipsis) End() token.Pos {
	if x.Elt != nil {
		return x.Elt.End()
	}
	return x.Ellipsis + 3 // len("...")
}
func (x *FuncLit) End() token.Pos        { return x.Body.End() }
func (x *CompositeLit) End() token.Pos   { return x.Rbrace + 1 }
func (x *IndexExpr) End() token.Pos      { return x.Rbrack + 1 }
func (x *IndexListExpr) End() token.Pos  { return x.Rbrack + 1 }
func (x *SliceExpr) End() token.Pos      { return x.Rbrack + 1 }
func (x *TypeAssertExpr) End() token.Pos { return x.Rparen + 1 }
func (x *StarExpr) End() token.Pos       { return x.X.End() }
func (x *KeyValueExpr) End() token.Pos   { return x.Value.End() }
func (x *ArrayType) End() token.Pos      { return x.Elt.End() }
func (x *StructType) End() token.Pos     { return x.Fields.End() }
func (x *FuncType) End() token.Pos {
	if x.Results != nil {
		return x.Results.End()
	}
	return x.Params.End()
}
func (x *InterfaceType) End() token.Pos { return x.Methods.End() }
func (x *MapType) End() token.Pos       { return x.Value.End() }
func (x *ChanType) End() token.Pos      { return x.Value.End() }
*/

// ----------------------------------------------------------------------------

// An EmptyStmt node represents an empty statement.
// The "position" of the empty statement is the position
// of the immediately following (explicit or implicit) semicolon.
type EmptyStmt struct {
	Semicolon token.Pos // position of following ";"
	Implicit  bool      // if set, ";" was omitted in the source
}

func (s *EmptyStmt) Pos() token.Pos { return s.Semicolon }
func (s *EmptyStmt) End() token.Pos {
	if s.Implicit {
		return s.Semicolon
	}
	return s.Semicolon + 1 /* len(";") */
}
func (*EmptyStmt) stmtNode() {}

// A BlockStmt node represents a braced statement list.
type BlockStmt struct {
	Lbrace token.Pos // position of "{"
	List   []Stmt
	Rbrace token.Pos // position of "}", if any (may be absent due to syntax error)
}

func (s *BlockStmt) Pos() token.Pos { return s.Lbrace }
func (s *BlockStmt) End() token.Pos {
	if s.Rbrace.IsValid() {
		return s.Rbrace + 1
	}
	if n := len(s.List); n > 0 {
		return s.List[n-1].End()
	}
	return s.Lbrace + 1
}
func (*BlockStmt) stmtNode() {}

// An IfStmt node represents an if statement.
type IfStmt struct {
	If   token.Pos // position of "if" keyword
	Cond Expr      // condition
	Body *BlockStmt
	Else Stmt // else branch; or nil
}

func (s *IfStmt) Pos() token.Pos { return s.If }
func (s *IfStmt) End() token.Pos {
	if s.Else != nil {
		return s.Else.End()
	}
	return s.Body.End()
}
func (*IfStmt) stmtNode() {}

// A ForStmt represents a for statement.
type ForStmt struct {
	For  token.Pos // position of "for" keyword
	Init Stmt      // initialization statement; or nil
	Cond Expr      // condition; or nil
	Post Stmt      // post iteration statement; or nil
	Body *BlockStmt
}

func (s *ForStmt) Pos() token.Pos { return s.For }
func (s *ForStmt) End() token.Pos { return s.Body.End() }
func (*ForStmt) stmtNode()        {}

// ----------------------------------------------------------------------------
