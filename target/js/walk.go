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
	"fmt"
	"iter"
)

// A Visitor's Visit method is invoked for each node encountered by [Walk].
// If the result visitor w is not nil, [Walk] visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

func walkList[N Node](v Visitor, list []N) {
	for _, node := range list {
		Walk(v, node)
	}
}

// TODO(gri): Investigate if providing a closure to Walk leads to
// simpler use (and may help eliminate Inspect in turn).

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// walk children
	switch n := node.(type) {
	// Comments and fields
	case *Comment:
		// nothing to do

	case *CommentGroup:
		walkList(v, n.List)

	/* TODO(xsw):
	case *Field:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		walkList(v, n.Names)
		if n.Type != nil {
			Walk(v, n.Type)
		}
		if n.Tag != nil {
			Walk(v, n.Tag)
		}
		if n.Comment != nil {
			Walk(v, n.Comment)
		}

	case *FieldList:
		walkList(v, n.List)
	*/

	// Expressions
	case *Ident, *BasicLit:
		// nothing to do

	/* TODO(xsw):
	case *BadExpr:
	case *Ellipsis:
		if n.Elt != nil {
			Walk(v, n.Elt)
		}
	*/
	case *FuncLit:
		walkList(v, n.Params)
		Walk(v, n.Body)

	case *ArrayLit:
		walkList(v, n.Values)

	/* TODO(xsw):
	case *CompositeLit:
		if n.Type != nil {
			Walk(v, n.Type)
		}
		walkList(v, n.Elts)
	*/
	case *ParenExpr:
		Walk(v, n.X)

	case *SelectorExpr:
		Walk(v, n.X)
		Walk(v, n.Sel)

	case *IndexExpr:
		Walk(v, n.X)
		Walk(v, n.Index)

	/* TODO(xsw):
	case *IndexListExpr:
		Walk(v, n.X)
		walkList(v, n.Indices)

	case *SliceExpr:
		Walk(v, n.X)
		if n.Low != nil {
			Walk(v, n.Low)
		}
		if n.High != nil {
			Walk(v, n.High)
		}
		if n.Max != nil {
			Walk(v, n.Max)
		}

	case *TypeAssertExpr:
		Walk(v, n.X)
		if n.Type != nil {
			Walk(v, n.Type)
		}
	*/
	case *CallExpr:
		Walk(v, n.Fun)
		walkList(v, n.Args)

	/* TODO(xsw):
	case *StarExpr:
		Walk(v, n.X)
	*/
	case *UnaryExpr:
		Walk(v, n.X)

	case *BinaryExpr:
		Walk(v, n.X)
		Walk(v, n.Y)

	/* TODO(xsw):
	case *KeyValueExpr:
		Walk(v, n.Key)
		Walk(v, n.Value)

	// Types
	case *ArrayType:
		if n.Len != nil {
			Walk(v, n.Len)
		}
		Walk(v, n.Elt)

	case *StructType:
		Walk(v, n.Fields)

	case *FuncType:
		if n.TypeParams != nil {
			Walk(v, n.TypeParams)
		}
		if n.Params != nil {
			Walk(v, n.Params)
		}
		if n.Results != nil {
			Walk(v, n.Results)
		}

	case *InterfaceType:
		Walk(v, n.Methods)

	case *MapType:
		Walk(v, n.Key)
		Walk(v, n.Value)

	case *ChanType:
		Walk(v, n.Value)

	// Statements
	case *BadStmt:
		// nothing to do

	case *DeclStmt:
		Walk(v, n.Decl)
	*/

	case *EmptyStmt:
		// nothing to do

	case *LabeledStmt:
		Walk(v, n.Label)
		Walk(v, n.Stmt)

	case *ExprStmt:
		Walk(v, n.X)

	/* TODO(xsw):
	case *SendStmt:
		Walk(v, n.Chan)
		Walk(v, n.Value)

	case *IncDecStmt:
		Walk(v, n.X)

	case *AssignStmt:
		walkList(v, n.Lhs)
		walkList(v, n.Rhs)

	case *GoStmt:
		Walk(v, n.Call)

	case *DeferStmt:
		Walk(v, n.Call)
	*/
	case *ReturnStmt:
		Walk(v, n.Result)

	case *BranchStmt:
		if n.Label != nil {
			Walk(v, n.Label)
		}

	case *BlockStmt:
		walkList(v, n.List)

	case *IfStmt:
		Walk(v, n.Cond)
		Walk(v, n.Body)
		if n.Else != nil {
			Walk(v, n.Else)
		}

	/* TODO(xsw):
	case *CaseClause:
		walkList(v, n.List)
		walkList(v, n.Body)

	case *SwitchStmt:
		if n.Init != nil {
			Walk(v, n.Init)
		}
		if n.Tag != nil {
			Walk(v, n.Tag)
		}
		Walk(v, n.Body)

	case *TypeSwitchStmt:
		if n.Init != nil {
			Walk(v, n.Init)
		}
		Walk(v, n.Assign)
		Walk(v, n.Body)

	case *CommClause:
		if n.Comm != nil {
			Walk(v, n.Comm)
		}
		walkList(v, n.Body)

	case *SelectStmt:
		Walk(v, n.Body)
	*/
	case *ForStmt:
		if n.Init != nil {
			Walk(v, n.Init)
		}
		if n.Cond != nil {
			Walk(v, n.Cond)
		}
		if n.Post != nil {
			Walk(v, n.Post)
		}
		Walk(v, n.Body)

	/* TODO(xsw):
	case *RangeStmt:
		if n.Key != nil {
			Walk(v, n.Key)
		}
		if n.Value != nil {
			Walk(v, n.Value)
		}
		Walk(v, n.X)
		Walk(v, n.Body)

	// Declarations
	case *ValueSpec:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		walkList(v, n.Names)
		if n.Type != nil {
			Walk(v, n.Type)
		}
		walkList(v, n.Values)
		if n.Comment != nil {
			Walk(v, n.Comment)
		}

	case *TypeSpec:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		Walk(v, n.Name)
		if n.TypeParams != nil {
			Walk(v, n.TypeParams)
		}
		Walk(v, n.Type)
		if n.Comment != nil {
			Walk(v, n.Comment)
		}

	case *BadDecl:
		// nothing to do

	case *GenDecl:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		walkList(v, n.Specs)
	*/
	case *FuncDecl:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		//if n.Recv != nil {
		//	Walk(v, n.Recv)
		//}
		Walk(v, n.Name)
		walkList(v, n.Params)
		Walk(v, n.Body)

	case *ImportSpec:
		Walk(v, n.Name)
		if n.Alias != nil {
			Walk(v, n.Alias)
		}

	case *ImportDecl:
		walkList(v, n.Specs)
		Walk(v, n.Path)

	default:
		panic(fmt.Sprintf("js.Walk: unexpected node type %T", n))
	}

	v.Visit(nil)
}

type inspector func(Node) bool

func (f inspector) Visit(node Node) Visitor {
	if f(node) {
		return f
	}
	return nil
}

// Inspect traverses an AST in depth-first order: It starts by calling
// f(node); node must not be nil. If f returns true, Inspect invokes f
// recursively for each of the non-nil children of node, followed by a
// call of f(nil).
//
// In many cases it may be more convenient to use [Preorder], which
// returns an iterator over the sequence of nodes, or [PreorderStack],
// which (like [Inspect]) provides control over descent into subtrees,
// but additionally reports the stack of enclosing nodes.
func Inspect(node Node, f func(Node) bool) {
	Walk(inspector(f), node)
}

// Preorder returns an iterator over all the nodes of the syntax tree
// beneath (and including) the specified root, in depth-first
// preorder.
//
// For greater control over the traversal of each subtree, use
// [Inspect] or [PreorderStack].
func Preorder(root Node) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		ok := true
		Inspect(root, func(n Node) bool {
			if n != nil {
				// yield must not be called once ok is false.
				ok = ok && yield(n)
			}
			return ok
		})
	}
}

// PreorderStack traverses the tree rooted at root,
// calling f before visiting each node.
//
// Each call to f provides the current node and traversal stack,
// consisting of the original value of stack appended with all nodes
// from root to n, excluding n itself. (This design allows calls
// to PreorderStack to be nested without double counting.)
//
// If f returns false, the traversal skips over that subtree. Unlike
// [Inspect], no second call to f is made after visiting node n.
// (In practice, the second call is nearly always used only to pop the
// stack, and it is surprisingly tricky to do this correctly.)
func PreorderStack(root Node, stack []Node, f func(n Node, stack []Node) bool) {
	before := len(stack)
	Inspect(root, func(n Node) bool {
		if n != nil {
			if !f(n, stack) {
				// Do not push, as there will be no corresponding pop.
				return false
			}
			stack = append(stack, n) // push
		} else {
			stack = stack[:len(stack)-1] // pop
		}
		return true
	})
	if len(stack) != before {
		panic("push/pop mismatch")
	}
}
