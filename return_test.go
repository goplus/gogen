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
	"testing"
)

func TestTermCheckerIsTerminating(t *testing.T) {
	var c termChecker

	// return
	if !c.isTerminating(&ast.ReturnStmt{}, "") {
		t.Error("return should be terminating")
	}

	// goto
	if !c.isTerminating(&ast.BranchStmt{Tok: token.GOTO, Label: &ast.Ident{Name: "label"}}, "") {
		t.Error("goto should be terminating")
	}

	// fallthrough
	if !c.isTerminating(&ast.BranchStmt{Tok: token.FALLTHROUGH}, "") {
		t.Error("fallthrough should be terminating")
	}

	// break is not terminating
	if c.isTerminating(&ast.BranchStmt{Tok: token.BREAK}, "") {
		t.Error("break should not be terminating")
	}

	// continue is not terminating
	if c.isTerminating(&ast.BranchStmt{Tok: token.CONTINUE}, "") {
		t.Error("continue should not be terminating")
	}

	// labeled return
	if !c.isTerminating(&ast.LabeledStmt{Label: &ast.Ident{Name: "done"}, Stmt: &ast.ReturnStmt{}}, "") {
		t.Error("labeled return should be terminating")
	}

	// labeled empty statement
	if c.isTerminating(&ast.LabeledStmt{Label: &ast.Ident{Name: "done"}, Stmt: &ast.EmptyStmt{}}, "") {
		t.Error("labeled empty statement should not be terminating")
	}

	// infinite for without break
	infiniteFor := &ast.ForStmt{
		Cond: nil,
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: &ast.Ident{Name: "x"}}}},
	}
	if !c.isTerminating(infiniteFor, "") {
		t.Error("infinite for without break should be terminating")
	}

	// infinite for with break
	infiniteForWithBreak := &ast.ForStmt{
		Cond: nil,
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	}
	if c.isTerminating(infiniteForWithBreak, "") {
		t.Error("infinite for with break should not be terminating")
	}

	// infinite for with labeled break to outer label
	infiniteForWithLabeledBreak := &ast.ForStmt{
		Cond: nil,
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}},
	}
	if !c.isTerminating(infiniteForWithLabeledBreak, "") {
		t.Error("infinite for with break to different label should be terminating")
	}

	// select with all cases terminating
	selectStmt := &ast.SelectStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.CommClause{Comm: &ast.SendStmt{}, Body: []ast.Stmt{&ast.ReturnStmt{}}},
				&ast.CommClause{Comm: nil, Body: []ast.Stmt{&ast.ReturnStmt{}}},
			},
		},
	}
	if !c.isTerminating(selectStmt, "") {
		t.Error("select with all cases terminating should be terminating")
	}

	// select with one case not terminating
	selectNotTerm := &ast.SelectStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.CommClause{Comm: &ast.SendStmt{}, Body: []ast.Stmt{&ast.ReturnStmt{}}},
				&ast.CommClause{Comm: nil, Body: []ast.Stmt{&ast.EmptyStmt{}}},
			},
		},
	}
	if c.isTerminating(selectNotTerm, "") {
		t.Error("select with non-terminating case should not be terminating")
	}

	// empty select
	if !c.isTerminating(&ast.SelectStmt{Body: &ast.BlockStmt{List: []ast.Stmt{}}}, "") {
		t.Error("empty select should be terminating")
	}

	// switch with break referring to outer label
	switchWithLabeledBreak := &ast.SwitchStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.CaseClause{
					List: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
					Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}},
				},
				&ast.CaseClause{List: nil, Body: []ast.Stmt{&ast.ReturnStmt{}}},
			},
		},
	}
	if c.isTerminating(switchWithLabeledBreak, "outer") {
		t.Error("switch with break to outer label should not be terminating")
	}

	// type switch with default and all cases terminating
	typeSwitchTerm := &ast.TypeSwitchStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.CaseClause{List: []ast.Expr{&ast.Ident{Name: "int"}}, Body: []ast.Stmt{&ast.ReturnStmt{}}},
				&ast.CaseClause{List: nil, Body: []ast.Stmt{&ast.ReturnStmt{}}},
			},
		},
	}
	if !c.isTerminating(typeSwitchTerm, "") {
		t.Error("type switch with default and all cases terminating should be terminating")
	}

	// type switch without default
	typeSwitchNoDefault := &ast.TypeSwitchStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.CaseClause{List: []ast.Expr{&ast.Ident{Name: "int"}}, Body: []ast.Stmt{&ast.ReturnStmt{}}},
			},
		},
	}
	if c.isTerminating(typeSwitchNoDefault, "") {
		t.Error("type switch without default should not be terminating")
	}

	// various non-terminating statements
	stmts := []ast.Stmt{
		nil,
		&ast.EmptyStmt{},
		&ast.DeclStmt{},
		&ast.SendStmt{},
		&ast.IncDecStmt{},
		&ast.AssignStmt{},
		&ast.GoStmt{},
		&ast.DeferStmt{},
		&ast.RangeStmt{},
	}
	for _, stmt := range stmts {
		if c.isTerminating(stmt, "") {
			t.Errorf("%T should not be terminating", stmt)
		}
	}
}

func TestTermCheckerIsTerminatingList(t *testing.T) {
	var c termChecker

	// empty list
	if c.isTerminatingList(nil, "") {
		t.Error("empty list should not be terminating")
	}

	// list with only empty statements
	if c.isTerminatingList([]ast.Stmt{&ast.EmptyStmt{}, &ast.EmptyStmt{}}, "") {
		t.Error("list with only empty statements should not be terminating")
	}

	// list ending with return
	if !c.isTerminatingList([]ast.Stmt{&ast.EmptyStmt{}, &ast.ReturnStmt{}}, "") {
		t.Error("list ending with return should be terminating")
	}

	// list ending with empty but has return before
	if !c.isTerminatingList([]ast.Stmt{&ast.ReturnStmt{}, &ast.EmptyStmt{}}, "") {
		t.Error("list with return before trailing empty should be terminating")
	}
}

func TestTermCheckerIsPanicCall(t *testing.T) {
	var c termChecker

	// non-call expression
	if c.isPanicCall(&ast.Ident{Name: "x"}) {
		t.Error("non-call expression should not be panic call")
	}

	// call not in panicCalls map
	call := &ast.CallExpr{Fun: &ast.Ident{Name: "panic"}}
	if c.isPanicCall(call) {
		t.Error("call not tracked in panicCalls should not be panic call")
	}

	// call in panicCalls map
	c.panicCalls = map[*ast.CallExpr]none{call: {}}
	if !c.isPanicCall(call) {
		t.Error("call tracked in panicCalls should be panic call")
	}

	// parenthesized call in panicCalls map
	if !c.isPanicCall(&ast.ParenExpr{X: call}) {
		t.Error("parenthesized call tracked in panicCalls should be panic call")
	}
}

func TestHasBreak(t *testing.T) {
	// break without label
	breakStmt := &ast.BranchStmt{Tok: token.BREAK}
	if !hasBreak(breakStmt, "", true) {
		t.Error("break without label should match implicit")
	}
	if hasBreak(breakStmt, "", false) {
		t.Error("break without label should not match when implicit=false")
	}

	// break with label
	breakWithLabel := &ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}
	if !hasBreak(breakWithLabel, "outer", false) {
		t.Error("break with label should match the label")
	}
	if hasBreak(breakWithLabel, "other", false) {
		t.Error("break with label should not match different label")
	}

	// continue is not break
	if hasBreak(&ast.BranchStmt{Tok: token.CONTINUE}, "", true) {
		t.Error("continue should not be treated as break")
	}

	// block with break
	block := &ast.BlockStmt{List: []ast.Stmt{&ast.EmptyStmt{}, &ast.BranchStmt{Tok: token.BREAK}}}
	if !hasBreak(block, "", true) {
		t.Error("block with break should have break")
	}

	// if with break in body
	ifWithBreak := &ast.IfStmt{Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}
	if !hasBreak(ifWithBreak, "", true) {
		t.Error("if with break in body should have break")
	}

	// if with break in else
	ifWithBreakInElse := &ast.IfStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.EmptyStmt{}}},
		Else: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}},
	}
	if !hasBreak(ifWithBreakInElse, "", true) {
		t.Error("if with break in else should have break")
	}

	// if without break
	if hasBreak(&ast.IfStmt{Body: &ast.BlockStmt{List: []ast.Stmt{&ast.EmptyStmt{}}}}, "", true) {
		t.Error("if without break should not have break")
	}

	// case clause with break
	if !hasBreak(&ast.CaseClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}, "", true) {
		t.Error("case clause with break should have break")
	}

	// comm clause with break
	if !hasBreak(&ast.CommClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}, "", true) {
		t.Error("comm clause with break should have break")
	}

	// switch does not propagate implicit break
	switchStmt := &ast.SwitchStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.CaseClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}},
	}
	if hasBreak(switchStmt, "", true) {
		t.Error("switch should not propagate implicit break")
	}

	// switch with labeled break
	switchWithLabeledBreak := &ast.SwitchStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{&ast.CaseClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}}},
		},
	}
	if !hasBreak(switchWithLabeledBreak, "outer", false) {
		t.Error("switch with labeled break should propagate labeled break")
	}

	// type switch does not propagate implicit break
	typeSwitchStmt := &ast.TypeSwitchStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.CaseClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}},
	}
	if hasBreak(typeSwitchStmt, "", true) {
		t.Error("type switch should not propagate implicit break")
	}

	// type switch with labeled break
	typeSwitchWithLabeledBreak := &ast.TypeSwitchStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{&ast.CaseClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}}},
		},
	}
	if !hasBreak(typeSwitchWithLabeledBreak, "outer", false) {
		t.Error("type switch with labeled break should propagate labeled break")
	}

	// select does not propagate implicit break
	selectStmt := &ast.SelectStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.CommClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}},
	}
	if hasBreak(selectStmt, "", true) {
		t.Error("select should not propagate implicit break")
	}

	// select with labeled break
	selectWithLabeledBreak := &ast.SelectStmt{
		Body: &ast.BlockStmt{
			List: []ast.Stmt{&ast.CommClause{Body: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}}},
		},
	}
	if !hasBreak(selectWithLabeledBreak, "outer", false) {
		t.Error("select with labeled break should propagate labeled break")
	}

	// for does not propagate implicit break
	forStmt := &ast.ForStmt{Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}
	if hasBreak(forStmt, "", true) {
		t.Error("for should not propagate implicit break")
	}

	// for with labeled break
	forWithLabeledBreak := &ast.ForStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}},
	}
	if !hasBreak(forWithLabeledBreak, "outer", false) {
		t.Error("for with labeled break should propagate labeled break")
	}

	// range does not propagate implicit break
	rangeStmt := &ast.RangeStmt{Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK}}}}
	if hasBreak(rangeStmt, "", true) {
		t.Error("range should not propagate implicit break")
	}

	// range with labeled break
	rangeWithLabeledBreak := &ast.RangeStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.BranchStmt{Tok: token.BREAK, Label: &ast.Ident{Name: "outer"}}}},
	}
	if !hasBreak(rangeWithLabeledBreak, "outer", false) {
		t.Error("range with labeled break should propagate labeled break")
	}

	// labeled statement with break
	labeled := &ast.LabeledStmt{Label: &ast.Ident{Name: "inner"}, Stmt: &ast.BranchStmt{Tok: token.BREAK}}
	if !hasBreak(labeled, "", true) {
		t.Error("labeled statement with break should have break")
	}
}

func TestUnparen(t *testing.T) {
	ident := &ast.Ident{Name: "x"}

	// no parentheses
	if unparen(ident) != ident {
		t.Error("unparen should return same expr when no parentheses")
	}

	// single level
	paren1 := &ast.ParenExpr{X: ident}
	if unparen(paren1) != ident {
		t.Error("unparen should remove single parentheses")
	}

	// multiple levels
	paren2 := &ast.ParenExpr{X: paren1}
	if unparen(paren2) != ident {
		t.Error("unparen should remove all parentheses")
	}
}
