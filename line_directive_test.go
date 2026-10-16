//go:build !genjs

/*
Copyright 2021 The XGo Authors (xgo.dev)
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

package gogen_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestStatementLineDirectives(t *testing.T) {
	pkg := newMainPackage()
	cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg)
	cb.NewVar(types.Typ[types.Int], "a").
		SetComments(comment("\n//line source.spx:10:1"), true).
		VarRef("a").IncDec(token.INC).EndStmt().
		If().Val(true).Then().
		SetComments(comment("\n//line source.spx:20:1"), true).
		VarRef("a").IncDec(token.INC).EndStmt().End().
		SetComments(comment("\n// ordinary comment"), true).
		VarRef("a").IncDec(token.DEC).EndStmt().End()

	var out bytes.Buffer
	if err := pkg.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	source := out.String()
	for _, directive := range []string{"//line source.spx:10:1", "//line source.spx:20:1"} {
		if !strings.Contains(source, "\n"+directive+"\n") {
			t.Errorf("line directive must start at column 1: %q\n%s", directive, source)
		}
	}
	if !strings.Contains(source, "\t\ta++") || !strings.Contains(source, "\t// ordinary comment\n\ta--") {
		t.Errorf("statement and ordinary comment indentation was not preserved:\n%s", source)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var positions []token.Position
	ast.Inspect(file, func(node ast.Node) bool {
		if stmt, ok := node.(*ast.IncDecStmt); ok && stmt.Tok == token.INC {
			positions = append(positions, fset.Position(stmt.Pos()))
		}
		return true
	})
	if len(positions) != 2 {
		t.Fatalf("got %d increment statements, want 2", len(positions))
	}
	// Go positions count each tab as one column: one or two tabs put the statements at columns 2 and 3.
	for i, wantLine := range []int{10, 20} {
		pos := positions[i]
		if pos.Filename != "source.spx" || pos.Line != wantLine || pos.Column != i+2 {
			t.Errorf("statement %d position = %v, want source.spx:%d:%d", i, pos, wantLine, i+2)
		}
	}
}
