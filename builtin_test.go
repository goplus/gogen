/*
 Copyright 2021 The GoPlus Authors (goplus.org)
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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/goplus/gogen/internal"
	"github.com/goplus/gogen/internal/go/format"
	"github.com/goplus/gogen/internal/typesutil"
	"github.com/goplus/gogen/packages"
)

var (
	gblConf = getConf()
)

func init() {
	debugImportIox = true
}

func getConf() *Config {
	fset := token.NewFileSet()
	imp := packages.NewImporter(fset)
	return &Config{Fset: fset, Importer: imp}
}

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

func TestCheckGopDeps(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	file := pkg.CurFile()
	id := file.newImport("env", "github.com/goplus/gop/env")
	file.forceImport("github.com/qiniu/x/errors")
	file.getDecls(pkg)
	if v := file.CheckGopDeps(pkg); v != FlagDepModX {
		t.Fatal("CheckGopDeps:", v)
	}
	id.Obj.Data = importUsed(true)
	if v := file.CheckGopDeps(pkg); v != FlagDepModGop|FlagDepModX {
		t.Fatal("CheckGopDeps:", v)
	}
}

func TestInitGopPkg(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	pkg.Types.Scope().Insert(types.NewVar(
		token.NoPos, pkg.Types, "GopPackage", types.Typ[types.Bool],
	))
	pkg.initGopPkg(nil, pkg.Types)
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

func TestCheckGopPkgNoop(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	pkg.Types.Scope().Insert(types.NewConst(
		token.NoPos, pkg.Types, "GopPackage", types.Typ[types.UntypedBool], constant.MakeBool(true),
	))
	if _, ok := checkGopPkg(pkg); ok {
		t.Fatal("checkGopPkg: ok?")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expDeps.typ: no panic?")
		}
	}()
	var ed expDeps
	tyParam := types.NewTypeParam(types.NewTypeName(0, pkg.Types, "T", nil), TyEmptyInterface)
	ed.typ(tyParam)
	ed.typ(types.NewUnion([]*types.Term{types.NewTerm(false, tyParam)}))
	ed.typ(&unboundFuncParam{})
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
	_, err := matchOverloadNamedTypeCast(pkg, tn, nil, nil, 0)
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
		defer func() {
			if e := recover(); e != "overload func foo__1 out of range 0..0\n" {
				t.Fatal("TestOverloadFuncs:", e)
			}
		}()
		overloadFuncs(5, []types.Object{fn})
	}()
	func() {
		defer func() {
			if e := recover(); e != "overload func foo__1 exists?\n" {
				t.Fatal("TestOverloadFuncs:", e)
			}
		}()
		overloadFuncs(5, []types.Object{fn, fn})
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
	if sig == nil || sig.Results().Len() != 1 {
		t.Fatal("string.Index (Results):", sig)
	}
	if tsig := tiStr.lookupByName("__unknown"); tsig != nil {
		t.Fatal("tsig:", tsig)
	}
}

func TestFindMethodType(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	tyFile := pkg.Import("os").Ref("File").Type().(*types.Named)
	sig := findMethodType(&pkg.cb, tyFile, "Gop_Enum")
	if sig == nil || sig.Params().Len() != 0 {
		t.Fatal("os.File.GopEnum (Params):", sig)
	}
	if sig == nil || sig.Results().Len() != 1 {
		t.Fatal("os.File.GopEnum (Results):", sig)
	}
}

func TestContractName(t *testing.T) {
	testcases := []struct {
		Contract
		name string
	}{
		{any, "any"},
		{capable, "capable"},
		{lenable, "lenable"},
		{makable, "makable"},
		{cbool, "bool"},
		{ninteger, "ninteger"},
		{orderable, "orderable"},
		{integer, "integer"},
		{number, "number"},
		{addable, "addable"},
		{comparable, "comparable"},
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
	tfoo := foo.Ref("Foo").Type()
	tarr := types.NewArray(tyInt, 10)
	testcases := []struct {
		Contract
		typ    types.Type
		result bool
	}{
		{integer, tyInt, true},
		{capable, types.Typ[types.String], false},
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
	if caller, pos := getFunExpr(nil); caller != "the closure call" || pos != token.NoPos {
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
	pkg.files[""] = &File{decls: []ast.Decl{
		&ast.GenDecl{Specs: []ast.Spec{
			&ast.ValueSpec{Type: &ast.Ident{}},
		}},
		nil,
	}}
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

func TestToVariadic(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestToVariadic: no error?")
		}
	}()
	toVariadic(&ast.Field{Type: &ast.Ident{Name: "int"}})
}

func TestToType(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	toType(pkg, &unboundType{tBound: tyInt})
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestToType: no error?")
		}
	}()
	toType(pkg, &unboundType{})
}

func isLeastGo122() bool {
	ver, err := strconv.ParseInt(runtime.Version()[4:6], 10, 0)
	return err == nil && ver >= 22
}

func TestToTypeAlias(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	alias := typesutil.NewAlias(types.NewTypeName(token.NoPos, nil, "Int", nil), types.Typ[types.Int])
	if isLeastGo122() {
		expr := toType(pkg, alias)
		if ident, ok := expr.(*ast.Ident); !ok || ident.Name != "Int" {
			t.Fatalf("bad alias %#v", expr)
		}
	} else {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestToTypeAlias: no error?")
			}
		}()
		toType(pkg, alias)
	}
}

/*
func typString(pkg *Package, t types.Type) string {
	v := toType(pkg, t)
	var b bytes.Buffer
	err := format.Node(&b, pkg.Fset, v)
	if err != nil {
		panic(err)
	}
	return b.String()
}
*/

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
		&unboundType{},
		&unboundMapElemType{},
		&TyOverloadFunc{},
		&TyOverloadMethod{},
		&TyStaticMethod{},
		&TyTemplateRecvMethod{},
		&TyInstruction{},
		&TyOverloadNamed{Obj: types.NewTypeName(0, pkg.Types, "bar", tyInt)},
		&TypeType{},
		&tyTypeAsParams{},
		&unboundFuncParam{},
		&unboundProxyParam{},
		&TemplateParamType{},
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
	bte := &boundTypeError{tyInt, TyByte}
	if bte.Error() != "boundType int => byte failed" {
		t.Fatal("boundTypeError:", bte)
	}
	ut := &unboundType{tBound: tyInt}
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("unboundType.boundTo: no error?")
		}
	}()
	ut.boundTo(pkg, TyByte)
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
	if !cb.fieldRef(nil, struc, "val", nil) {
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
	if !cb.fieldRef(nil, struc, "val", nil) {
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
	kv, allowTwoValue := cb.getIdxValTypes(typ, false, nil)
	if allowTwoValue || kv[0] != types.Typ[types.Int] || kv[1] != types.Typ[types.Int] {
		t.Fatal("TestGetIdxValTypes failed:", kv, allowTwoValue)
	}
}

func TestGetIdxValTypes2(t *testing.T) {
	pkg := NewPackage("", "foo", gblConf)
	cb := pkg.CB()
	intArr := types.NewArray(types.Typ[types.Int], 10)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "intArr", nil), intArr, nil)
	kv, allowTwoValue := cb.getIdxValTypes(types.NewPointer(typ), false, nil)
	if allowTwoValue || kv[0] != types.Typ[types.Int] || kv[1] != types.Typ[types.Int] {
		t.Fatal("TestGetIdxValTypes2 failed:", kv, allowTwoValue)
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
	ret := doBinaryOp(nega, token.NEQ, constant.MakeInt64(-10))
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
	ret := doBinaryOp(i2, token.EQL, j2)
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
	ret := doBinaryOp(a_div_b, token.NEQ, c)
	if constant.BoolVal(ret) {
		t.Fatal("TestBinaryOp failed:", a_div_b, c)
	}
	args2 := []*Element{
		{CVal: a},
		{CVal: b},
	}
	a_div_b2 := binaryOp(nil, token.QUO, args2)
	a_div_b3 := constant.BinaryOp(a, token.QUO, b)
	ret2 := doBinaryOp(a_div_b2, token.NEQ, a_div_b3)
	if constant.BoolVal(ret2) {
		t.Fatal("TestBinaryOp failed:", a_div_b, c)
	}
}

func TestBuiltinCall(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestBuiltinCall: no error?")
		}
	}()
	builtinCall(&internal.Elem{Val: ident("undefined")}, nil)
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
	pkg.utBigInt = big.Ref("Gop_untyped_bigint").Type().(*types.Named)
	pkg.utBigRat = big.Ref("Gop_untyped_bigrat").Type().(*types.Named)
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

func TestIsUnbound(t *testing.T) {
	if !isUnboundTuple(types.NewTuple(types.NewParam(token.NoPos, nil, "", &unboundFuncParam{}))) {
		t.Fatal("TestIsUnbound failed")
	}
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
	WriteFile("_gop_autogen.go", pkg)
}

func TestLoadExpr(t *testing.T) {
	var cb CodeBuilder
	if src, pos := cb.loadExpr(nil); src != "" || pos != token.NoPos {
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
		if e := recover(); !isError(e, "VarVal: variable `unknown` not found\n") {
			t.Fatal("TestVarVal:", e)
		}
	}()
	var cb CodeBuilder
	cb.VarVal("unknown")
}

func isError(e interface{}, msg string) bool {
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
	err := &types.Error{Msg: "foo"}
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
	getRecv(nil)
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

// ----------------------------------------------------------------------------
