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

package gogen

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/go/format"
	"github.com/goplus/gogen/packages"
)

var (
	gblConf = getConf()
)

func init() {
	debugImportOsx = true
}

// aliasType is a test helper mirroring the removed Package.AliasType: it
// declares a package-level type alias using the current API
// (NewType(...).AliasType(...)).
func aliasType(pkg *Package, name string, typ types.Type, src ...ast.Node) types.Type {
	return pkg.NewTypeDefs().NewType(name, nil, src...).AliasType(pkg, typ)
}

func getConf() *Config {
	fset := token.NewFileSet()
	imp := packages.NewImporter(fset)
	return &Config{Fset: fset, Importer: imp}
}

/*
NOTE(xsw): Remove auto-unpacking (https://github.com/goplus/xgo/issues/2538)

func Test_commaOK(t *testing.T) {
	x := types.NewField(token.NoPos, nil, "X_0", types.Typ[types.Int], false)
	y := types.NewField(token.NoPos, nil, "X_1", types.Typ[types.Int], false)
	flds := []*types.Var{x, y}
	if commaOK(types.NewStruct(flds, nil), 2) {
		t.Fatal("commaOK(2): return true?")
	}
	if !commaOK(types.NewStruct(flds, nil), 3) {
		t.Fatal("commaOK(3): return false?")
	}
}
*/

func TestSwitchStmtThen(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	defer func() {
		if e := recover(); e != "use None() for empty switch tag" {
			t.Fatal("TestSwitchStmtThen:", e)
		}
	}()
	cb.Switch().Then()
}

func TestSwitchStmtThen2(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	defer func() {
		if e := recover(); e != "switch statement has too many init statements" {
			t.Fatal("TestSwitchStmtThen:", e)
		}
	}()
	cb.Switch()
	cb.emitStmt(&ast.EmptyStmt{})
	cb.emitStmt(&ast.EmptyStmt{})
	cb.None().Then()
}

func TestIfStmtThen(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	defer func() {
		if e := recover(); e != "if statement has too many init statements" {
			t.Fatal("TestIfStmtThen:", e)
		}
	}()
	cb.If()
	cb.emitStmt(&ast.EmptyStmt{})
	cb.emitStmt(&ast.EmptyStmt{})
	cb.Val(true).Then()
}

func TestIfStmtElse(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	defer func() {
		if e := recover(); e != "else statement already exists" {
			t.Fatal("TestIfStmtThen:", e)
		}
	}()
	cb.If().Else().Else()
}

func TestCommCase(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	cb.emitStmt(&ast.EmptyStmt{})
	cb.emitStmt(&ast.EmptyStmt{})
	defer func() {
		if e := recover(); e != "multi commStmt in comm clause?" {
			t.Fatal("TestCommCase:", e)
		}
	}()
	c := &commCase{}
	c.Then(cb)
}

func TestCheckXGoDeps(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	file := pkg.CurFile()
	id := file.newImport("env", "github.com/goplus/xgo/env")
	file.forceImport("github.com/qiniu/x/errors")
	file.getDecls(pkg)
	if v := file.CheckXGoDeps(pkg); v != FlagDepModX {
		t.Fatal("CheckXGoDeps:", v)
	}
	id.Obj.Data = importUsed(true)
	if v := file.CheckXGoDeps(pkg); v != FlagDepModXGo|FlagDepModX {
		t.Fatal("CheckXGoDeps:", v)
	}
}

// TestCheckXGoDepsWithNilFuncType tests that CheckXGoDeps does not panic
// when a FuncDecl has a nil Type field (e.g., when generated before End() is called).
func TestCheckXGoDepsWithNilFuncType(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	file := pkg.CurFile()
	file.forceImport("github.com/qiniu/x/errors")
	// Simulate a FuncDecl with nil Type (as created by NewFuncWith before End() is called)
	file.appendFuncDecl(&funcDecl{}, nil)
	file.dirtyUsed = true
	// Should not panic
	_ = file.CheckXGoDeps(pkg)
}

func TestInitXGoPkg(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	pkg.Types.Scope().Insert(types.NewVar(
		token.NoPos, pkg.Types, "XGoPackage", types.Typ[types.Bool],
	))
	pkg.initXGoPkg(nil, pkg.Types)
}

func TestCheckOverloads(t *testing.T) {
	defer func() {
		if e := recover(); e != "checkOverloads: should be string constant - foo" {
			t.Fatal("TestCheckOverloads:", e)
		}
	}()
	scope := types.NewScope(nil, 0, 0, "")
	scope.Insert(types.NewLabel(0, nil, "foo"))
	checkOverloads(scope, "bar")
	checkOverloads(scope, "foo")
}

func TestCheckXGoPkgNoop(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	pkg.Types.Scope().Insert(types.NewConst(
		token.NoPos, pkg.Types, "XGoPackage", types.Typ[types.UntypedBool], constant.MakeBool(true),
	))
	if _, ok := checkXGoPkg(pkg); ok {
		t.Fatal("checkXGoPkg: ok?")
	}
}

func TestDenoted(t *testing.T) {
	if denoteRecv(&ast.SelectorExpr{Sel: ast.NewIdent("foo")}) != nil {
		t.Fatal("denoteRecv: not nil?")
	}
	id := ast.NewIdent("foo")
	obj := &ast.Object{}
	setDenoted(id, obj)
	if getDenoted(id) != obj {
		t.Fatal("setDenoted failed")
	}
}

func TestCheckNamed(t *testing.T) {
	foo := types.NewPackage("github.com/bar/foo", "foo")
	tn := types.NewTypeName(0, foo, "t", nil)
	typ := types.NewNamed(tn, types.Typ[types.Int], nil)
	if v, ok := checkNamed(types.NewPointer(typ)); !ok || v != typ {
		t.Fatal("TestCheckNamed failed:", v, ok)
	}
}

func TestErrMethodSig(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	foo := types.NewPackage("github.com/bar/foo", "foo")
	tn := types.NewTypeName(0, foo, "t", nil)
	recv := types.NewNamed(tn, types.Typ[types.Int], nil)
	t.Run("methodToFuncSig global func", func(t *testing.T) {
		fnt := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		fn := types.NewFunc(0, foo, "bar", fnt)
		if methodToFuncSig(pkg, fn, &internal.Elem{}) != fnt {
			t.Fatal("methodToFuncSig failed")
		}
	})
	t.Run("recv not pointer", func(t *testing.T) {
		defer func() {
			if e := recover(); e != "recv of method github.com/bar/foo.t.bar isn't a pointer\n" {
				t.Fatal("TestErrMethodSigOf:", e)
			}
		}()
		method := types.NewSignatureType(types.NewVar(0, foo, "", recv), nil, nil, nil, nil, false)
		arg := &Element{
			Type: &TypeType{typ: types.NewPointer(recv)},
		}
		ret := &Element{
			Val: &ast.SelectorExpr{Sel: ast.NewIdent("bar")},
		}
		pkg.cb.methodSigOf(method, memberFlagMethodToFunc, arg, ret)
	})
}

func TestMatchOverloadNamedTypeCast(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	foo := types.NewPackage("github.com/bar/foo", "foo")
	tn := types.NewTypeName(0, foo, "t", nil)
	types.NewNamed(tn, types.Typ[types.Int], nil)
	_, err := matchOverloadNamedTypeCast(pkg, tn, nil, nil, 0, 0)
	if err == nil || err.Error() != "-: typecast github.com/bar/foo.t not found" {
		t.Fatal("TestMatchOverloadNamedTypeCast:", err)
	}
}

func TestSetTypeParams(t *testing.T) {
	pkg := types.NewPackage("", "")
	tn := types.NewTypeName(0, pkg, "foo__1", nil)
	named := types.NewNamed(tn, TyByte, nil)

	setTypeParams(nil, named, &ast.TypeSpec{}, nil)
}

func TestOverloadNameds(t *testing.T) {
	pkg := types.NewPackage("", "")
	tn := types.NewTypeName(0, pkg, "foo__1", nil)
	named := types.NewNamed(tn, TyByte, nil)
	func() {
		defer func() {
			if e := recover(); e != "overload type foo__1 out of range 0..0\n" {
				t.Fatal("TestOverloadFuncs:", e)
			}
		}()
		overloadNameds(5, []*types.Named{named})
	}()
	func() {
		defer func() {
			if e := recover(); e != "overload type foo__1 exists?\n" {
				t.Fatal("TestOverloadFuncs:", e)
			}
		}()
		overloadNameds(5, []*types.Named{named, named})
	}()
}

func TestOverloadFuncs(t *testing.T) {
	pkg := types.NewPackage("", "")
	fn := types.NewFunc(0, pkg, "foo__1", nil)
	func() {
		overloadFuncs(pkg, 5, []types.Object{fn})
	}()
	func() {
		defer func() {
			if e := recover(); e != "overload func foo__1 exists?\n" {
				t.Fatal("TestOverloadFuncs:", e)
			}
		}()
		overloadFuncs(pkg, 5, []types.Object{fn, fn})
	}()
}

func TestCheckTypeMethod(t *testing.T) {
	scope := types.NewScope(nil, 0, 0, "")
	func() {
		defer func() {
			if e := recover(); e != "checkTypeMethod: notFound not found or not a named type\n" {
				t.Fatal("TestCheckTypeMethod:", e)
			}
		}()
		checkTypeMethod(scope, "_notFound__method")
	}()
}

func TestNewPosNode(t *testing.T) {
	if ret := NewPosNode(1); ret.Pos() != 1 || ret.End() != 1 {
		t.Fatal("NewPosNode(1): end -", ret.End())
	}
	if ret := NewPosNode(1, 2); ret.End() != 2 {
		t.Fatal("NewPosNode(1, 2): end -", ret.End())
	}
}

func TestGetSrcPos(t *testing.T) {
	if getSrcPos(nil) != token.NoPos {
		t.Fatal("TestGetSrcPos: not nopos?")
	}
}

func TestIsTypeEx(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	o := NewInstruction(0, pkg, "bar", lenInstr{})
	if !IsTypeEx(o.Type()) {
		t.Fatal("IsTypeEx: not Instruction?")
	}
	of := NewOverloadFunc(0, pkg, "bar")
	if !IsTypeEx(of.Type()) {
		t.Fatal("IsTypeEx: not OverloadFunc?")
	}
	if IsTypeEx(tyInt) {
		t.Fatal("IsTypeEx: not tyInt?")
	}
}

func TestGetBuiltinTI(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := &pkg.cb
	if cb.getBuiltinTI(types.NewPointer(types.Typ[0])) != nil {
		t.Fatal("TestGetBuiltinTI failed")
	}
	tiStr := cb.getBuiltinTI(types.Typ[types.String])
	sig := tiStr.lookupByName("Index")
	if sig == nil || sig.Params().Len() != 1 {
		t.Fatal("string.Index (Params):", sig)
	}
	if sig.Results().Len() != 1 {
		t.Fatal("string.Index (Results):", sig)
	}
	if tsig := tiStr.lookupByName("__unknown"); tsig != nil {
		t.Fatal("tsig:", tsig)
	}
}

func Test_aliasNameOf(t *testing.T) {
	name, flag := aliasNameOf("_text", MemberFlagAutoProperty)
	if name != "XGo_text" || flag != MemberFlagAutoProperty {
		t.Fatal("Test_aliasNameOf:", name, flag)
	}
}

func TestFindMethodType(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	tyFile := pkg.Import("os").Ref("File").Type().(*types.Named)
	sig := findMethodType(&pkg.cb, tyFile, "XGo_Enum")
	if sig == nil || sig.Params().Len() != 0 {
		t.Fatal("os.File.XGoEnum (Params):", sig)
	}
	if sig.Results().Len() != 1 {
		t.Fatal("os.File.XGoEnum (Results):", sig)
	}
}

func TestContractName(t *testing.T) {
	testcases := []struct {
		Contract
		name string
	}{
		{_any, "any"},
		{capable, "capable"},
		{lenable, "lenable"},
		{makable, "makable"},
		{_bool, "bool"},
		{ninteger, "ninteger"},
		{orderable, "orderable"},
		{integer, "integer"},
		{number, "number"},
		{addable, "addable"},
		{comparable, "comparable"},
		{clearable, "clearable"},
	}
	for _, c := range testcases {
		if c.String() != c.name {
			t.Fatal("Unexpected contract name:", c.name)
		}
	}
}

func TestContract(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	at := types.NewPackage("foo", "foo")
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	tfoo := foo.Ref("Foo5").Type()
	tarr := types.NewArray(tyInt, 10)
	arrayAlias := aliasType(pkg, "Array", tarr)
	testcases := []struct {
		Contract
		typ    types.Type
		result bool
	}{
		{integer, tyInt, true},
		{capable, types.Typ[types.String], false},
		{capable, types.NewPointer(arrayAlias), true},
		{lenable, types.NewPointer(arrayAlias), true},
		{capable, types.NewPointer(aliasType(pkg, "Item", tyInt)), false},
		{capable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), tarr, nil), true},
		{capable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.NewPointer(tarr), nil), true},
		{capable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.Typ[types.String], nil), false},
		{lenable, types.Typ[types.String], true},
		{lenable, types.NewMap(tyInt, tyInt), true},
		{lenable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.Typ[types.String], nil), true},
		{makable, types.NewMap(tyInt, tyInt), true},
		{makable, types.NewChan(0, tyInt), true},
		{makable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), tyInt, nil), false},
		{comparable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), tyInt, nil), true},
		{comparable, types.NewSlice(tyInt), false},
		{comparable, types.NewMap(tyInt, tyInt), false},
		{comparable, types.NewChan(0, tyInt), true},
		{comparable, types.NewSignatureType(nil, nil, nil, nil, nil, false), false},
		{comparable, NewTemplateSignature(nil, nil, nil, nil, false), false},
		{addable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.Typ[types.Bool], nil), false},
		{addable, tfoo, true},
		{clearable, types.NewMap(tyInt, tyInt), true},
		{clearable, types.NewSlice(tyInt), true},
		{clearable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.NewSlice(tyInt), nil), true},
		{clearable, types.Typ[types.String], false},
	}
	for _, c := range testcases {
		if c.Match(pkg, c.typ) != c.result {
			t.Fatalf("%s.Match %v expect %v\n", c.String(), c.typ, c.result)
		}
	}
}

func TestComparableTo(t *testing.T) {
	tyStr := types.NewNamed(types.NewTypeName(token.NoPos, nil, "str", nil), types.Typ[types.String], nil)
	cases := []struct {
		v, t types.Type
		ret  bool
	}{
		{types.Typ[types.UntypedNil], types.Typ[types.Int], false},
		{types.Typ[types.UntypedComplex], types.Typ[types.Int], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.Bool], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.Complex128], true},
		{types.Typ[types.String], types.Typ[types.Bool], false},
		{types.Typ[types.String], types.Typ[types.String], true},
		{types.Typ[types.String], tyStr, true},
		{types.Typ[types.UntypedBool], types.Typ[types.Bool], true},
		{types.Typ[types.Bool], types.Typ[types.UntypedBool], true},
		{types.Typ[types.UntypedRune], types.Typ[types.UntypedString], false},
		{types.Typ[types.Rune], types.Typ[types.UntypedString], false},
		{types.Typ[types.UntypedInt], types.Typ[types.Int64], true},
		{types.Typ[types.Int64], types.Typ[types.UntypedInt], true},
	}
	pkg := NewPackage("", "foo", gblConf)
	for _, a := range cases {
		av := &Element{Type: a.v}
		at := &Element{Type: a.t}
		if ret := ComparableTo(pkg, av, at); ret != a.ret {
			t.Fatalf("Failed: ComparableTo %v => %v returns %v\n", a.v, a.t, ret)
		}
	}
}

func TestComparableTo2(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignatureType(nil, nil, nil, nil, nil, false)),
	}
	methods2 := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "F", types.NewSignatureType(nil, nil, nil, nil, nil, false)),
	}
	tyInterf := types.NewInterfaceType(methods, nil).Complete()
	tyInterfF := types.NewInterfaceType(methods2, nil).Complete()
	bar1 := pkg.NewType("bar").InitType(pkg, tyInterf)
	bar2 := pkg.NewType("bar2").InitType(pkg, tyInterf)
	f1 := pkg.NewType("f1").InitType(pkg, tyInterfF)
	tySlice := types.NewSlice(types.Typ[types.Int])
	cases := []struct {
		v, t types.Type
		ret  bool
	}{
		{bar1, bar2, true},
		{bar1, types.Typ[types.Int], false},
		{types.Typ[types.Int], bar2, false},
		{bar1, tySlice, false},
		{tySlice, bar2, false},
		{f1, bar2, false},
		{types.Typ[types.UntypedNil], bar2, true},
		{bar1, types.Typ[types.UntypedNil], true},
		{tySlice, types.Typ[types.UntypedInt], false},
		{types.Typ[types.UntypedInt], tySlice, false},
		{TyEmptyInterface, types.Typ[types.UntypedInt], true},
		{types.Typ[types.UntypedInt], TyEmptyInterface, true},
	}
	for _, a := range cases {
		av := &Element{Type: a.v}
		at := &Element{Type: a.t}
		if ret := ComparableTo(pkg, av, at); ret != a.ret {
			t.Fatalf("Failed: ComparableTo %v => %v returns %v\n", a.v, a.t, ret)
		}
	}
	av := &Element{Type: types.Typ[types.UntypedFloat], CVal: constant.MakeFromLiteral("1e1", token.FLOAT, 0)}
	at := &Element{Type: types.Typ[types.Int]}
	if !ComparableTo(pkg, av, at) {
		t.Fatalf("Failed: ComparableTo %v => %v returns %v\n", av, at, false)
	}
}

func TestAssignableTo(t *testing.T) {
	cases := []struct {
		v, t types.Type
		ret  bool
	}{
		{types.Typ[types.UntypedInt], types.Typ[types.Int], true},
		{types.Typ[types.Int], types.Typ[types.UntypedInt], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedComplex], true},
		{types.Typ[types.UntypedComplex], types.Typ[types.UntypedFloat], false},
		{types.Typ[types.UntypedInt], types.Typ[types.UntypedFloat], true},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedInt], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedBool], false},
		{types.Typ[types.UntypedInt], types.Typ[types.UntypedRune], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.Int], false},
		{types.Typ[types.UntypedFloat], types.Typ[types.UntypedRune], false},
		{types.Typ[types.UntypedRune], types.Typ[types.UntypedInt], true},
		{types.Typ[types.UntypedRune], types.Typ[types.UntypedFloat], true},
	}
	pkg := NewPackage("", "foo", gblConf)
	for _, a := range cases {
		if ret := AssignableTo(pkg, a.v, a.t); ret != a.ret {
			t.Fatalf("Failed: AssignableTo %v => %v returns %v\n", a.v, a.t, ret)
		}
	}
	if Default(pkg, types.Typ[types.UntypedInt]) != types.Typ[types.Int] {
		t.Fatal("gogen.Default failed")
	}
}

func TestToIndex(t *testing.T) {
	if toIndex('b') != 11 {
		t.Fatal("toIndex('b') != 11")
	}
	defer func() {
		if recover() != "invalid character out of [0-9,a-z]" {
			t.Fatal("toIndex('!') not panic?")
		}
	}()
	toIndex('!')
}

func TestCheckOverloadMethod(t *testing.T) {
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	if _, ok := CheckOverloadMethod(sig); ok {
		t.Fatal("TestCheckOverloadMethod failed:")
	}
}

func TestIsFunc(t *testing.T) {
	if IsFunc(nil) {
		t.Fatal("nil is func?")
	}
	if !IsFunc(types.NewSignatureType(nil, nil, nil, nil, nil, false)) {
		t.Fatal("func() is not func?")
	}
}

func TestCheckUdt(t *testing.T) {
	o := types.NewNamed(types.NewTypeName(token.NoPos, nil, "foo", nil), types.Typ[types.Int], nil)
	var frs forRangeStmt
	var cb CodeBuilder
	if _, ok := frs.checkUdt(&cb, o); ok {
		t.Fatal("findMethod failed: bar exists?")
	}
}

func TestNodeInterp(t *testing.T) {
	interp := nodeInterp{}
	if src := interp.LoadExpr(nil); src != "" {
		t.Fatal("TestNodeInterp interp.LoadExpr failed:", src)
	}
	if caller := getCaller(&internal.Elem{}); caller != "the function call" {
		t.Fatal("TestNodeInterp getCaller failed:", caller)
	}
	if caller, pos, _ := getFunExpr(nil); caller != "the closure call" || pos != token.NoPos {
		t.Fatal("TestNodeInterp getGoExpr failed:", caller, pos)
	}
}

func TestInternalStack(t *testing.T) {
	var cb CodeBuilder
	cb.InternalStack().Push(nil)
	if cb.Get(-1) != nil {
		t.Fatal("InternalStack/Get failed")
	}
}

func TestCheckInterface(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	if typ, ok := cb.checkInterface(types.Typ[types.Int]); typ != nil || ok {
		t.Fatal("TestCheckInterface failed:", typ, ok)
	}

	cb.loadNamed = func(at *Package, t *types.Named) {
		t.SetUnderlying(TyEmptyInterface)
	}
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	if typ, ok := cb.checkInterface(named); typ == nil || !ok {
		t.Fatal("TestCheckInterface failed:", typ, ok)
	}
}

func TestEnsureLoaded(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	cb.loadNamed = func(at *Package, t *types.Named) {
		panic("loadNamed")
	}
	defer func() {
		if e := recover(); e != "loadNamed" {
			t.Fatal("TestEnsureLoaded failed")
		}
	}()
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	cb.ensureLoaded(named)
}

func TestGetUnderlying(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	cb.loadNamed = func(at *Package, t *types.Named) {
		panic("loadNamed")
	}
	defaultLoadNamed(nil, nil)
	defer func() {
		if e := recover(); e != "loadNamed" {
			t.Fatal("TestGetUnderlying failed")
		}
	}()
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	cb.getUnderlying(named)
}

func TestGetUnderlying2(t *testing.T) {
	var pkg = new(Package)
	var cb = &pkg.cb
	cb.pkg = pkg
	cb.loadNamed = func(at *Package, t *types.Named) {
		panic("loadNamed")
	}
	defaultLoadNamed(nil, nil)
	defer func() {
		if e := recover(); e != "loadNamed" {
			t.Fatal("TestGetUnderlying2 failed")
		}
	}()
	named := types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil)
	getUnderlying(pkg, named)
}

func TestWriteFile(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	if WriteFile("/", pkg, "") == nil {
		t.Fatal("WriteFile: no error?")
	}
	pkg.files[""] = &File{fileDecls: fileDecls{goDecls: []ast.Decl{
		&ast.GenDecl{Specs: []ast.Spec{
			&ast.ValueSpec{Type: &ast.Ident{}},
		}},
		nil,
	}}}
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("WriteFile: no error?")
		}
	}()
	WriteFile("_unknown.go", pkg, "")
}

func TestToFields(t *testing.T) {
	pkg := new(Package)
	pkg.Types = types.NewPackage("", "foo")
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "bar", nil), types.Typ[types.Int], nil)
	flds := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "bar", typ, true),
	}
	struc := types.NewStruct(flds, []string{"`bar`"})
	out := toFields(pkg, struc)
	if !(len(out) == 1 && out[0].Names == nil) {
		t.Fatal("TestToFields failed:", out)
	}
}

func TestMethodAutoProperty(t *testing.T) {
	pkg := types.NewPackage("", "")
	typs := []types.Type{
		tyInt,
		sigFuncEx(nil, nil, &TyOverloadFunc{}),
		sigFuncEx(nil, nil, &TyTemplateRecvMethod{types.NewParam(0, nil, "", tyInt)}),
	}
	for _, typ := range typs {
		if methodHasAutoProperty(typ, 0) {
			t.Fatal("TestMethodAutoProperty:", typ)
		}
		if HasAutoProperty(typ) {
			t.Fatal("HasAutoProperty:", typ)
		}
	}
	fnt := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	fn := types.NewFunc(0, pkg, "foo", fnt)
	sig := sigFuncEx(nil, nil, &TyOverloadFunc{Funcs: []types.Object{fn}})
	if !HasAutoProperty(sig) {
		t.Fatal("HasAutoProperty:", sig)
	}
}

func TestCheckSigFuncExObjects(t *testing.T) {
	pkg := types.NewPackage("", "")
	objs := []types.Object{
		types.NewFunc(0, pkg, "foo", types.NewSignatureType(nil, nil, nil, nil, nil, false)),
		types.NewFunc(0, pkg, "bar", types.NewSignatureType(nil, nil, nil, nil, nil, false)),
	}
	named := types.NewNamed(types.NewTypeName(0, pkg, "named", nil), types.NewSignatureType(nil, nil, nil, nil, nil, false), nil)
	fn := types.NewFunc(0, pkg, "fn", sigFuncEx(nil, nil, &TyOverloadFunc{objs}))
	tests := []struct {
		name  string
		sig   *types.Signature
		count int
	}{
		{"TyOverloadFunc", sigFuncEx(nil, nil, &TyOverloadFunc{objs}), 2},
		{"TyOverloadMethod", sigFuncEx(nil, nil, &TyOverloadMethod{objs}), 2},
		{"TyTemplateRecvMethod", sigFuncEx(nil, nil, &TyTemplateRecvMethod{types.NewParam(0, nil, "", tyInt)}), 1},
		{"TyTemplateRecvMethod", sigFuncEx(nil, nil, &TyTemplateRecvMethod{fn}), 2},
		{"TyOverloadNamed", sigFuncEx(nil, nil, &TyOverloadNamed{Types: []*types.Named{named}}), 1},
		{"TyTypeAsParams", sigFuncEx(nil, nil, &TyTypeAsParams{fn}), 2},
		{"TyTypeAsParams", sigFuncEx(nil, nil, &TyTypeAsParams{types.NewFunc(0, nil, "fn", nil)}), 1},
	}
	for n, test := range tests {
		typ, objs := CheckSigFuncExObjects(test.sig)
		if typ == nil || len(objs) != test.count {
			t.Fatalf("CheckSigFuncExObjects error: %v %v", n, test.name)
		}
	}
}

func TestHasAutoProperty(t *testing.T) {
	if HasAutoProperty(nil) {
		t.Fatal("nil has autoprop?")
	}
	if !HasAutoProperty(types.NewSignatureType(nil, nil, nil, nil, nil, false)) {
		t.Fatal("func() has not autoprop?")
	}
}

func TestTypeEx(t *testing.T) {
	subst := &TySubst{}
	pkg := NewPackage("example.com/foo", "foo", gblConf)
	tyInt := types.Typ[types.Int]
	typs := []types.Type{
		&refType{},
		subst,
		&TyOverloadFunc{},
		&TyOverloadMethod{},
		&TyStaticMember{},
		&TyTemplateRecvMethod{},
		&TyInstruction{},
		&TyOverloadNamed{Obj: types.NewTypeName(0, pkg.Types, "bar", tyInt)},
		&TypeType{},
		&TyTypeAsParams{},
		&TemplateSignature{},
	}
	if v := subst.String(); v != "substType{real: <nil>}" {
		t.Fatal("substType.String:", v)
	}
	for _, typ := range typs {
		func() {
			log.Println("type:", typ.String())
			if fex, ok := typ.(TyFuncEx); ok {
				fex.funcEx()
			}
			if fex, ok := typ.(TyTypeEx); ok {
				fex.typeEx()
			}
			if fex, ok := typ.(iSubstType); ok {
				fex.Obj()
			}
			if fex, ok := typ.(OverloadType); ok {
				fex.Len()
				func() {
					defer func() {
						if e := recover(); e == nil {
							t.Fatal("iOverloadType.At: no error?")
						}
					}()
					fex.At(0)
				}()
			}
			typ.Underlying()
		}()
	}
}

func Test_isReflectType(t *testing.T) {
	if isReflectType(nil) {
		t.Fatal("nil isReflectType?")
	}
}

func TestIsNumeric(t *testing.T) {
	var cb CodeBuilder
	if isNumeric(&cb, nil) {
		t.Fatal("TestIsNumeric: nil isNumeric?")
	}
	pkg := types.NewPackage("", "foo")
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "MyInt", nil), types.Typ[types.Int], nil)
	if !isNumeric(&cb, typ) {
		t.Fatal("TestIsNumeric: MyInt not isNumeric?")
	}
}

func TestStructFieldType(t *testing.T) {
	var pkg = types.NewPackage("", "foo")
	var cb CodeBuilder
	subFlds := []*types.Var{
		types.NewField(token.NoPos, pkg, "val", types.Typ[types.Int], false),
	}
	subStruc := types.NewStruct(subFlds, nil)
	bar := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Bar", nil), subStruc, nil)
	flds := []*types.Var{
		types.NewField(token.NoPos, pkg, "Bar", bar, true),
	}
	struc := types.NewStruct(flds, nil)
	cb.Val(nil)
	if !cb.fieldRef(nil, struc, "val", nil, nil) {
		t.Fatal("structFieldType failed")
	}
}

func TestStructFieldType2(t *testing.T) {
	var pkg = types.NewPackage("", "foo")
	var cb CodeBuilder
	subFlds := []*types.Var{
		types.NewField(token.NoPos, pkg, "val", types.Typ[types.Int], false),
	}
	subStruc := types.NewStruct(subFlds, nil)
	bar := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Bar", nil), subStruc, nil)
	flds := []*types.Var{
		types.NewField(token.NoPos, pkg, "Bar", types.NewPointer(bar), true),
	}
	struc := types.NewStruct(flds, nil)
	cb.Val(nil)
	if !cb.fieldRef(nil, struc, "val", nil, nil) {
		t.Fatal("structFieldType failed")
	}
}

func TestVarDeclEnd(t *testing.T) {
	var decl VarDecl
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestVarDeclEnd failed: no error?")
		}
	}()
	decl.End(nil, nil)
}

func TestCheckParenExpr(t *testing.T) {
	x := checkParenExpr(&ast.CompositeLit{})
	if _, ok := x.(*ast.ParenExpr); !ok {
		t.Fatal("TestCheckParenExpr failed:", x)
	}
	x = checkParenExpr(&ast.SelectorExpr{X: &ast.CompositeLit{}, Sel: ast.NewIdent("sel")})
	if _, ok := x.(*ast.SelectorExpr).X.(*ast.ParenExpr); !ok {
		t.Fatal("TestCheckParenExpr failed:", x)
	}
}

func TestNoFuncName(t *testing.T) {
	var pkg Package
	defer func() {
		if e := recover(); e == nil || e.(string) != "no func name" {
			t.Fatal("TestNoFuncName failed:", e)
		}
	}()
	pkg.NewFuncWith(0, "", nil, nil)
}

func TestGetIdxValTypes(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	cb := pkg.CB()
	intArr := types.NewArray(types.Typ[types.Int], 10)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "intArr", nil), intArr, nil)
	kv, ivKind := cb.getIdxValTypes(typ, false, nil)
	if ivKind == ivTwoValue || kv[0] != types.Typ[types.Int] || kv[1] != types.Typ[types.Int] {
		t.Fatal("TestGetIdxValTypes failed:", kv, ivKind)
	}
}

func Test_checkIteratorFunc(t *testing.T) {
	type testCase struct {
		name     string
		in, out  []types.Type // nil means yield
		yieldIn  []types.Type
		yieldOut []types.Type
		expect   []types.Type
	}
	in := []types.Type{nil}
	tyInt := []types.Type{types.Typ[types.Int]}
	ty3Ints := []types.Type{types.Typ[types.Int], types.Typ[types.Int], types.Typ[types.Int]}
	tyBool := []types.Type{types.Typ[types.Bool]}
	cases := []testCase{
		{name: "func()", expect: nil},
		{name: "func() int", out: tyInt, expect: nil},
		{name: "func(int)", in: tyInt, expect: nil},
		{name: "yield: func()", in: in, expect: nil},
		{name: "yield: func() int", in: in, yieldOut: tyInt, expect: nil},
		{name: "yield: func(int, int, int) bool", in: in, yieldIn: ty3Ints, yieldOut: tyBool, expect: nil},
	}
	for _, c := range cases {
		yieldIn := newParams(c.yieldIn, nil)
		yieldOut := newParams(c.yieldOut, nil)
		yield := types.NewSignatureType(nil, nil, nil, yieldIn, yieldOut, false)
		in := newParams(c.in, yield)
		out := newParams(c.out, nil)
		sig := types.NewSignatureType(nil, nil, nil, in, out, false)
		if ret := checkIteratorFunc(sig); !reflect.DeepEqual(ret, c.expect) {
			t.Fatal("checkIteratorFunc failed:", c.name)
		}
	}
}

func newParams(typs []types.Type, yldt types.Type) *types.Tuple {
	in := make([]*types.Var, len(typs))
	for i, typ := range typs {
		if i == 0 && typ == nil {
			typ = yldt
		}
		in[i] = types.NewParam(0, nil, "", typ)
	}
	return types.NewTuple(in...)
}

func TestGetIdxValTypes2(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	cb := pkg.CB()
	intArr := types.NewArray(types.Typ[types.Int], 10)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "intArr", nil), intArr, nil)
	kv, ivKind := cb.getIdxValTypes(types.NewPointer(typ), false, nil)
	if ivKind == ivTwoValue || kv[0] != types.Typ[types.Int] || kv[1] != types.Typ[types.Int] {
		t.Fatal("TestGetIdxValTypes2 failed:", kv, ivKind)
	}
}

func TestGetStruct(t *testing.T) {
	if getStruct(nil, types.NewPointer(tyInt)) != nil {
		t.Fatal("getStruct failed: not nil?")
	}
}

func TestGetElemType(t *testing.T) {
	cval := constant.MakeFromLiteral("1.1e5", token.FLOAT, 0)
	arg := types.Typ[types.UntypedFloat]
	typ := getElemTypeIf(arg, &internal.Elem{CVal: cval, Type: arg})
	if typ != types.Typ[types.UntypedInt] {
		t.Fatal("getElemTypeIf failed")
	}
	typ = getElemType(&internal.Elem{CVal: cval, Type: arg})
	if typ != types.Typ[types.UntypedInt] {
		t.Fatal("getElemType failed")
	}
}

func getElemType(arg *internal.Elem) types.Type {
	t := arg.Type
	if arg.CVal != nil && t == types.Typ[types.UntypedFloat] {
		if v, ok := constant.Val(arg.CVal).(*big.Rat); ok && v.IsInt() {
			return types.Typ[types.UntypedInt]
		}
	}
	return t
}

func TestBoundElementType(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	elts := []*internal.Elem{
		{Type: types.Typ[types.String]},
		{Type: types.Typ[types.Int]},
	}
	typ := boundElementType(pkg, elts, 0, len(elts), 1)
	if typ != TyEmptyInterface {
		t.Fatal("TestBoundElementType failed:", typ)
	}
}

func TestUnaryOp(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	a := constant.MakeFromLiteral("1e1", token.FLOAT, 0)
	args := []*internal.Elem{
		{CVal: a},
	}
	nega := unaryOp(pkg, token.SUB, args)
	ret := doBinaryOp(nega, token.NEQ, constant.MakeInt64(-10), nil)
	if constant.BoolVal(ret) {
		t.Fatal("TestUnaryOp failed:", nega)
	}
}

func TestUnaryOpXor(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	type testinfo struct {
		typ   types.Type
		value constant.Value
		check constant.Value
	}
	namedUint8 := types.NewNamed(types.NewTypeName(0, nil, "Uint", nil), types.Typ[types.Uint8], nil)
	for _, info := range []testinfo{
		{types.Typ[types.Uint8], constant.MakeInt64(0), constant.MakeUint64(255)},
		{types.Typ[types.Uint16], constant.MakeInt64(1), constant.MakeUint64(65534)},
		{types.Typ[types.Int8], constant.MakeInt64(0), constant.MakeInt64(-1)},
		{types.Typ[types.Int16], constant.MakeInt64(1), constant.MakeInt64(-2)},
		{types.Typ[types.Uint8], constant.MakeInt64(0), constant.MakeUint64(255)},
		{namedUint8, constant.MakeInt64(0), constant.MakeUint64(255)},
	} {
		args := []*internal.Elem{
			{Type: info.typ, CVal: info.value},
		}
		v := unaryOp(pkg, token.XOR, args)
		if !constant.Compare(v, token.EQL, info.check) {
			t.Fatalf("test xor failed: ^%v(%v) result %v, must %v", args[0].Type, info.value, v, info.check)
		}
	}
}

func TestBinaryOp(t *testing.T) {
	a := constant.MakeFromLiteral("1e1", token.FLOAT, 0)
	args := []*internal.Elem{
		{CVal: a},
		{CVal: constant.MakeInt64(3)},
	}
	if cval := binaryOp(nil, token.SHR, args); constant.Val(cval) != int64(1) {
		t.Fatal("binaryOp failed:", cval)
	}
	b := constant.MakeFromLiteral("1e100", token.FLOAT, 0)
	args[1] = &internal.Elem{CVal: b}
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("binaryOp failed: no error?")
		}
	}()
	binaryOp(nil, token.SHR, args)
}

func TestBinaryOp2(t *testing.T) {
	i2 := constant.MakeImag(constant.MakeInt64(2))
	j2 := makeComplex(constant.MakeInt64(0), constant.MakeInt64(2))
	ret := doBinaryOp(i2, token.EQL, j2, nil)
	if !constant.BoolVal(ret) {
		t.Fatal("TestBinaryOp2 failed:", ret)
	}
}

func TestBinaryOpIssue805(t *testing.T) {
	a := constant.MakeInt64(5)
	b := constant.MakeInt64(3)
	c := constant.MakeInt64(1)
	args := []*Element{
		{CVal: a, Type: types.Typ[types.UntypedInt]},
		{CVal: b, Type: types.Typ[types.UntypedInt]},
	}
	a_div_b := binaryOp(nil, token.QUO, args)
	ret := doBinaryOp(a_div_b, token.NEQ, c, nil)
	if constant.BoolVal(ret) {
		t.Fatal("TestBinaryOp failed:", a_div_b, c)
	}
	args2 := []*Element{
		{CVal: a},
		{CVal: b},
	}
	a_div_b2 := binaryOp(nil, token.QUO, args2)
	a_div_b3 := constant.BinaryOp(a, token.QUO, b)
	ret2 := doBinaryOp(a_div_b2, token.NEQ, a_div_b3, nil)
	if constant.BoolVal(ret2) {
		t.Fatal("TestBinaryOp failed:", a_div_b, c)
	}
}

func TestMinConst(t *testing.T) {
	t.Run("TwoInts", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(3),
			constant.MakeInt64(1),
		}
		result := minConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(1)) {
			t.Fatal("minConst failed:", result)
		}
	})

	t.Run("MultipleInts", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(5),
			constant.MakeInt64(2),
			constant.MakeInt64(8),
			constant.MakeInt64(1),
			constant.MakeInt64(9),
		}
		result := minConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(1)) {
			t.Fatal("minConst failed:", result)
		}
	})

	t.Run("Floats", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeFloat64(3.5),
			constant.MakeFloat64(1.2),
			constant.MakeFloat64(2.8),
		}
		result := minConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeFloat64(1.2)) {
			t.Fatal("minConst failed:", result)
		}
	})

	t.Run("Strings", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeString("banana"),
			constant.MakeString("apple"),
			constant.MakeString("cherry"),
		}
		result := minConst(args)
		if constant.StringVal(result) != "apple" {
			t.Fatal("minConst failed:", result)
		}
	})

	t.Run("SingleValue", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(42),
		}
		result := minConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(42)) {
			t.Fatal("minConst failed:", result)
		}
	})

	t.Run("ComplexNotOrderable", func(t *testing.T) {
		c1 := constant.BinaryOp(constant.MakeFloat64(1), token.ADD, constant.MakeImag(constant.MakeFloat64(2)))
		c2 := constant.BinaryOp(constant.MakeFloat64(2), token.ADD, constant.MakeImag(constant.MakeFloat64(3)))
		args := []constant.Value{c1, c2}
		result := minConst(args)
		if result != nil {
			t.Fatal("minConst should return nil for complex args:", result)
		}
	})

	t.Run("BoolNotOrderable", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeBool(true),
			constant.MakeBool(false),
		}
		result := minConst(args)
		if result != nil {
			t.Fatal("minConst should return nil for bool args:", result)
		}
	})
}

func TestMaxConst(t *testing.T) {
	t.Run("TwoInts", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(3),
			constant.MakeInt64(1),
		}
		result := maxConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(3)) {
			t.Fatal("maxConst failed:", result)
		}
	})

	t.Run("MultipleInts", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(5),
			constant.MakeInt64(2),
			constant.MakeInt64(8),
			constant.MakeInt64(1),
			constant.MakeInt64(9),
		}
		result := maxConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(9)) {
			t.Fatal("maxConst failed:", result)
		}
	})

	t.Run("Floats", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeFloat64(3.5),
			constant.MakeFloat64(1.2),
			constant.MakeFloat64(2.8),
		}
		result := maxConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeFloat64(3.5)) {
			t.Fatal("maxConst failed:", result)
		}
	})

	t.Run("Strings", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeString("banana"),
			constant.MakeString("apple"),
			constant.MakeString("cherry"),
		}
		result := maxConst(args)
		if constant.StringVal(result) != "cherry" {
			t.Fatal("maxConst failed:", result)
		}
	})

	t.Run("SingleValue", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeInt64(42),
		}
		result := maxConst(args)
		if !constant.Compare(result, token.EQL, constant.MakeInt64(42)) {
			t.Fatal("maxConst failed:", result)
		}
	})

	t.Run("ComplexNotOrderable", func(t *testing.T) {
		c1 := constant.BinaryOp(constant.MakeFloat64(1), token.ADD, constant.MakeImag(constant.MakeFloat64(2)))
		c2 := constant.BinaryOp(constant.MakeFloat64(2), token.ADD, constant.MakeImag(constant.MakeFloat64(3)))
		args := []constant.Value{c1, c2}
		result := maxConst(args)
		if result != nil {
			t.Fatal("maxConst should return nil for complex args:", result)
		}
	})

	t.Run("BoolNotOrderable", func(t *testing.T) {
		args := []constant.Value{
			constant.MakeBool(true),
			constant.MakeBool(false),
		}
		result := maxConst(args)
		if result != nil {
			t.Fatal("maxConst should return nil for bool args:", result)
		}
	})
}

func TestTryBuiltinCall(t *testing.T) {
	t.Run("Min", func(t *testing.T) {
		fn := &internal.Elem{Val: ident("min")}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
			{CVal: constant.MakeInt64(3)},
		}
		result := tryBuiltinCall(fn, args, true)
		if result == nil || !constant.Compare(result, token.EQL, constant.MakeInt64(3)) {
			t.Fatal("tryBuiltinCall min failed:", result)
		}
	})

	t.Run("Max", func(t *testing.T) {
		fn := &internal.Elem{Val: ident("max")}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
			{CVal: constant.MakeInt64(3)},
		}
		result := tryBuiltinCall(fn, args, true)
		if result == nil || !constant.Compare(result, token.EQL, constant.MakeInt64(5)) {
			t.Fatal("tryBuiltinCall max failed:", result)
		}
	})

	t.Run("UnknownFuncCanfail", func(t *testing.T) {
		fn := &internal.Elem{Val: ident("unknown")}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
		}
		result := tryBuiltinCall(fn, args, true)
		if result != nil {
			t.Fatal("tryBuiltinCall unknown with canfail=true should return nil:", result)
		}
	})

	t.Run("UnknownFuncPanic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("tryBuiltinCall unknown with canfail=false should panic")
			}
		}()
		fn := &internal.Elem{Val: ident("unknown")}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
		}
		tryBuiltinCall(fn, args, false)
	})

	t.Run("NonIdent", func(t *testing.T) {
		fn := &internal.Elem{Val: &ast.SelectorExpr{}}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
		}
		result := tryBuiltinCall(fn, args, true)
		if result != nil {
			t.Fatal("tryBuiltinCall non-ident should return nil:", result)
		}
	})

	t.Run("NilCVal", func(t *testing.T) {
		fn := &internal.Elem{Val: ident("min")}
		args := []*internal.Elem{
			{CVal: constant.MakeInt64(5)},
			{CVal: nil},
		}
		result := tryBuiltinCall(fn, args, true)
		if result != nil {
			t.Fatal("tryBuiltinCall with nil CVal should return nil:", result)
		}
	})

	t.Run("ZeroArgs", func(t *testing.T) {
		fn := &internal.Elem{Val: ident("min")}
		args := []*internal.Elem{}
		result := tryBuiltinCall(fn, args, true)
		if result != nil {
			t.Fatal("tryBuiltinCall with zero args should return nil:", result)
		}
	})

	t.Run("ComplexArgs", func(t *testing.T) {
		c1 := constant.BinaryOp(constant.MakeFloat64(1), token.ADD, constant.MakeImag(constant.MakeFloat64(2)))
		c2 := constant.BinaryOp(constant.MakeFloat64(2), token.ADD, constant.MakeImag(constant.MakeFloat64(3)))
		fn := &internal.Elem{Val: ident("min")}
		args := []*internal.Elem{
			{CVal: c1},
			{CVal: c2},
		}
		result := tryBuiltinCall(fn, args, true)
		if result != nil {
			t.Fatal("tryBuiltinCall with complex args should return nil:", result)
		}
	})
}

func TestUnsafe(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	sizeof := unsafeRef("Sizeof")
	expr := toObjectExpr(pkg, sizeof)
	if v, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := v.X.(*ast.Ident); !ok || id.Name != "unsafe" || v.Sel.Name != "Sizeof" {
			t.Fatal("toObjectExpr failed:", v.X)
		}
	} else {
		t.Fatal("TestUnsafe failed:", expr)
	}
}

func TestLookupFunc(t *testing.T) {
	pkg := types.NewPackage("", "")
	scope := pkg.Scope()
	sig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	methods := []*types.Func{
		types.NewFunc(token.NoPos, pkg, "f", sig),
	}
	intf := types.NewInterfaceType(methods, nil)
	obj := types.NewTypeName(token.NoPos, pkg, "foo", nil)
	types.NewNamed(obj, intf, nil)
	scope.Insert(obj)
	if o := lookupFunc(scope, ".f", "foo"); o == nil {
		t.Fatal("lookupFunc failed: not found?")
	}
}

func TestTryImport(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			t.Fatal("TestTryImport: panic?")
		}
	}()
	pkg := NewPackage("foo", "foo", gblConf)
	if pkg.TryImport("not/exist").Types != nil {
		t.Fatal("TryImport: exist?")
	}
}

func TestUntypeBig(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	big := pkg.Import("github.com/goplus/gogen/internal/builtin")
	big.EnsureImported()
	pkg.utBigInt = big.Ref("XGo_untyped_bigint").Type().(*types.Named)
	pkg.utBigRat = big.Ref("XGo_untyped_bigrat").Type().(*types.Named)
	if ret, ok := untypeBig(pkg, constant.MakeInt64(1), pkg.utBigRat); !ok || ret.Type != pkg.utBigRat {
		t.Fatal("TestUntypeBig failed:", *ret)
	}
	val := constant.Shift(constant.MakeInt64(1), token.SHL, 256)
	if ret, ok := untypeBig(pkg, val, pkg.utBigRat); !ok || ret.Type != pkg.utBigRat {
		t.Fatal("TestUntypeBig failed:", *ret)
	}
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestUntypeBig failed: no error?")
			}
		}()
		untypeBig(pkg, constant.MakeBool(true), pkg.utBigRat)
	}()
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestUntypeBig failed: no error?")
			}
		}()
		untypeBig(pkg, constant.MakeBool(true), pkg.utBigInt)
	}()
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("pkg.Import not-found: no error?")
			}
		}()
		pkg.Import("not-found").EnsureImported()
	}()
}

func TestErrImport(t *testing.T) {
	pkg := NewPackage("github.com/x/foo", "foo", gblConf)
	_, err := importPkg(pkg, "./bar", nil)
	if err == nil || !strings.HasPrefix(err.Error(), "no required module provides package github.com/x/foo/bar;") {
		t.Fatal("importPkg failed:", err)
	}
}

func TestErrWriteFile(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	pkg.Types = nil
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestErrWriteFile: no error?")
		}
	}()
	WriteFile("_xgo_autogen.go", pkg)
}

func TestLoadExpr(t *testing.T) {
	var cb CodeBuilder
	if src, pos, _ := cb.loadExpr(nil); src != "" || pos != token.NoPos {
		t.Fatal("TestLoadExpr failed")
	}
}

func TestRef(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestRef: no error?")
		}
	}()
	pkg := &PkgRef{Types: types.NewPackage("foo", "foo")}
	pkg.Ref("bar")
}

func TestLookupLabel(t *testing.T) {
	var cb CodeBuilder
	if _, ok := cb.LookupLabel("foo"); ok {
		t.Fatal("TestLookupLabel failed")
	}
}

func TestVarVal(t *testing.T) {
	defer func() {
		if e := recover(); !isError(e, "VarVal: variable `unknown` not found") {
			t.Fatal("TestVarVal:", e)
		}
	}()
	var cb CodeBuilder
	cb.VarVal("unknown")
}

func isError(e any, msg string) bool {
	if e != nil {
		if err, ok := e.(error); ok {
			return err.Error() == msg
		}
		if err, ok := e.(string); ok {
			return err == msg
		}
	}
	return false
}

func TestImportError(t *testing.T) {
	err := types.Error{Msg: "foo"}
	e := &ImportError{Err: err}
	if v := e.Unwrap(); v != err {
		t.Fatal("TestImportError2:", v)
	}
}

func TestForRangeStmtPanic(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			t.Fatal("forRangeStmt.End panic")
		}
	}()
	var s forRangeStmt
	s.End(nil, nil)
}

func TestNewFuncDeclPanic(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestNewFuncDeclPanic: not panic")
		}
	}()
	pkg := NewPackage("", "foo", gblConf)
	a := types.NewParam(token.NoPos, pkg.Types, "", types.Typ[types.Int])
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(a), nil, false)
	pkg.NewFuncDecl(token.NoPos, "init", sig)
}

func TestNewFuncPanic(t *testing.T) {
	getRecv(nil, types.NewParam(1, nil, "", types.Typ[types.Int]))
	getRecv(nil, types.NewParam(token.NoPos, nil, "", types.Typ[types.Int]))
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestNewFuncPanic: not panic")
		}
	}()
	pkg := NewPackage("", "foo", gblConf)
	a := types.NewParam(token.NoPos, pkg.Types, "", types.Typ[types.Int])
	pkg.NewFunc(nil, "init", types.NewTuple(a), nil, false)
}

func TestSwitchStmtPanic(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			t.Fatal("siwtchStmt.End panic")
		}
	}()
	var s switchStmt
	s.End(nil, nil)
}

func TestCallIncDec(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestCallIncDec not panic")
		} else if e.(error).Error() != "-: invalid operation: ++ (non-numeric type string)" {
			t.Fatal(e)
		}
	}()
	pkg := NewPackage("", "foo", gblConf)
	if uintptr(pkg.Sizeof(tyInt)) != unsafe.Sizeof(int(0)) {
		t.Fatal("pkg.Sizeof?")
	}
	if len(pkg.Offsetsof(nil)) != 0 {
		t.Fatal("pkg.Offsetsof?")
	}
	args := []*Element{
		{Type: &refType{typ: types.Typ[types.String]}},
	}
	callIncDec(pkg, args, token.INC)
}

func TestTypeAST(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	fset := token.NewFileSet()
	expr := TypeAST(pkg, TyEmptyInterface)
	b := bytes.NewBuffer(nil)
	format.Node(b, fset, expr)
	if b.String() != `interface{}` {
		t.Fatal("TypeAST failed:", b.String())
	}
}

func TestCastFromBool(t *testing.T) {
	ret, ok := CastFromBool(nil, types.Typ[types.Uint], &Element{
		Type: types.Typ[types.UntypedBool],
		CVal: constant.MakeBool(true),
	})
	if !ok || constant.Val(ret.CVal).(int64) != 1 {
		t.Fatal("CastFromBool failed:", ret.CVal, ok)
	}
	ret, ok = CastFromBool(nil, types.Typ[types.Uint], &Element{
		Type: types.Typ[types.Bool],
		CVal: constant.MakeBool(false),
	})
	if !ok || constant.Val(ret.CVal).(int64) != 0 {
		t.Fatal("CastFromBool failed:", ret.CVal, ok)
	}
}

func TestSubstVar(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	a := types.NewParam(0, pkg, "a", types.Typ[types.Int])
	scope := pkg.Scope()
	scope.Insert(NewSubst(token.NoPos, pkg, "bar", a))
	o := Lookup(scope, "bar")
	if o != a {
		t.Fatal("TestSubstVar:", o)
	}
	_, o = LookupParent(scope, "bar", token.NoPos)
	if o != a {
		t.Fatal("TestSubstVar:", o)
	}
	scope.Insert(a)
	_, o2 := LookupParent(scope, "a", token.NoPos)
	if o != o2 {
		t.Fatal("TestSubstVar:", o2)
	}
	o2 = Lookup(scope, "a")
	if o != o2 {
		t.Fatal("TestSubstVar:", o2)
	}
	LookupParent(scope, "b", token.NoPos)
	Lookup(scope, "b")
}

func TestToTag(t *testing.T) {
	if v := toTag(`json:"mytag"`).Value; v != "`json:\"mytag\"`" {
		t.Fatal(v)
	}
	if v := toTag("json:\"mytag\"").Value; v != "`json:\"mytag\"`" {
		t.Fatal(v)
	}
	if v := toTag("json:`mytag`").Value; v != "\"json:`mytag`\"" {
		t.Fatal(v)
	}
}

func TestAssignableUntyped(t *testing.T) {
	f64 := types.NewNamed(types.NewTypeName(token.NoPos, nil, "Float64", nil),
		types.Typ[types.UntypedFloat], nil)
	i64 := types.NewNamed(types.NewTypeName(token.NoPos, nil, "Int64", nil),
		types.Typ[types.UntypedInt], nil)
	if assignableTo(f64, types.Typ[types.UntypedInt], nil) {
		t.Fatal("error f2i")
	}
	if !assignableTo(f64, types.Typ[types.UntypedFloat], nil) {
		t.Fatal("must f2f")
	}
	if !assignableTo(i64, types.Typ[types.UntypedInt], nil) {
		t.Fatal("must i2i")
	}
	if !assignableTo(i64, types.Typ[types.UntypedFloat], nil) {
		t.Fatal("must i2f")
	}
}

func TestValidType(t *testing.T) {
	var errs []error
	conf := &Config{
		HandleErr: func(err error) {
			errs = append(errs, err)
		},
	}
	pkg := NewPackage("", "foo", conf)
	typeA := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "A", nil), nil, nil)
	typeB := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "B", nil), nil, nil)
	typeA.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "B", typeB, true), // Embed B.
	}, nil))
	typeB.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "A", typeA, true), // Embed A.
	}, nil))
	pkg.ValidType(typeA)
	if len(errs) == 0 {
		t.Fatal("TestValidType: no error?")
	}
}

func TestBuiltinRace(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			p := NewPackage("", "foo", nil)
			newBuiltinDefault(p, &Config{})
		}()
	}
	wg.Wait()
}

func TestAliasTypeMethod(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	at := types.NewPackage("foo", "foo")
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	tfoo := foo.Ref("Foo5").Type()
	testcases := []struct {
		Contract
		typ    types.Type
		result bool
	}{
		{addable, types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.Typ[types.Bool], nil), false},
		{addable, tfoo, true},
		{addable, aliasType(pkg, "Foo1", tfoo), true},
		{addable, aliasType(pkg, "Foo2", aliasType(pkg, "Foo3", tfoo)), true},
	}
	for _, c := range testcases {
		if c.Match(pkg, c.typ) != c.result {
			t.Fatalf("%s.Match %v expect %v\n", c.String(), c.typ, c.result)
		}
	}
}

func TestAliasCheckInterface(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	alias := aliasType(pkg, "Any", types.NewInterfaceType(nil, nil))
	if typ, ok := pkg.cb.checkInterface(alias); typ == nil || !ok {
		t.Fatal("TestAliasCheckInterface failed:", typ, ok)
	}
}

func TestAliasUnsigned(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	typ := aliasType(pkg, "Int", types.Typ[types.Uint8])
	if !isUnsigned(typ) {
		t.Fatal("TestAliasUnsigned failed:", typ)
	}
}

func TestAliasContract(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	at := types.NewPackage("foo", "foo")
	foo := pkg.Import("github.com/goplus/gogen/internal/foo")
	tfoo := foo.Ref("Foo5").Type()
	tarr := types.NewArray(tyInt, 10)
	testcases := []struct {
		Contract
		typ    types.Type
		result bool
	}{
		{integer, aliasType(pkg, "Int", tyInt), true},
		{capable, aliasType(pkg, "Bar", types.NewNamed(types.NewTypeName(0, at, "bar", nil), tarr, nil)), true},
		{lenable, aliasType(pkg, "String", types.Typ[types.String]), true},
		{makable, aliasType(pkg, "Map", types.NewMap(tyInt, tyInt)), true},
		{comparable, aliasType(pkg, "Map1", types.NewMap(tyInt, tyInt)), false},
		{comparable, aliasType(pkg, "Chan1", types.NewChan(0, tyInt)), true},
		{addable, aliasType(pkg, "Bar1", types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.Typ[types.Bool], nil)), false},
		{addable, aliasType(pkg, "Foo1", tfoo), true},
		{clearable, aliasType(pkg, "Map2", types.NewMap(tyInt, tyInt)), true},
		{clearable, aliasType(pkg, "Slice1", types.NewSlice(tyInt)), true},
		{clearable, aliasType(pkg, "Bar2", types.NewNamed(types.NewTypeName(0, at, "bar", nil), types.NewSlice(tyInt), nil)), true},
		{clearable, aliasType(pkg, "String1", types.Typ[types.String]), false},
	}
	for _, c := range testcases {
		if c.Match(pkg, c.typ) != c.result {
			t.Fatalf("%s.Match %v expect %v\n", c.String(), c.typ, c.result)
		}
	}
}

func TestAliasIsNumeric(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "MyInt", nil), types.Typ[types.Int], nil)
	if !isNumeric(&pkg.cb, typ) {
		t.Fatal("TestAliasIsNumeric: MyInt not isNumeric?")
	}
	alias := aliasType(pkg, "AliasInt", typ)
	if !isNumeric(&pkg.cb, alias) {
		t.Fatal("TestAliasIsNumeric: AliasInt not isNumeric?")
	}
}

func TestAliasGetStruct(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	st := types.NewStruct(
		[]*types.Var{types.NewField(token.NoPos, pkg.Types, "F", types.Typ[types.Int], false)},
		nil,
	)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "MyStruct", nil), st, nil)
	if st := getStruct(pkg, aliasType(pkg, "Alias", typ)); st == nil || st.NumFields() != 1 {
		t.Fatal("getStruct failed", typ)
	}
}

func TestAliasRecv(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	var instr recvInstr
	elem := &Element{
		Type: aliasType(pkg, "MyChan", types.NewChan(types.SendRecv, types.Typ[types.Int])),
		Val:  ast.NewIdent("ch"),
	}
	_, err := instr.Call(pkg, []*Element{elem}, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAliasOffsetof(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	var instr unsafeOffsetofInstr
	typ := types.NewNamed(
		types.NewTypeName(token.NoPos, pkg.Types, "Point", nil),
		types.NewStruct([]*types.Var{
			types.NewField(token.NoPos, pkg.Types, "X", types.Typ[types.Int], false),
			types.NewField(token.NoPos, pkg.Types, "Y", types.Typ[types.Int], false),
		}, nil),
		nil,
	)
	aliasTyp := aliasType(pkg, "MyPoint", typ)
	styp := types.NewNamed(
		types.NewTypeName(token.NoPos, pkg.Types, "Rect", nil),
		types.NewStruct([]*types.Var{
			types.NewField(token.NoPos, pkg.Types, "", aliasTyp, true),
			types.NewField(token.NoPos, pkg.Types, "Width", types.Typ[types.Int], false),
			types.NewField(token.NoPos, pkg.Types, "Height", types.Typ[types.Int], false),
		}, nil),
		nil,
	)
	elem := &Element{
		Type: types.Typ[types.Int],
		Val: selector(&Element{
			Type: aliasType(pkg, "MyRect", styp),
			Val:  ast.NewIdent("rect"),
		}, "Y"),
	}
	_, err := instr.Call(pkg, []*Element{elem}, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCodeBuilderAliasIndex(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  func(*Package, types.Type) types.Type
		key  any
	}{
		{"Slice", func(_ *Package, elem types.Type) types.Type { return types.NewSlice(elem) }, 0},
		{"Map", func(_ *Package, elem types.Type) types.Type { return types.NewMap(tyInt, elem) }, 0},
		{"Array", func(_ *Package, elem types.Type) types.Type { return types.NewArray(elem, 2) }, 0},
		{"String", func(_ *Package, _ types.Type) types.Type { return types.Typ[types.String] }, 0},
		{"PointerArray", func(_ *Package, elem types.Type) types.Type {
			return types.NewPointer(types.NewArray(elem, 2))
		}, 0},
		{"PointerNamedArray", func(pkg *Package, elem types.Type) types.Type {
			return types.NewPointer(pkg.NewType("Array").InitType(pkg, types.NewArray(elem, 2)))
		}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			elem := aliasType(pkg, "Item", tyInt)
			typ := aliasType(pkg, "Values", aliasType(pkg, "Inner", tt.typ(pkg, elem)))
			v := types.NewVar(token.NoPos, pkg.Types, "values", typ)
			cb := pkg.NewFunc(nil, "f", types.NewTuple(v), nil, false).BodyStart(pkg)
			cb.Val(v).Val(tt.key).Index(1, 1)
			want := elem
			if tt.name == "String" {
				want = TyByte
			}
			if got := cb.stk.Pop().Type; got != want {
				t.Errorf("index type = %v, want %v", got, want)
			}
			if tt.name != "String" {
				cb.Val(v).Val(tt.key).IndexRef(1).Val(1).Assign(1)
			}
			if tt.name == "Map" {
				cb.Val(v).Val(tt.key).Index(1, 2)
				got, ok := cb.stk.Pop().Type.(*types.Tuple)
				if !ok || got.Len() != 2 || got.At(0).Type() != elem || got.At(1).Type() != types.Typ[types.Bool] {
					t.Errorf("comma-ok index type = %v, want (%v, bool)", got, elem)
				}
			}
			cb.End()
		})
	}
}

func TestCodeBuilderAliasIndirect(t *testing.T) {
	for _, name := range []string{"Star", "Elem", "ElemRef"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			elem := aliasType(pkg, "Item", tyInt)
			typ := aliasType(pkg, "Pointer", types.NewPointer(elem))
			v := types.NewVar(token.NoPos, pkg.Types, "ptr", typ)
			cb := pkg.CB().Val(v)
			switch name {
			case "Star":
				cb.Star()
			case "Elem":
				cb.Elem()
			case "ElemRef":
				cb.ElemRef()
			}
			got, ref := DerefType(cb.Get(-1).Type)
			if got != elem || ref != (name == "ElemRef") {
				t.Errorf("indirect type = %v, ref = %v, want %v", got, ref, elem)
			}
		})
	}
}

func TestCodeBuilderAliasRange(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  func(*Package, types.Type) types.Type
	}{
		{"Slice", func(_ *Package, elem types.Type) types.Type { return types.NewSlice(elem) }},
		{"Map", func(_ *Package, elem types.Type) types.Type { return types.NewMap(elem, elem) }},
		{"Array", func(_ *Package, elem types.Type) types.Type { return types.NewArray(elem, 2) }},
		{"PointerArray", func(_ *Package, elem types.Type) types.Type {
			return types.NewPointer(types.NewArray(elem, 2))
		}},
		{"Chan", func(_ *Package, elem types.Type) types.Type { return types.NewChan(types.RecvOnly, elem) }},
		{"String", func(_ *Package, _ types.Type) types.Type { return types.Typ[types.String] }},
		{"Integer", func(_ *Package, _ types.Type) types.Type { return tyInt }},
		{"Function", func(pkg *Package, elem types.Type) types.Type {
			params := types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", tyInt),
				types.NewParam(token.NoPos, pkg.Types, "", elem))
			result := types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", types.Typ[types.Bool]))
			yield := types.NewSignatureType(nil, nil, nil, params, result, false)
			return types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "yield", yield)), nil, false)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			elem := aliasType(pkg, "Item", tyInt)
			alias := aliasType(pkg, "Values", tt.typ(pkg, elem))
			v := types.NewVar(token.NoPos, pkg.Types, "values", alias)
			cb := pkg.NewFunc(nil, "f", types.NewTuple(v), nil, false).BodyStart(pkg)
			want := []types.Type{tyInt, elem}
			switch tt.name {
			case "Map":
				want[0] = elem
			case "Chan":
				want = []types.Type{elem}
			case "String":
				want[1] = types.Typ[types.Rune]
			case "Integer":
				want = []types.Type{tyInt}
			}
			names := []string{"i", "v"}[:len(want)]
			cb.ForRange(names...).Val(v).RangeAssignThen(token.NoPos)
			for i, name := range names {
				if got := cb.Scope().Lookup(name).Type(); got != want[i] {
					t.Errorf("range variable %s type = %v, want %v", name, got, want[i])
				}
			}
			cb.End().End()
		})
	}
}

func TestCodeBuilderAliasLenCap(t *testing.T) {
	for _, name := range []string{"String", "Array", "PointerArray"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			var typ types.Type = types.NewArray(tyInt, 2)
			if name == "PointerArray" {
				typ = types.NewPointer(typ)
			}
			if name == "String" {
				typ = types.Typ[types.String]
			}
			typ = aliasType(pkg, "Alias", typ)
			var value types.Object = types.NewVar(token.NoPos, pkg.Types, "value", typ)
			if name == "String" {
				value = types.NewConst(token.NoPos, pkg.Types, "value", typ, constant.MakeString("hi"))
			}
			for _, builtin := range []string{"len", "cap"} {
				if name == "String" && builtin == "cap" {
					continue
				}
				ret := pkg.ConstStart().Val(pkg.Builtin().Ref(builtin)).Val(value).Call(1).EndConst()
				if ret.CVal == nil || ret.CVal.ExactString() != "2" {
					t.Errorf("%s constant value = %v, want 2", builtin, ret.CVal)
				}
			}
		})
	}
}

func TestCodeBuilderPointerAliasArray(t *testing.T) {
	for _, name := range []string{"Alias", "AliasChain", "PointerAlias", "GenericAlias"} {
		for _, op := range []string{"Index", "IndexRef", "Range", "Len", "Cap"} {
			t.Run(name+"/"+op, func(t *testing.T) {
				pkg := NewPackage("", "foo", gblConf)
				elem := aliasType(pkg, "Item", tyInt)
				var array types.Type
				if name == "GenericAlias" {
					tp := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg.Types, "T", nil), TyEmptyInterface)
					generic := types.NewAlias(types.NewTypeName(token.NoPos, pkg.Types, "Array", nil), types.NewArray(tp, 2))
					generic.SetTypeParams([]*types.TypeParam{tp})
					var err error
					array, err = types.Instantiate(nil, generic, []types.Type{elem}, true)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					array = aliasType(pkg, "Array", types.NewArray(elem, 2))
					if name == "AliasChain" {
						array = aliasType(pkg, "Next", array)
					}
				}
				var typ types.Type = types.NewPointer(array)
				if name == "PointerAlias" {
					typ = aliasType(pkg, "Pointer", typ)
				}
				value := types.NewVar(token.NoPos, pkg.Types, "value", typ)
				cb := pkg.NewFunc(nil, "f", types.NewTuple(value), nil, false).BodyStart(pkg)
				cb.Val(value)
				if got := cb.stk.Pop().Type; got != typ {
					t.Fatalf("pointer type = %v, want preserved type %v", got, typ)
				}
				switch op {
				case "Index":
					cb.Val(value).Val(0).Index(1, 1)
					if got := cb.stk.Pop().Type; got != elem {
						t.Errorf("index type = %v, want %v", got, elem)
					}
				case "IndexRef":
					cb.Val(value).Val(0).IndexRef(1)
					if got, ref := DerefType(cb.Get(-1).Type); got != elem || !ref {
						t.Errorf("index reference type = %v, ref = %v, want %v", got, ref, elem)
					}
					cb.Val(1).Assign(1)
				case "Range":
					cb.ForRange("i", "v").Val(value).RangeAssignThen(token.NoPos)
					for i, name := range []string{"i", "v"} {
						want := []types.Type{tyInt, elem}[i]
						if got := cb.Scope().Lookup(name).Type(); got != want {
							t.Errorf("range variable %s type = %v, want %v", name, got, want)
						}
					}
					cb.End()
				case "Len", "Cap":
					builtin := strings.ToLower(op)
					cb.Val(pkg.Builtin().Ref(builtin)).Val(value).Call(1)
					ret := cb.stk.Pop()
					if ret.CVal == nil || ret.CVal.ExactString() != "2" {
						t.Errorf("%s constant value = %v, want 2", builtin, ret.CVal)
					}
					if ret.Type != tyInt {
						t.Errorf("%s type = %v, want int", builtin, ret.Type)
					}
				}
				cb.End()
			})
		}
	}
}

func TestCodeBuilderAliasSlice(t *testing.T) {
	for _, name := range []string{"Slice", "Array", "PointerArray", "PointerAliasArray", "String"} {
		for _, mode := range []string{"TwoIndex", "ThreeIndex"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				full := mode == "ThreeIndex"
				pkg := NewPackage("", "foo", gblConf)
				elem := aliasType(pkg, "Item", tyInt)
				var typ types.Type = types.NewArray(elem, 2)
				switch name {
				case "Slice":
					typ = types.NewSlice(elem)
				case "PointerArray":
					typ = types.NewPointer(typ)
				case "PointerAliasArray":
					typ = types.NewPointer(aliasType(pkg, "Array", typ))
				case "String":
					typ = types.Typ[types.String]
				}
				typ = aliasType(pkg, "Alias", typ)
				cb := pkg.CB().Val(types.NewVar(token.NoPos, pkg.Types, "values", typ)).Val(0).Val(1)
				if full {
					cb.Val(2)
				}
				if name == "String" && full {
					defer func() {
						err, ok := recover().(*CodeError)
						if !ok || !strings.Contains(err.Msg, "3-index slice of string") {
							t.Errorf("full string slice panic = %v, want CodeError", err)
						}
					}()
				}
				cb.Slice(full)
				want := typ
				if name != "Slice" && name != "String" {
					want = types.NewSlice(elem)
				}
				if got := cb.Get(-1).Type; !types.Identical(got, want) || got.String() != want.String() {
					t.Errorf("slice type = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestCodeBuilderAliasMapMember(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	key := aliasType(pkg, "Key", types.Typ[types.String])
	elem := aliasType(pkg, "Item", tyInt)
	v := types.NewVar(token.NoPos, pkg.Types, "values", types.NewMap(key, elem))
	cb := pkg.NewFunc(nil, "f", types.NewTuple(v), nil, false).BodyStart(pkg)
	cb.Val(v).MemberVal("item", 0)
	if got := cb.stk.Pop().Type; got != elem {
		t.Errorf("member type = %v, want %v", got, elem)
	}
	cb.Val(v).MemberRef("item").Val(1).Assign(1).End()
}

func TestComparableToAlias(t *testing.T) {
	for _, name := range []string{"Slice", "Map", "Pointer", "Channel", "Function", "Interface", "Integer"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			var typ types.Type
			switch name {
			case "Slice":
				typ = types.NewSlice(tyInt)
			case "Map":
				typ = types.NewMap(tyInt, tyInt)
			case "Pointer":
				typ = types.NewPointer(tyInt)
			case "Channel":
				typ = types.NewChan(types.SendRecv, tyInt)
			case "Function":
				typ = types.NewSignatureType(nil, nil, nil, nil, nil, false)
			case "Interface":
				typ = TyEmptyInterface
			case "Integer":
				typ = tyInt
			}
			alias := aliasType(pkg, "Alias", typ)
			value := &Element{Type: alias}
			nilValue := &Element{Type: types.Typ[types.UntypedNil]}
			want := name != "Integer"
			if got := ComparableTo(pkg, value, nilValue); got != want {
				t.Errorf("alias compared to nil = %v, want %v", got, want)
			}
			if got := ComparableTo(pkg, nilValue, value); got != want {
				t.Errorf("nil compared to alias = %v, want %v", got, want)
			}
		})
	}
}

func TestCheckIteratorFuncAlias(t *testing.T) {
	for _, name := range []string{"AliasBool", "AliasDefinedBool"} {
		t.Run(name, func(t *testing.T) {
			named := name == "AliasDefinedBool"
			pkg := NewPackage("", "foo", gblConf)
			var result types.Type = types.Typ[types.Bool]
			if named {
				result = pkg.NewType("Boolean").InitType(pkg, result)
			}
			result = aliasType(pkg, "Bool", result)
			yield := types.NewSignatureType(nil, nil, nil, nil,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", result)), false)
			sig := types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "yield", yield)), nil, false)
			got := checkIteratorFunc(sig)
			if named {
				if got != nil {
					t.Errorf("iterator with defined bool result accepted: %v", got)
				}
			} else if len(got) != 2 || got[0] != nil || got[1] != nil {
				t.Errorf("zero-value iterator types = %v, want [nil nil]", got)
			}
		})
	}
}

func TestCodeBuilderAliasReturnErr(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	alias := aliasType(pkg, "Err", TyError)
	results := types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", tyInt),
		types.NewParam(token.NoPos, pkg.Types, "", alias))
	pkg.NewFunc(nil, "f", nil, results, false).BodyStart(pkg).Val(nil).ReturnErr(false).End()
}

func TestCodeBuilderAliasUnsafe(t *testing.T) {
	for _, name := range []string{"Slice", "String", "SliceData", "StringData", "Add"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			elem := aliasType(pkg, "Byte", types.Typ[types.Byte])
			var typ, want types.Type
			switch name {
			case "Slice":
				typ, want = types.NewPointer(elem), types.NewSlice(elem)
			case "String":
				typ, want = types.NewPointer(elem), types.Typ[types.String]
			case "SliceData":
				typ, want = types.NewSlice(elem), types.NewPointer(elem)
			case "StringData":
				typ, want = types.Typ[types.String], types.NewPointer(types.Typ[types.Byte])
			case "Add":
				typ, want = types.Typ[types.UnsafePointer], types.Typ[types.UnsafePointer]
			}
			value := types.NewVar(token.NoPos, pkg.Types, "value", aliasType(pkg, "Alias", typ))
			cb := pkg.CB().Val(pkg.unsafe_.Ref(name)).Val(value)
			arity := 1
			if name == "Slice" || name == "String" || name == "Add" {
				cb.Val(1)
				arity++
			}
			cb.Call(arity)
			if got := cb.Get(-1).Type; !types.Identical(got, want) || got.String() != want.String() {
				t.Errorf("unsafe.%s type = %v, want %v", name, got, want)
			}
		})
	}
}

func TestCodeBuilderAliasOperators(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pointer bool
		binary  bool
		right   bool
	}{
		{name: "Unary"},
		{name: "PointerUnary", pointer: true},
		{name: "Binary", binary: true},
		{name: "PointerBinary", pointer: true, binary: true},
		{name: "RightBinary", binary: true, right: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			var underlying types.Type = types.NewStruct(nil, nil)
			if tt.right {
				underlying = tyInt
			}
			named := pkg.NewType("Value").InitType(pkg, underlying)
			var recv types.Type = named
			if tt.pointer {
				recv = types.NewPointer(recv)
			}
			name := "XGo_Neg"
			var params *types.Tuple
			if tt.binary {
				name = "XGo_Add"
				params = types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "other", recv))
			}
			result := aliasType(pkg, "Result", tyInt)
			sig := types.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "v", recv), nil, nil, params,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", result)), false)
			named.AddMethod(types.NewFunc(token.NoPos, pkg.Types, name, sig))
			value := types.NewVar(token.NoPos, pkg.Types, "value", aliasType(pkg, "Alias", recv))
			cb := pkg.CB()
			if tt.right {
				cb.Val(1)
			} else {
				cb.Val(value)
			}
			if tt.binary {
				cb.Val(value).BinaryOp(token.ADD)
			} else {
				cb.UnaryOp(token.SUB)
			}
			if got := cb.Get(-1).Type; got != result {
				t.Errorf("operator type = %v, want %v", got, result)
			}
		})
	}
}

func TestCodeBuilderAliasAppendString(t *testing.T) {
	for _, name := range []string{"Slice", "String", "Byte", "Both"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			var elem types.Type = types.Typ[types.Byte]
			if name == "Byte" {
				elem = aliasType(pkg, "Byte", elem)
			}
			var slice types.Type = types.NewSlice(elem)
			if name == "Slice" || name == "Both" {
				slice = aliasType(pkg, "Bytes", slice)
			}
			var str types.Type = types.Typ[types.String]
			if name == "String" || name == "Both" {
				str = aliasType(pkg, "Text", str)
			}
			values := types.NewVar(token.NoPos, pkg.Types, "values", slice)
			text := types.NewVar(token.NoPos, pkg.Types, "text", str)
			cb := pkg.CB().Val(pkg.Builtin().Ref("append")).Val(values).Val(text).CallWith(2, 0, InstrFlagEllipsis)
			if got := cb.Get(-1).Type; got != slice {
				t.Errorf("append type = %v, want %v", got, slice)
			}
		})
	}
}

func TestCodeBuilderAliasTemplateRecv(t *testing.T) {
	for _, name := range []string{"Pointer", "AliasPointer"} {
		t.Run(name, func(t *testing.T) {
			pkg := NewPackage("", "foo", gblConf)
			named := pkg.NewType("Value").InitType(pkg, types.NewStruct(nil, nil))
			ptr := types.NewPointer(named)
			sig := types.NewSignatureType(nil, nil, nil,
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "value", types.NewPointer(ptr))),
				types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", tyInt)), false)
			extension := types.NewFunc(token.NoPos, pkg.Types, "Extension", sig)
			NewTemplateRecvMethod(named, token.NoPos, pkg.Types, "Run", extension)
			var typ types.Type = ptr
			if name == "AliasPointer" {
				typ = aliasType(pkg, "Ref", ptr)
			}
			value := types.NewVar(token.NoPos, pkg.Types, "value", typ)
			cb := pkg.CB().Val(value).MemberVal("Run", 0)
			if err := cb.CallWithEx(0, 0, 0); err == nil {
				t.Error("template method unexpectedly took the address of a pointer")
			}
		})
	}
}

func TestCodeBuilderAliasRcast(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	named := pkg.NewType("Value").InitType(pkg, types.NewStruct(nil, nil))
	sig := types.NewSignatureType(types.NewVar(token.NoPos, pkg.Types, "v", named), nil, nil, nil,
		types.NewTuple(types.NewParam(token.NoPos, pkg.Types, "", tyInt)), false)
	named.AddMethod(types.NewFunc(token.NoPos, pkg.Types, "XGo_Rcast", sig))
	value := types.NewVar(token.NoPos, pkg.Types, "value", aliasType(pkg, "Alias", named))
	cb := pkg.CB().Typ(tyInt).Val(value).Call(1)
	call, ok := cb.Get(-1).Val.(*ast.CallExpr)
	if !ok {
		t.Fatalf("cast expression = %T, want call", cb.Get(-1).Val)
	}
	fn, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || fn.Sel.Name != "XGo_Rcast" {
		t.Errorf("cast function = %v, want value.XGo_Rcast", call.Fun)
	}
}

func TestCodeBuilderAliasDelayedType(t *testing.T) {
	conf := *gblConf
	conf.LoadNamed = func(_ *Package, typ *types.Named) {
		typ.SetUnderlying(types.Typ[types.Int])
	}
	pkg := NewPackage("", "foo", &conf)
	named := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "Value", nil), nil, nil)
	typ := aliasType(pkg, "Alias", named)
	param := types.NewParam(token.NoPos, pkg.Types, "value", typ)
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(param), types.NewTuple(param), false)
	fn := types.NewFunc(token.NoPos, pkg.Types, "f", sig)
	cb := pkg.CB().Val(fn).Val(types.NewVar(token.NoPos, pkg.Types, "value", typ)).Call(1)
	if named.Underlying() == nil {
		t.Error("aliased named type was not loaded")
	}
	if got := cb.Get(-1).Type; got != typ {
		t.Errorf("call type = %v, want %v", got, typ)
	}
}

func TestCodeBuilderInvalidAliasOperations(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  types.Type
		run  func(*CodeBuilder)
		want string
	}{
		{"Call", tyInt, func(cb *CodeBuilder) { cb.Call(0) }, "cannot call non-function"},
		{"Index", types.Typ[types.Bool], func(cb *CodeBuilder) { cb.Val(0).Index(1, 1) }, "does not support indexing"},
		{"IndexRef", types.Typ[types.String], func(cb *CodeBuilder) { cb.Val(0).IndexRef(1) }, "strings are immutable"},
		{"Star", tyInt, func(cb *CodeBuilder) { cb.Star() }, "invalid indirect"},
		{"Elem", tyInt, func(cb *CodeBuilder) { cb.Elem() }, "invalid indirect"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				got := recover()
				if err, ok := got.(*CodeError); !ok || !strings.Contains(err.Msg, tt.want) {
					t.Errorf("panic = %v, want CodeError containing %q", got, tt.want)
				}
			}()
			pkg := NewPackage("", "foo", gblConf)
			value := types.NewVar(token.NoPos, pkg.Types, "value", aliasType(pkg, "Alias", tt.typ))
			tt.run(pkg.CB().Val(value))
		})
	}
}

func TestAliasBasic(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	aliasTyp := aliasType(pkg, "MyInt", types.Typ[types.Int])
	elem := &Element{
		Type: aliasTyp,
		Val:  ast.NewIdent("v"),
	}
	if b := isBasicKind(&pkg.cb, elem, types.IsInteger); b != true {
		t.Fatal("failed")
	}
}

// ----------------------------------------------------------------------------
