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

package gogen

import (
	"go/ast"
	"go/token"
)

// termChecker checks whether statements are terminating.
//
// Based on go/types/return.go from the Go standard library, see
// https://github.com/golang/go/blob/927c89bbc5cc7366e86ecbb0f77267435b1d6d2c/src/go/types/return.go#L14-L17.
type termChecker struct {
	panicCalls map[*ast.CallExpr]none
}

// isTerminating reports whether s is a terminating statement.
func (c *termChecker) isTerminating(s ast.Stmt, label string) bool {
	switch s := s.(type) {
	case *ast.BadStmt, *ast.DeclStmt, *ast.EmptyStmt, *ast.SendStmt,
		*ast.IncDecStmt, *ast.AssignStmt, *ast.GoStmt, *ast.DeferStmt,
		*ast.RangeStmt:
		return false
	case *ast.LabeledStmt:
		return c.isTerminating(s.Stmt, s.Label.Name)
	case *ast.ExprStmt:
		return c.isPanicCall(s.X)
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.GOTO || s.Tok == token.FALLTHROUGH
	case *ast.BlockStmt:
		return c.isTerminatingList(s.List, "")
	case *ast.IfStmt:
		return s.Else != nil &&
			c.isTerminating(s.Body, "") &&
			c.isTerminating(s.Else, "")
	case *ast.SwitchStmt:
		return c.isTerminatingSwitch(s.Body, label)
	case *ast.TypeSwitchStmt:
		return c.isTerminatingSwitch(s.Body, label)
	case *ast.SelectStmt:
		for _, cc := range s.Body.List {
			body := cc.(*ast.CommClause).Body
			if !c.isTerminatingList(body, "") || hasBreakList(body, label, true) {
				return false
			}
		}
		return true
	case *ast.ForStmt:
		return s.Cond == nil && !hasBreak(s.Body, label, true)
	}
	return false
}

// isTerminatingList reports whether the last non-empty statement in list is terminating.
func (c *termChecker) isTerminatingList(list []ast.Stmt, label string) bool {
	// Trailing empty statements are permitted, skip them.
	for i := len(list) - 1; i >= 0; i-- {
		if _, ok := list[i].(*ast.EmptyStmt); !ok {
			return c.isTerminating(list[i], label)
		}
	}
	return false
}

// isTerminatingSwitch reports whether body is a terminating switch body.
func (c *termChecker) isTerminatingSwitch(body *ast.BlockStmt, label string) bool {
	hasDefault := false
	for _, cc := range body.List {
		cc := cc.(*ast.CaseClause)
		if cc.List == nil {
			hasDefault = true
		}
		if !c.isTerminatingList(cc.Body, "") || hasBreakList(cc.Body, label, true) {
			return false
		}
	}
	return hasDefault
}

// isPanicCall reports whether x is a call to the builtin panic function.
// It checks the panicCalls map which was populated during code generation,
// ensuring that shadowed panic calls are not incorrectly identified.
func (c *termChecker) isPanicCall(x ast.Expr) bool {
	call, ok := unparen(x).(*ast.CallExpr)
	if !ok {
		return false
	}
	_, ok = c.panicCalls[call]
	return ok
}

// hasBreak reports whether s contains a break statement referring to the label
// or (if isTarget) an unlabeled break.
func hasBreak(s ast.Stmt, label string, isTarget bool) bool {
	switch s := s.(type) {
	case *ast.BranchStmt:
		if s.Tok == token.BREAK {
			if s.Label == nil {
				return isTarget
			}
			return s.Label.Name == label
		}
	case *ast.BlockStmt:
		return hasBreakList(s.List, label, isTarget)
	case *ast.IfStmt:
		return hasBreak(s.Body, label, isTarget) || (s.Else != nil && hasBreak(s.Else, label, isTarget))
	case *ast.SwitchStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	case *ast.TypeSwitchStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	case *ast.SelectStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	case *ast.ForStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	case *ast.RangeStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	case *ast.LabeledStmt:
		return hasBreak(s.Stmt, label, isTarget)
	case *ast.CaseClause:
		return hasBreakList(s.Body, label, isTarget)
	case *ast.CommClause:
		return hasBreakList(s.Body, label, isTarget)
	}
	return false
}

// hasBreakList reports whether any statement in list contains a qualifying break.
func hasBreakList(list []ast.Stmt, label string, isTarget bool) bool {
	for _, s := range list {
		if hasBreak(s, label, isTarget) {
			return true
		}
	}
	return false
}

// unparen returns the expression with any enclosing parentheses removed.
func unparen(x ast.Expr) ast.Expr {
	for {
		p, ok := x.(*ast.ParenExpr)
		if !ok {
			return x
		}
		x = p.X
	}
}
