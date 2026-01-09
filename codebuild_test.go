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

package gogen

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"testing"
)

func TestCircularEmbeddedFieldLookup(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()

	typeA := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "A", nil), nil, nil)
	typeB := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "B", nil), nil, nil)

	// Creates a circular embedding relationship between type A and B.
	typeA.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "", typeB, true), // Embed B.
	}, nil))
	typeB.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "", typeA, true), // Embed A.
	}, nil))

	cb.stk.Push(&Element{Type: typeA})
	kind, _ := cb.Member("any", MemberFlagVal)
	if kind != MemberInvalid {
		t.Fatal("Member should return MemberInvalid for circular embedding")
	}
	kind, _ = cb.Member("any", MemberFlagRef)
	if kind != MemberInvalid {
		t.Fatal("Member should return MemberInvalid for circular embedding")
	}
}

func TestFindMember(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()

	// create interface
	// type Impl interface {
	//     step__0()
	//     step__1()
	// }
	fn0 := types.NewFunc(token.NoPos, pkg.Types, "step__0", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	fn1 := types.NewFunc(token.NoPos, pkg.Types, "step__1", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	iface := types.NewInterfaceType([]*types.Func{fn0, fn1}, []types.Type{})
	tnamed := types.NewTypeName(token.NoPos, pkg.Types, "Impl", nil)
	named := types.NewNamed(tnamed, iface, nil)
	cb.Scope().Insert(named.Obj())

	// then call init to process overloads etc in the package
	InitThisGopPkg(pkg.Types)

	// create an interface that has method foo and embeds Impl
	// type Impl2 interface {
	//     foo()
	//     Impl
	// }
	var methods []*types.Func
	var embeddeds []types.Type
	mthd := types.NewFunc(token.NoPos, pkg.Types, "foo", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	methods = append(methods, mthd)
	embeddeds = append(embeddeds, named)
	iface2 := types.NewInterfaceType(methods, embeddeds)
	tnamed2 := types.NewTypeName(token.NoPos, pkg.Types, "Impl2", nil)
	named2 := types.NewNamed(tnamed2, iface2, nil)

	// push an element whose type is the interface
	cb.stk.Push(&Element{Type: named2})

	kind, _ := cb.Member("step", MemberFlagVal)
	if kind != MemberMethod {
		t.Fatalf("expected MemberMethod (1), got %v", kind)
	}
}

func TestIsValidStmtExprWithParenExpr(t *testing.T) {
	// Test that parenthesized call expressions are valid: (foo())
	callExpr := &ast.CallExpr{Fun: ast.NewIdent("foo")}
	parenCall := &ast.ParenExpr{X: callExpr}
	elem := &Element{Val: parenCall}
	if !isValidStmtExpr(elem) {
		t.Error("(foo()) should be a valid statement expression")
	}

	// Test nested parentheses: ((foo()))
	nestedParen := &ast.ParenExpr{X: parenCall}
	elem2 := &Element{Val: nestedParen}
	if !isValidStmtExpr(elem2) {
		t.Error("((foo())) should be a valid statement expression")
	}

	// Test that parenthesized receive is valid: (<-ch)
	recvExpr := &ast.UnaryExpr{Op: token.ARROW, X: ast.NewIdent("ch")}
	parenRecv := &ast.ParenExpr{X: recvExpr}
	elem3 := &Element{Val: parenRecv}
	if !isValidStmtExpr(elem3) {
		t.Error("(<-ch) should be a valid statement expression")
	}

	// Test that parenthesized non-call/recv is still invalid: (a + b)
	binExpr := &ast.BinaryExpr{X: ast.NewIdent("a"), Op: token.ADD, Y: ast.NewIdent("b")}
	parenBin := &ast.ParenExpr{X: binExpr}
	elem4 := &Element{Val: parenBin}
	if isValidStmtExpr(elem4) {
		t.Error("(a + b) should NOT be a valid statement expression")
	}
}

func TestExprCode(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expr
		expected string
	}{
		{
			name:     "Ident",
			expr:     ast.NewIdent("foo"),
			expected: "foo",
		},
		{
			name:     "BasicLitInt",
			expr:     &ast.BasicLit{Kind: token.INT, Value: "42"},
			expected: "42",
		},
		{
			name:     "CallExprIdent",
			expr:     &ast.CallExpr{Fun: ast.NewIdent("foo")},
			expected: "foo(...)",
		},
		{
			name: "CallExprSelector",
			expr: &ast.CallExpr{
				Fun: &ast.SelectorExpr{X: ast.NewIdent("pkg"), Sel: ast.NewIdent("Func")},
			},
			expected: "pkg.Func(...)",
		},
		{
			name: "CallExprOther",
			expr: &ast.CallExpr{
				Fun: &ast.IndexExpr{X: ast.NewIdent("funcs"), Index: ast.NewIdent("i")},
			},
			expected: "funcs[i](...)",
		},
		{
			name: "CallExprNestedSelector",
			expr: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X:   ast.NewIdent("a"),
						Sel: ast.NewIdent("b"),
					},
					Sel: ast.NewIdent("c"),
				},
			},
			expected: "a.b.c(...)",
		},
		{
			name:     "SelectorExprIdent",
			expr:     &ast.SelectorExpr{X: ast.NewIdent("obj"), Sel: ast.NewIdent("Field")},
			expected: "obj.Field",
		},
		{
			name: "SelectorExprNested",
			expr: &ast.SelectorExpr{
				X:   &ast.CallExpr{Fun: ast.NewIdent("getObj")},
				Sel: ast.NewIdent("Field"),
			},
			expected: "getObj(...).Field",
		},
		{
			name: "SelectorExprDeepNested",
			expr: &ast.SelectorExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("a"),
					Sel: ast.NewIdent("b"),
				},
				Sel: ast.NewIdent("c"),
			},
			expected: "a.b.c",
		},
		{
			name:     "IndexExpr",
			expr:     &ast.IndexExpr{X: ast.NewIdent("arr"), Index: ast.NewIdent("i")},
			expected: "arr[i]",
		},
		{
			name:     "SliceExprIdent",
			expr:     &ast.SliceExpr{X: ast.NewIdent("arr")},
			expected: "arr[...]",
		},
		{
			name:     "SliceExprOther",
			expr:     &ast.SliceExpr{X: &ast.CallExpr{Fun: ast.NewIdent("getArr")}},
			expected: "getArr(...)[...]",
		},
		{
			name:     "BinaryExpr",
			expr:     &ast.BinaryExpr{X: ast.NewIdent("a"), Op: token.ADD, Y: ast.NewIdent("b")},
			expected: "a + b",
		},
		{
			name:     "UnaryExpr",
			expr:     &ast.UnaryExpr{Op: token.SUB, X: ast.NewIdent("x")},
			expected: "-x",
		},
		{
			name:     "TypeAssertExprWithType",
			expr:     &ast.TypeAssertExpr{X: ast.NewIdent("x"), Type: ast.NewIdent("int")},
			expected: "x.(int)",
		},
		{
			name:     "TypeAssertExprNilType",
			expr:     &ast.TypeAssertExpr{X: ast.NewIdent("x"), Type: nil},
			expected: "x.(type)",
		},
		{
			name:     "ParenExpr",
			expr:     &ast.ParenExpr{X: ast.NewIdent("x")},
			expected: "(x)",
		},
		{
			name:     "StarExpr",
			expr:     &ast.StarExpr{X: ast.NewIdent("ptr")},
			expected: "*ptr",
		},
		{
			name:     "CompositeLit",
			expr:     &ast.CompositeLit{Type: ast.NewIdent("T")},
			expected: "T{...}",
		},
		{
			name:     "Unknown",
			expr:     &ast.FuncLit{},
			expected: "expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exprCode(tt.expr)
			if got != tt.expected {
				t.Errorf("exprCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsAddressableParenExpr(t *testing.T) {
	// Test that parenthesized identifiers are addressable: (x)
	parenIdent := &ast.ParenExpr{X: ast.NewIdent("x")}
	if !isAddressable(parenIdent) {
		t.Error("(x) should be addressable")
	}

	// Test nested parentheses: ((x))
	nestedParen := &ast.ParenExpr{X: parenIdent}
	if !isAddressable(nestedParen) {
		t.Error("((x)) should be addressable")
	}

	// Test that parenthesized non-addressable is not addressable: (a + b)
	binExpr := &ast.BinaryExpr{X: ast.NewIdent("a"), Op: token.ADD, Y: ast.NewIdent("b")}
	parenBin := &ast.ParenExpr{X: binExpr}
	if isAddressable(parenBin) {
		t.Error("(a + b) should NOT be addressable")
	}
}

func TestExprDescriptionNilType(t *testing.T) {
	elem := &Element{Val: ast.NewIdent("x"), Type: nil}
	got := exprDescription(elem, "x")
	if got != "expression" {
		t.Errorf("exprDescription() with nil type = %q, want %q", got, "expression")
	}
}

func TestIsLiteralCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		cval     constant.Value
		typ      *types.Basic
		expected bool
	}{
		{
			name:     "EmptyCode",
			code:     "",
			cval:     constant.MakeInt64(42),
			typ:      types.Typ[types.UntypedInt],
			expected: false,
		},
		{
			name:     "UntypedRune",
			code:     "'a'",
			cval:     constant.MakeInt64(97),
			typ:      types.Typ[types.UntypedRune],
			expected: false,
		},
		{
			name:     "TypedInt",
			code:     "42",
			cval:     constant.MakeInt64(42),
			typ:      types.Typ[types.Int],
			expected: false,
		},
		{
			name:     "BigInt",
			code:     "99999999999999999999999999999999999999",
			cval:     constant.MakeFromLiteral("99999999999999999999999999999999999999", token.INT, 0),
			typ:      types.Typ[types.UntypedInt],
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLiteralCode(tt.code, tt.cval, tt.typ)
			if got != tt.expected {
				t.Errorf("isLiteralCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExprDescriptionTypedConstant(t *testing.T) {
	// Test typed constant: "constant V of type X"
	elem := &Element{
		Val:  ast.NewIdent("c"),
		Type: types.Typ[types.Int],
		CVal: constant.MakeInt64(42),
	}
	got := exprDescription(elem, "c")
	expected := "constant 42 of type int"
	if got != expected {
		t.Errorf("exprDescription() = %q, want %q", got, expected)
	}
}

func TestUnusedExprWithoutSource(t *testing.T) {
	// Test that unused expression error works without source position
	// This triggers the exprCode fallback path
	defer func() {
		if e := recover(); e != nil {
			err, ok := e.(*CodeError)
			if !ok {
				t.Fatalf("expected CodeError, got %T: %v", e, e)
			}
			// Verify the error message contains expected content
			errMsg := err.Error()
			if errMsg == "" {
				t.Error("expected non-empty error message")
			}
		} else {
			t.Fatal("expected error for unused expression")
		}
	}()
	pkg := NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar(types.Typ[types.Int], "a").
		VarVal("a").EndStmt(). // No source() parameter
		End()
}
