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
	Name string // declared name
	Data any    // object-specific data; or nil
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

// A File node represents a JavaScript source file.
type File struct {
	Doc      *CommentGroup   // associated documentation; or nil
	Stmts    []Stmt          // top-level statements; or nil
	Comments []*CommentGroup // comments in the file, in lexical order
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

// An ArrayLit node represents an array literal.
type ArrayLit struct {
	Lbrack token.Pos // position of "["
	Values []Expr    // array elements; or nil
	Rbrack token.Pos // position of "]"
}

func (x *ArrayLit) Pos() token.Pos { return x.Lbrack }
func (x *ArrayLit) End() token.Pos { return x.Rbrack + 1 }
func (*ArrayLit) exprNode()        {}

// A FuncLit node represents a function literal.
type FuncLit struct {
	Opening token.Pos  // position of (
	Params  []*Ident   // parameters
	Closing token.Pos  // position of )
	Body    *BlockStmt // function body
}

func (x *FuncLit) Pos() token.Pos { return x.Opening }
func (x *FuncLit) End() token.Pos { return x.Body.End() }
func (*FuncLit) exprNode()        {}

// An IndexExpr node represents an expression followed by an index.
type IndexExpr struct {
	X      Expr      // expression
	Lbrack token.Pos // position of "["
	Index  Expr      // index expression
	Rbrack token.Pos // position of "]"
}

func (x *IndexExpr) Pos() token.Pos { return x.X.Pos() }
func (x *IndexExpr) End() token.Pos { return x.Rbrack + 1 }
func (*IndexExpr) exprNode()        {}

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

// A LabeledStmt node represents a labeled statement.
type LabeledStmt struct {
	Label *Ident
	Colon token.Pos // position of ":"
	Stmt  Stmt
}

func (s *LabeledStmt) Pos() token.Pos { return s.Label.Pos() }
func (s *LabeledStmt) End() token.Pos { return s.Stmt.End() }
func (*LabeledStmt) stmtNode()        {}

// An ExprStmt node represents a (stand-alone) expression
// in a statement list.
type ExprStmt struct {
	X Expr // expression
}

func (s *ExprStmt) Pos() token.Pos { return s.X.Pos() }
func (s *ExprStmt) End() token.Pos { return s.X.End() }
func (*ExprStmt) stmtNode()        {}

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

// A BranchStmt node represents a break or continue statement.
type BranchStmt struct {
	TokPos token.Pos   // position of Tok
	Tok    token.Token // keyword token (BREAK, CONTINUE)
	Label  *Ident      // label name; or nil
}

func (s *BranchStmt) Pos() token.Pos { return s.TokPos }
func (s *BranchStmt) End() token.Pos {
	if s.Label != nil {
		return s.Label.End()
	}
	return token.Pos(int(s.TokPos) + len(s.Tok.String()))
}
func (*BranchStmt) stmtNode() {}

// A ReturnStmt node represents a return statement.
type ReturnStmt struct {
	Return token.Pos // position of "return" keyword
	Result Expr      // result expression; or nil
}

func (s *ReturnStmt) Pos() token.Pos { return s.Return }
func (s *ReturnStmt) End() token.Pos {
	if s.Result != nil {
		return s.Result.End()
	}
	return s.Return + 6 // len("return")
}
func (*ReturnStmt) stmtNode() {}

// ----------------------------------------------------------------------------

// A FuncDecl node represents a function declaration.
//
// function Name(params) { body }
// Recv.prototype.Name = function(params) { body }
type FuncDecl struct {
	Doc     *CommentGroup // associated documentation; or nil
	Recv    *Ident        // receiver (methods); or nil (functions)
	Func    token.Pos     // position of "function" keyword
	Name    *Ident        // function/method name
	Opening token.Pos     // position of (
	Params  []*Ident      // parameters
	Closing token.Pos     // position of )
	Body    *BlockStmt    // function body
}

func (f *FuncDecl) Pos() token.Pos {
	if f.Recv != nil {
		return f.Recv.Pos()
	}
	return f.Func
}
func (f *FuncDecl) End() token.Pos {
	return f.Body.End()
}
func (*FuncDecl) stmtNode() {}

// ----------------------------------------------------------------------------
