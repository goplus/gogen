//go:build !genjs

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
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"go/version"
	"runtime"
	"strings"
	"testing"
)

func checkGeneratedScope(t *testing.T, pkg *Package, existing ...string) {
	t.Helper()
	checkGeneratedScopeWithAliases(t, pkg, false, existing...)
}

func checkGeneratedScopeWithAliases(t *testing.T, pkg *Package, wantGenericAlias bool, existing ...string) {
	t.Helper()
	var out bytes.Buffer
	if err := pkg.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	existing = append(existing, out.String())
	files := make([]*ast.File, len(existing))
	for i, source := range existing {
		file, err := parser.ParseFile(fset, "source.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[i] = file
	}
	hasGenericAlias := false
	ast.Inspect(files[len(files)-1], func(node ast.Node) bool {
		if spec, ok := node.(*ast.TypeSpec); ok && spec.Assign.IsValid() && spec.TypeParams != nil {
			hasGenericAlias = true
		}
		return !hasGenericAlias
	})
	if hasGenericAlias != wantGenericAlias {
		t.Fatalf("generic alias in generated Go = %t, want %t\n%s", hasGenericAlias, wantGenericAlias, out.String())
	}
	if wantGenericAlias && version.Compare(runtime.Version(), "go1.24") < 0 {
		// Go 1.23 can parse generic aliases, but type-checking needs experimental support.
		return
	}
	conf := types.Config{Importer: importer.Default(), GoVersion: "go1.23"}
	if _, err := conf.Check(pkg.Path(), fset, files, nil); err != nil {
		t.Fatalf("generated Go does not type-check: %v\n%s", err, out.String())
	}
}

func TestShadowedEmbeddedType(t *testing.T) {
	for _, tt := range []struct {
		name   string
		object string
		typ    types.Type
	}{
		{"Any", "any", TyEmptyInterface},
		{"Int", "int", types.Typ[types.Int]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("p", "p", nil)
			typ := types.NewStruct([]*types.Var{types.NewField(0, pkg.Types, tt.object, tt.typ, true)}, nil)
			param := types.NewParam(0, pkg.Types, tt.object, types.Typ[types.String])
			other := types.NewParam(0, pkg.Types, "_autoGo_1", types.Typ[types.Int])
			cb := pkg.NewFunc(nil, "convert", types.NewTuple(param, other), nil, false).BodyStart(pkg)
			cb.NewTypeDefs().NewType("List", nil).AliasType(pkg, types.NewSlice(typ))
			cb.NewTypeDefs().NewType("Table", nil).AliasType(pkg, types.NewMap(types.Typ[types.String], typ))
			cb.End()
			checkGeneratedScope(t, pkg)
		})
	}
}

func TestEquivalentEmbeddedTypeBinding(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	name := types.NewField(0, pkg.Types, "any", TyEmptyInterface, true)
	builder := pkg.Import("strings").Ref("Builder").Type()
	embedded := types.NewField(0, pkg.Types, "Builder", types.NewPointer(builder), true)
	typ := types.NewStruct([]*types.Var{name, embedded}, nil)
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	cb.NewTypeDefs().NewType("any", nil).AliasType(pkg, TyEmptyInterface)
	cb.NewTypeDefs().NewType("List", nil).AliasType(pkg, types.NewSlice(typ))
	cb.End()
	checkGeneratedScope(t, pkg)

	var out bytes.Buffer
	if err := pkg.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "_autoGo_") {
		t.Fatalf("generated an alias for an equivalent type name:\n%s", out.String())
	}
}

func TestLocalEmbeddedTypeSpelling(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	named := cb.NewTypeDefs().NewType("Inner", nil).InitType(pkg, types.Typ[types.Int])
	typ := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Inner", named, true),
	}, []string{"name", ""})
	cb.Block()
	cb.NewTypeDefs().NewType("Sibling", nil).AliasType(pkg, typ)
	cb.End()
	cb.NewTypeDefs().NewType("Source", nil).AliasType(pkg, typ)
	cb.DefineVarStart(0, "any").Val(1).EndInit(1)
	cb.VarRef(nil).VarVal("any").Assign(1)
	cb.NewTypeDefs().NewType("Output", nil).AliasType(pkg, types.NewSlice(typ))
	cb.End()
	checkGeneratedScope(t, pkg)

	var first, second bytes.Buffer
	if err := pkg.WriteTo(&first); err != nil {
		t.Fatal(err)
	}
	if err := pkg.WriteTo(&second); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() || strings.Count(first.String(), "type _autoGo_") != 1 {
		t.Fatalf("repeated writes changed the generated alias\n%s\n%s", first.String(), second.String())
	}
}

func TestLocalEmbeddedTypeAliasAfterDeletingDeclaration(t *testing.T) {
	for _, tt := range []struct {
		name    string
		keepUse bool
	}{
		{name: "Referenced", keepUse: true},
		{name: "Unused"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var reported []error
			pkg := NewPackage("p", "p", &Config{
				HandleErr: func(err error) {
					reported = append(reported, err)
				},
			})
			cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
			local := cb.NewTypeDefs().NewType("Local", nil).InitType(pkg, types.Typ[types.Int])
			typ := types.NewStruct([]*types.Var{
				types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
				types.NewField(0, pkg.Types, "Local", local, true),
			}, nil)
			defs := cb.NewTypeDefs()
			source := defs.NewType("Source", nil)
			source.AliasType(pkg, typ)
			source.Delete()
			defs.Complete()
			cb.DefineVarStart(0, "any").Val(1).EndInit(1)
			cb.VarRef(nil).VarVal("any").Assign(1)
			output := cb.NewTypeDefs().NewType("Output", nil)
			output.AliasType(pkg, types.NewSlice(typ))
			if !tt.keepUse {
				output.Delete()
			}
			cb.End()

			if tt.keepUse {
				pkg.ASTFile()
				if len(reported) != 1 || !strings.Contains(reported[0].Error(), "missing declaration site for local type alias") {
					t.Fatalf("reported errors = %v, want a missing alias declaration", reported)
				}
			} else {
				checkGeneratedScope(t, pkg)
				if len(reported) != 0 {
					t.Fatalf("reported errors = %v, want none for an unused alias", reported)
				}
			}
		})
	}
}

func TestGenericEmbeddedTypeSpelling(t *testing.T) {
	source := `package p
type Inner[S ~[]E, E ~int] struct { V S }
type Other[E ~int] struct { Value E }
`
	if version.Compare(runtime.Version(), "go1.24") >= 0 {
		source = strings.Replace(source, "Other[E ~int] struct", "Other[E ~int] = struct", 1)
	}
	fset := token.NewFileSet()
	input, err := parser.ParseFile(fset, "input.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	imported, err := conf.Check("p", fset, []*ast.File{input}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pkg := NewPackage("p", "p", &Config{Types: imported})
	params := types.NewTuple(types.NewParam(0, pkg.Types, "any", types.Typ[types.Int]))
	cb := pkg.NewFunc(nil, "convert", params, nil, false).BodyStart(pkg)
	local := cb.NewTypeDefs().NewType("Local", nil).InitType(pkg, types.Typ[types.Int])
	inner, err := types.Instantiate(nil, imported.Scope().Lookup("Inner").Type(), []types.Type{types.NewSlice(local), local}, true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := types.Instantiate(nil, imported.Scope().Lookup("Other").Type(), []types.Type{local}, true)
	if err != nil {
		t.Fatal(err)
	}
	typ := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Inner", types.NewPointer(inner), true),
		types.NewField(0, pkg.Types, "Other", other, true),
		types.NewField(0, pkg.Types, "Info", local, false),
	}, nil)
	cb.NewTypeDefs().NewType("Output", nil).AliasType(pkg, types.NewSlice(typ))
	cb.End()
	checkGeneratedScopeWithAliases(t, pkg, true, source)
}

func TestTypeASTWithoutPackage(t *testing.T) {
	typ := types.NewStruct([]*types.Var{types.NewField(0, nil, "any", TyEmptyInterface, true)}, nil)
	expr := TypeAST(nil, typ)
	st, ok := expr.(*ast.StructType)
	if !ok || len(st.Fields.List) != 1 {
		t.Fatalf("type AST = %v, want one-field struct", expr)
	}
	if field, ok := st.Fields.List[0].Type.(*ast.Ident); !ok || field.Name != "any" {
		t.Fatalf("type AST has an invalid embedded field: %v", st.Fields.List[0])
	}
	param := types.NewTypeParam(types.NewTypeName(0, nil, "T", nil), types.NewInterfaceType(nil, nil))
	if name, ok := TypeAST(nil, param).(*ast.Ident); !ok || name.Name != "T" {
		t.Fatalf("type parameter AST = %v, want T", name)
	}
}

func TestLocalEmbeddedTypeAliasInControlFlow(t *testing.T) {
	for _, tt := range []struct {
		name  string
		start func(*CodeBuilder, *types.Struct, *types.Var)
	}{
		{"TypeSwitch", func(cb *CodeBuilder, typ *types.Struct, input *types.Var) {
			cb.TypeSwitch("").Val(input).TypeAssertThen().TypeCase().Typ(typ).Then()
		}},
		{"Select", func(cb *CodeBuilder, typ *types.Struct, input *types.Var) {
			recv := types.NewChan(types.SendRecv, typ)
			cb.Select().CommCase().Val(cb.pkg.Builtin().Ref("make")).Typ(recv).Call(1).UnaryOp(token.ARROW).EndStmt().Then()
		}},
		{"Default", func(cb *CodeBuilder, typ *types.Struct, input *types.Var) {
			cb.Select().CommDefaultThen()
			cb.NewTypeDefs().NewType("Source", nil).AliasType(cb.pkg, typ)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("p", "p", nil)
			input := types.NewParam(0, pkg.Types, "input", TyEmptyInterface)
			cb := pkg.NewFunc(nil, "convert", types.NewTuple(input), nil, false).BodyStart(pkg)
			local := cb.NewTypeDefs().NewType("Inner", nil).InitType(pkg, types.Typ[types.Int])
			typ := types.NewStruct([]*types.Var{
				types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
				types.NewField(0, pkg.Types, "Inner", local, true),
			}, nil)
			tt.start(cb, typ, input)
			cb.DefineVarStart(0, "any").Val(1).EndInit(1)
			cb.VarRef(nil).VarVal("any").Assign(1)
			cb.NewTypeDefs().NewType("Output", nil).AliasType(pkg, types.NewSlice(typ))
			cb.End().End().End()
			checkGeneratedScope(t, pkg)
		})
	}
}

func TestLocalEmbeddedTypeAfterLabel(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	label := cb.NewLabel(0, 0, "start")
	cb.Goto(label).Label(label)
	node := cb.NewTypeDefs().NewType("Node", nil)
	typ := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Node", types.NewPointer(node.Type()), true),
	}, nil)
	node.InitType(pkg, types.NewStruct([]*types.Var{types.NewField(0, pkg.Types, "Value", typ, false)}, nil))
	cb.DefineVarStart(0, "any").Val(1).EndInit(1)
	cb.VarRef(nil).VarVal("any").Assign(1)
	cb.NewTypeDefs().NewType("Output", nil).AliasType(pkg, types.NewSlice(typ))
	cb.End()
	checkGeneratedScope(t, pkg)
}

func TestShadowedTypeNamesInStruct(t *testing.T) {
	const source = `package p
type Holder[T comparable] struct { Value T }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	input, err := conf.Check("p", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pkg := NewPackage("p", "p", &Config{Types: input})
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	local := cb.NewTypeDefs().NewType("Local", nil).InitType(pkg, types.Typ[types.Int])
	localAlias := cb.NewTypeDefs().NewType("LocalAlias", nil).AliasType(pkg, local)
	holder, err := types.Instantiate(nil, input.Scope().Lookup("Holder").Type(), []types.Type{local}, true)
	if err != nil {
		t.Fatal(err)
	}
	elem := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Entry", holder, false),
		types.NewField(0, pkg.Types, "Value", localAlias, false),
		types.NewField(0, pkg.Types, "Items", types.NewSlice(types.Typ[types.Int]), false),
	}, nil)
	list := types.NewSlice(elem)
	cb.NewVar(list, "original").Block()
	cb.NewTypeDefs().NewType("Holder", nil).InitType(pkg, types.Typ[types.Bool])
	cb.NewTypeDefs().NewType("Local", nil).InitType(pkg, types.Typ[types.String])
	cb.NewTypeDefs().NewType("LocalAlias", nil).InitType(pkg, types.Typ[types.String])
	cb.NewTypeDefs().NewType("int", nil).InitType(pkg, types.Typ[types.String])
	cb.NewVar(list, "generated")
	cb.VarRef("generated").VarVal("original").Assign(1)
	cb.VarRef(nil).VarVal("generated").Assign(1)
	cb.NewTypeDefs().NewType("Table", nil).AliasType(pkg, types.NewMap(types.Typ[types.String], elem))
	cb.End().End()
	checkGeneratedScopeWithAliases(t, pkg, true, source)
}

func TestPackageDeclarationsUseDeclaredScope(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	cb.DefineVarStart(0, "any").Val(1).EndInit(1)
	typ := types.NewStruct([]*types.Var{types.NewField(0, pkg.Types, "any", TyEmptyInterface, true)}, nil)
	pkg.NewVar(0, typ, "global")
	pkg.NewTypeDefs().NewType("Defined", nil).InitType(pkg, typ)
	pkg.NewTypeDefs().NewType("Alias", nil).AliasType(pkg, typ)
	param := types.NewParam(0, pkg.Types, "input", typ)
	pkg.NewFunc(nil, "take", types.NewTuple(param), nil, false)
	cb.VarRef(nil).VarVal("any").Assign(1)
	cb.End()

	var out bytes.Buffer
	if err := pkg.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "_autoGo_") {
		t.Fatalf("generated a shadowing alias for a package declaration:\n%s", out.String())
	}
	checkGeneratedScope(t, pkg)
}

func TestClosureWithInferredResultAndEmbeddedType(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	cb := pkg.NewFunc(nil, "convert", nil, nil, false).BodyStart(pkg)
	local := cb.NewTypeDefs().NewType("Local", nil).InitType(pkg, types.Typ[types.Int])
	elem := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Local", local, true),
	}, nil)
	input := types.NewParam(0, pkg.Types, "input", types.NewSlice(elem))
	result := types.NewParam(0, pkg.Types, "result", types.Typ[types.Invalid])
	cb.VarRef(nil).NewClosure(types.NewTuple(input), types.NewTuple(result), false).BodyStart(pkg)
	cb.DefineVarStart(0, "any").Val(1).EndInit(1)
	cb.VarRef(nil).VarVal("any").Assign(1)
	cb.NewVar(input.Type(), "values")
	cb.VarRef("values").Val(input).Assign(1)
	cb.VarRef(nil).VarVal("values").Assign(1)
	*result = *types.NewVar(0, pkg.Types, "result", types.Typ[types.Int])
	cb.Val(1).Return(1).End()
	cb.Assign(1).End()
	checkGeneratedScope(t, pkg)
}

func TestShadowedEmbeddedAnyInGenericFunction(t *testing.T) {
	pkg := NewPackage("p", "p", nil)
	pkg.NewVar(0, types.Typ[types.UnsafePointer], "pointer")
	checkGeneratedScope(t, pkg)

	param := types.NewTypeParam(types.NewTypeName(0, pkg.Types, "T", nil), TyAny)
	elem := types.NewStruct([]*types.Var{
		types.NewField(0, pkg.Types, "any", TyEmptyInterface, true),
		types.NewField(0, pkg.Types, "Value", param, false),
		types.NewField(0, pkg.Types, "Pointer", types.Typ[types.UnsafePointer], false),
	}, nil)
	input := types.NewParam(0, pkg.Types, "input", types.NewSlice(elem))
	sig := types.NewSignatureType(nil, nil, []*types.TypeParam{param}, types.NewTuple(input), nil, false)
	fn, err := pkg.NewFuncWith(0, "convert", sig, nil)
	if err != nil {
		t.Fatal(err)
	}
	cb := fn.BodyStart(pkg)
	cb.DefineVarStart(0, "any").Val(1).EndInit(1)
	cb.VarRef(nil).VarVal("any").Assign(1)
	cb.NewVar(input.Type(), "output")
	cb.VarRef("output").Val(input).Assign(1)
	cb.VarRef(nil).VarVal("output").Assign(1)
	cb.End()
	checkGeneratedScopeWithAliases(t, pkg, true)
}
