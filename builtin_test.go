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

package gox

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"testing"

	"github.com/goplus/gox/internal"
	"github.com/goplus/gox/packages"
)

var (
	gblConf = getConf()
)

func getConf() *Config {
	fset := token.NewFileSet()
	conf := &packages.Config{
		ModPath: "github.com/goplus/gox",
		Loaded:  make(map[string]*types.Package),
		Fset:    fset,
	}
	imp, _, err := packages.NewImporter(
		conf, ".", "github.com/goplus/gox/internal/builtin", "github.com/goplus/gox/internal/foo")
	if err != nil {
		panic(err)
	}
	return &Config{Fset: fset, Importer: imp}
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
	pkg := NewPackage("", "foo", gblConf)
	at := types.NewPackage("foo", "foo")
	foo := pkg.Import("github.com/goplus/gox/internal/foo")
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
		{comparable, types.NewSignature(nil, nil, nil, false), false},
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
		types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
	}
	methods2 := []*types.Func{
		types.NewFunc(token.NoPos, pkg.Types, "F", types.NewSignature(nil, nil, nil, false)),
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
		t.Fatal("gox.Default failed")
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
	sig := types.NewSignature(nil, nil, nil, false)
	if _, ok := CheckOverloadMethod(sig); ok {
		t.Fatal("TestCheckOverloadMethod failed:")
	}
}

func TestIsFunc(t *testing.T) {
	if IsFunc(nil) {
		t.Fatal("nil is func?")
	}
	if !IsFunc(types.NewSignature(nil, nil, nil, false)) {
		t.Fatal("func() is not func?")
	}
	if HasAutoProperty(nil) {
		t.Fatal("nil has autoprop?")
	}
	if !HasAutoProperty(types.NewSignature(nil, nil, nil, false)) {
		t.Fatal("func() has not autoprop?")
	}
}

func TestCheckUdt(t *testing.T) {
	o := types.NewNamed(types.NewTypeName(token.NoPos, nil, "foo", nil), types.Typ[types.Int], nil)
	var frs forRangeStmt
	if _, ok := frs.checkUdt(o); ok {
		t.Fatal("findMethod failed: bar exists?")
	}
}

func TestNodeInterp(t *testing.T) {
	interp := nodeInterp{}
	if pos := interp.Position(1); pos.Line != 0 {
		t.Fatal("TestNodeInterp interp.Position failed:", pos)
	}
	if interp.Caller(nil) != "the function call" {
		t.Fatal("TestNodeInterp interp.Caller failed")
	}
	if src, pos := interp.LoadExpr(nil); src != "" || pos.Line != 0 {
		t.Fatal("TestNodeInterp interp.LoadExpr failed:", src, pos)
	}
	var cb CodeBuilder
	if cb.getCaller(nil) != "" {
		t.Fatal("TestNodeInterp cb.getCaller failed")
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
	if WriteFile("/", nil, false) == nil {
		t.Fatal("WriteFile: no error?")
	}
}

func TestScopeHasName(t *testing.T) {
	scope := types.NewScope(types.Universe, 0, 0, "")
	child := types.NewScope(scope, 0, 0, "")
	child.Insert(types.NewVar(0, nil, "foo", types.Typ[types.Int]))
	has := scopeHasName(scope, "foo")
	if !has {
		t.Fatal("scopeHasName failed: foo not found?")
	}
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
	getPos([]token.Pos{1})
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestToVariadic: no error?")
		}
	}()
	toVariadic(&ast.Field{Type: &ast.Ident{Name: "int"}})
}

func TestUnderlying(t *testing.T) {
	typs := []types.Type{
		&refType{},
		&unboundType{},
		&unboundMapElemType{},
		&overloadFuncType{},
		&templateRecvMethodType{},
		&instructionType{},
		&TypeType{},
		&unboundFuncParam{},
		&unboundProxyParam{},
		&TemplateParamType{},
		&TemplateSignature{},
	}
	for _, typ := range typs {
		func() {
			defer func() {
				if e := recover(); e == nil {
					t.Fatal("TestUnderlying failed: no error?")
				}
			}()
			typ.Underlying()
		}()
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
	if typ := cb.structFieldType(struc, "val"); typ != types.Typ[types.Int] {
		t.Fatal("structFieldType failed:", typ)
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
	if typ := cb.structFieldType(struc, "val"); typ != types.Typ[types.Int] {
		t.Fatal("structFieldType failed:", typ)
	}
}

func TestVarDeclEnd(t *testing.T) {
	var decl VarDecl
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestVarDeclEnd failed: no error?")
		}
	}()
	decl.End(nil)
}

func TestCheckParenExpr(t *testing.T) {
	x := checkParenExpr(&ast.CompositeLit{})
	if _, ok := x.(*ast.ParenExpr); !ok {
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
	a := constant.MakeFromLiteral("1e1", token.FLOAT, 0)
	args := []*internal.Elem{
		{CVal: a},
	}
	nega := unaryOp(token.SUB, args)
	ret := doBinaryOp(nega, token.NEQ, constant.MakeInt64(-10))
	if constant.BoolVal(ret) {
		t.Fatal("TestUnaryOp failed:", nega)
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
	sizeof := pkg.unsafe().Ref("Sizeof")
	expr := toObjectExpr(pkg, sizeof)
	if v, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := v.X.(*ast.Ident); !ok || id.Name != "unsafe" || v.Sel.Name != "Sizeof" {
			t.Fatal("toObjectExpr failed:", v.X)
		}
	} else {
		t.Fatal("TestUnsafe failed:", expr)
	}
}

func TestUntypeBig(t *testing.T) {
	pkg := NewPackage("foo", "foo", gblConf)
	big := pkg.Import("github.com/goplus/gox/internal/builtin")
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

func TestCheckSignature(t *testing.T) {
	denoteRecv(&ast.SelectorExpr{Sel: ident("x")})
	if CheckSignature(nil, 0, 0) != nil {
		t.Fatal("TestCheckSignature failed: CheckSignature(nil) != nil")
	}
	sig := types.NewSignature(nil, nil, nil, false)
	if CheckSignature(sig, 0, 0) != sig {
		t.Fatal("TestCheckSignature failed: CheckSignature(sig) != sig")
	}
	pkg := types.NewPackage("", "foo")
	arg := types.NewParam(token.NoPos, pkg, "", sig)
	sig2 := types.NewSignature(nil, types.NewTuple(arg, arg), nil, false)
	o := types.NewFunc(token.NoPos, pkg, "bar", sig2)
	if CheckSignature(&templateRecvMethodType{fn: o}, 0, 0) == nil {
		t.Fatal("TestCheckSignature failed: CheckSignature == nil")
	}

	of := NewOverloadFunc(token.NoPos, pkg, "bar", o)
	if CheckSignature(of.Type(), 0, 0) == nil {
		t.Fatal("TestCheckSignature failed: OverloadFunc CheckSignature == nil")
	}
	if HasAutoProperty(of.Type()) {
		t.Fatal("func bar has autoprop?")
	}

	o2 := types.NewFunc(token.NoPos, pkg, "bar2", sig)
	of2 := NewOverloadFunc(token.NoPos, pkg, "bar3", o2)
	if !HasAutoProperty(of2.Type()) {
		t.Fatal("func bar3 has autoprop?")
	}

	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "t", nil), types.Typ[types.Int], nil)
	om := NewOverloadMethod(typ, token.NoPos, pkg, "bar", o)
	if CheckSignature(om.Type(), 0, 1) != nil {
		t.Fatal("TestCheckSignature failed: OverloadMethod CheckSignature != nil")
	}
}

func TestCheckSigParam(t *testing.T) {
	if checkSigParam(types.NewPointer(types.Typ[types.Int]), -1) {
		t.Fatal("TestCheckSigParam failed: checkSigParam *int should return false")
	}
	pkg := types.NewPackage("", "foo")
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "t", nil), types.Typ[types.Int], nil)
	if !checkSigParam(typ, -1) {
		t.Fatal("TestCheckSigParam failed: checkSigParam *t should return true")
	}
	typ2 := types.NewStruct(nil, nil)
	if !checkSigParam(typ2, -1) {
		t.Fatal("TestCheckSigParam failed: checkSigParam *t should return true")
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
	WriteFile("_gop_autogen.go", pkg, false)
}

func TestLoadExpr(t *testing.T) {
	var cb CodeBuilder
	if src, pos := cb.loadExpr(nil); src != "" || pos.Filename != "" {
		t.Fatal("TestLoadExpr failed")
	}
}

func TestGetBuiltinTI(t *testing.T) {
	if getBuiltinTI(types.NewPointer(types.Typ[0])) != nil {
		t.Fatal("TestGetBuiltinTI failed")
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

func TestImportPkg(t *testing.T) {
	pkg := NewPackage("foo/bar", "bar", gblConf)
	f := &file{importPkgs: make(map[string]*PkgRef)}
	a := f.importPkg(pkg, "./internal/a")
	if f.importPkgs["foo/bar/internal/a"] != a {
		t.Fatal("TestImportPkg failed")
	}
}

// ----------------------------------------------------------------------------
