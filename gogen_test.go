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

package gogen_test

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/goplus/gogen"
	"github.com/goplus/gogen/packages"
)

// aliasType is a test helper mirroring the removed Package.AliasType: it
// declares a package-level type alias using the current API
// (NewType(...).AliasType(...)). It lives in this build-tag-agnostic file so
// both the default and genjs test builds can use it.
func aliasType(pkg *gogen.Package, name string, typ types.Type, src ...ast.Node) types.Type {
	return pkg.NewTypeDefs().NewType(name, nil, src...).AliasType(pkg, typ)
}

var (
	gblFset   *token.FileSet
	gblImp    types.Importer
	handleErr func(err error)
)

func init() {
	gogen.SetDebug(gogen.DbgFlagAll)
	gblFset = token.NewFileSet()
	gblImp = packages.NewImporter(gblFset)
}

func ctxRef(pkg *gogen.Package, name string) gogen.Ref {
	_, o := pkg.CB().Scope().LookupParent(name, token.NoPos)
	return o
}

type eventRecorder struct{}

func (p eventRecorder) Member(id ast.Node, obj types.Object) {}
func (p eventRecorder) Call(fn ast.Node, obj types.Object)   {}

type txtNode struct {
	Msg string
	pos token.Pos
}

func (p *txtNode) Pos() token.Pos {
	return p.pos
}

func (p *txtNode) End() token.Pos {
	return p.pos + token.Pos(len(p.Msg))
}

var (
	mutex         sync.Mutex
	pos2Positions = map[token.Pos]token.Position{}
)

// text, line, column
func source(text string, args ...interface{}) ast.Node {
	if len(args) < 2 {
		return &txtNode{Msg: text}
	}
	pos := position(args[0].(int), args[1].(int))
	return &txtNode{Msg: text, pos: pos}
}

const (
	posBits = 16
	posMask = (1 << posBits) - 1
)

func position(line, column int) token.Pos {
	mutex.Lock()
	defer mutex.Unlock()

	pos := token.Pos(len(pos2Positions)+1) << posBits
	pos2Positions[pos] = token.Position{Filename: "./foo.gop", Line: line, Column: column}
	return pos
}

type nodeInterp struct{}

func (p nodeInterp) Position(pos token.Pos) (ret token.Position) {
	mutex.Lock()
	defer mutex.Unlock()
	ret = pos2Positions[(pos &^ posMask)]
	ret.Column += int(pos & posMask)
	return
}

func (p nodeInterp) Caller(node ast.Node) string {
	t := node.(*txtNode)
	idx := strings.Index(t.Msg, "(")
	if idx > 0 {
		return t.Msg[:idx]
	}
	return t.Msg
}

func (p nodeInterp) LoadExpr(node ast.Node) string {
	t := node.(*txtNode)
	return t.Msg
}

func newParam(pkg *gogen.Package, pos token.Pos, name string, typ types.Type) *types.Var {
	return types.NewParam(pos, pkg.Types, name, typ)
}

func newMainPackage(
	implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	return newPackage("main", implicitCast...)
}

func newPackage(
	name string, implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	conf := &gogen.Config{
		Fset:            gblFset,
		Importer:        gblImp,
		Recorder:        eventRecorder{},
		NodeInterpreter: nodeInterp{},
		DbgPositioner:   nodeInterp{},
	}
	if len(implicitCast) > 0 {
		conf.CanImplicitCast = implicitCast[0]
	}
	if handleErr != nil {
		conf.HandleErr = handleErr
		handleErr = nil
	}
	return gogen.NewPackage("", name, conf)
}

func domTest(t *testing.T, pkg *gogen.Package, expected string) {
	domTestEx(t, pkg, expected, "")
}

func domTestEx(t *testing.T, pkg *gogen.Package, expected string, fname string) {
	doDomTest(t, pkg, expected, fname)
	log.Printf("====================== %s End =========================\n", t.Name())
}

func doDomTest(t *testing.T, pkg *gogen.Package, expected string, fname string) {
	var b bytes.Buffer
	t.Helper()
	err := gogen.WriteTo(&b, pkg, fname)
	if err != nil {
		t.Fatal("gogen.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\ngogen.WriteTo Result:\n%s\nExpected:\n%s\n", result, expected)
	}
}

func TestCodeBuilderAliasCall(t *testing.T) {
	for _, name := range []string{"Function", "NamedFunction", "AliasChain", "GenericFunction", "VariadicFunction", "FunctionResult", "StructField"} {
		t.Run(name, func(t *testing.T) {
			pkg := gogen.NewPackage("", "foo", nil)
			result := aliasType(pkg, "Result", types.Typ[types.Int])
			paramType := types.Type(types.Typ[types.Int])
			variadic := name == "VariadicFunction"
			if variadic {
				paramType = types.NewSlice(paramType)
			}
			sig := types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "value", paramType)),
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", result)), variadic)
			var typ types.Type = sig
			if name == "NamedFunction" {
				typ = pkg.NewType("Callback").InitType(pkg, sig)
			}
			if name == "GenericFunction" {
				tp := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T", nil), gogen.TyEmptyInterface)
				tuple := types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", tp))
				generic := types.NewAlias(types.NewTypeName(token.NoPos, pkg.Types, "Generic", nil),
					types.NewSignatureType(nil, nil, nil, tuple, tuple, false))
				generic.SetTypeParams([]*types.TypeParam{tp})
				var err error
				typ, err = types.Instantiate(nil, generic, []types.Type{result}, true)
				if err != nil {
					t.Fatal(err)
				}
			}
			typ = aliasType(pkg, "Alias", typ)
			if name == "AliasChain" {
				typ = aliasType(pkg, "Next", typ)
			}
			fn := types.NewVar(token.NoPos, pkg.Types, "fn", typ)
			cb := pkg.CB()
			switch name {
			case "FunctionResult":
				factory := types.NewFunc(token.NoPos, pkg.Types, "factory",
					types.NewSignatureType(nil, nil, nil, nil,
						types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", typ)), false))
				cb.Val(factory).Call(0)
			case "StructField":
				holder := types.NewStruct([]*types.Var{types.NewField(token.NoPos, pkg.Types, "Callback", typ, false)}, nil)
				cb.Val(types.NewVar(token.NoPos, pkg.Types, "holder", holder)).MemberVal("Callback", 0)
			default:
				cb.Val(fn)
			}
			if got := cb.Get(-1).Type; got != typ {
				t.Fatalf("function type = %v, want preserved alias %v", got, typ)
			}
			if name != "NamedFunction" && !gogen.IsFunc(cb.Get(-1).Type) {
				t.Error("function alias is not recognized as a function")
			}
			cb.Val(types.NewVar(token.NoPos, pkg.Types, "value", types.Typ[types.Int])).Call(1)
			if got := cb.Get(-1).Type; got != result {
				t.Errorf("call type = %v, want %v", got, result)
			}
		})
	}
}

func TestHasAutoPropertyAlias(t *testing.T) {
	for _, count := range []int{0, 1} {
		name := "NoParams"
		if count != 0 {
			name = "WithParam"
		}
		t.Run(name, func(t *testing.T) {
			pkg := gogen.NewPackage("", "foo", nil)
			var params *types.Tuple
			if count != 0 {
				params = types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", types.Typ[types.Int]))
			}
			sig := types.NewSignatureType(nil, nil, nil, params, nil, false)
			alias := aliasType(pkg, "Alias", aliasType(pkg, "Inner", sig))
			if got, want := gogen.HasAutoProperty(alias), count == 0; got != want {
				t.Errorf("alias auto-property = %v, want %v", got, want)
			}
		})
	}
}
