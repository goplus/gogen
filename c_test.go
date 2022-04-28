package gox_test

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

// ----------------------------------------------------------------------------

func TestBitFields(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.Uint], false),
	}
	tyT := pkg.NewType("T").InitType(pkg, types.NewStruct(fields, nil))
	pkg.SetVFields(tyT, gox.NewBitFields([]*gox.BitField{
		{Name: "z1", FldName: "x", Off: 0, Bits: 1},
		{Name: "z2", FldName: "x", Off: 1, Bits: 3},
		{Name: "u1", FldName: "y", Off: 0, Bits: 1},
		{Name: "u2", FldName: "y", Off: 1, Bits: 3},
	}))
	cb := pkg.NewFunc(nil, "test", nil, nil, false).BodyStart(pkg).
		NewVar(tyT, "a").
		NewVarStart(types.Typ[types.Int], "z").
		Val(ctxRef(pkg, "a")).MemberVal("z1").UnaryOp(token.SUB).
		Val(ctxRef(pkg, "a")).MemberVal("z2").
		BinaryOp(token.MUL).EndInit(1).
		NewVarStart(types.Typ[types.Uint], "u").
		Val(ctxRef(pkg, "a")).MemberVal("u1").UnaryOp(token.XOR).
		Val(ctxRef(pkg, "a")).MemberVal("u2").
		BinaryOp(token.MUL).EndInit(1).
		Val(ctxRef(pkg, "a")).MemberRef("z1").Val(1).Assign(1).
		Val(ctxRef(pkg, "a")).MemberRef("z2").Val(1).Assign(1).
		End()
	domTest(t, pkg, `package main

type T struct {
	x int
	y uint
}

func test() {
	var a T
	var z int = -(a.x << 63 >> 63) * (a.x << 60 >> 61)
	var u uint = ^(a.y & 1) * (a.y >> 1 & 7)
	{
		_autoGo_1 := &a.x
		*_autoGo_1 = *_autoGo_1&^1 | 1&1
	}
	{
		_autoGo_2 := &a.x
		*_autoGo_2 = *_autoGo_2&^14 | 1&7<<1
	}
}
`)
	cb.NewVar(tyT, "a").Val(ctxRef(pkg, "a"))
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestBitFields: no error?")
			}
		}()
		cb.MemberVal("z3")
	}()
	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestBitFields: no error?")
			}
		}()
		cb.MemberRef("z3")
	}()
}

func TestUnionFields(t *testing.T) {
	pkg := newMainPackage()
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
	}
	tyT := pkg.NewType("T").InitType(pkg, types.NewStruct(fields, nil))
	tyFlt := types.Typ[types.Float32]
	pkg.SetVFields(tyT, gox.NewUnionFields([]*gox.UnionField{
		{Name: "z", Type: tyFlt},
		{Name: "val", Type: tyFlt, Off: 4},
	}))
	barFields := []*types.Var{
		types.NewField(token.NoPos, pkg.Types, "a", types.Typ[types.Int], false),
		types.NewField(token.NoPos, pkg.Types, "T", tyT, true),
	}
	tyBar := pkg.NewType("Bar").InitType(pkg, types.NewStruct(barFields, nil))
	cb := pkg.NewFunc(nil, "test", nil, nil, false).BodyStart(pkg).
		NewVar(tyT, "a").NewVar(types.NewPointer(tyT), "b").
		NewVar(tyBar, "bara").NewVar(types.NewPointer(tyBar), "barb").
		NewVarStart(tyFlt, "z").Val(ctxRef(pkg, "a")).MemberVal("z").EndInit(1).
		NewVarStart(tyFlt, "val").Val(ctxRef(pkg, "a")).MemberVal("val").EndInit(1).
		NewVarStart(tyFlt, "z2").Val(ctxRef(pkg, "b")).MemberVal("z").EndInit(1).
		NewVarStart(tyFlt, "val2").Val(ctxRef(pkg, "b")).MemberVal("val").EndInit(1).
		NewVarStart(tyFlt, "barz").Val(ctxRef(pkg, "bara")).MemberVal("z").EndInit(1).
		NewVarStart(tyFlt, "barval").Val(ctxRef(pkg, "bara")).MemberVal("val").EndInit(1).
		NewVarStart(tyFlt, "barz2").Val(ctxRef(pkg, "barb")).MemberVal("z").EndInit(1).
		NewVarStart(tyFlt, "barval2").Val(ctxRef(pkg, "barb")).MemberVal("val").EndInit(1).
		Val(ctxRef(pkg, "a")).MemberRef("z").Val(1).Assign(1).
		Val(ctxRef(pkg, "barb")).MemberRef("val").Val(1.2).Assign(1).
		End()
	domTest(t, pkg, `package main

import unsafe "unsafe"

type T struct {
	x int
	y string
}
type Bar struct {
	a int
	T
}

func test() {
	var a T
	var b *T
	var bara Bar
	var barb *Bar
	var z float32 = *(*float32)(unsafe.Pointer(&a))
	var val float32 = *(*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(&a)) + 4))
	var z2 float32 = *(*float32)(unsafe.Pointer(b))
	var val2 float32 = *(*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(b)) + 4))
	var barz float32 = *(*float32)(unsafe.Pointer(&bara.T))
	var barval float32 = *(*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(&bara.T)) + 4))
	var barz2 float32 = *(*float32)(unsafe.Pointer(&barb.T))
	var barval2 float32 = *(*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(&barb.T)) + 4))
	*(*float32)(unsafe.Pointer(&a)) = 1
	*(*float32)(unsafe.Pointer(uintptr(unsafe.Pointer(&barb.T)) + 4)) = 1.2
}
`)
	defer func() {
		if e := recover(); e == nil {
			t.Fatal("TestUnionFields: no error?")
		}
	}()
	cb.NewVar(tyT, "a").Val(ctxRef(pkg, "a")).MemberVal("unknown")
}

// ----------------------------------------------------------------------------

func TestCFunc(t *testing.T) {
	pkg := newMainPackage()
	cfn := gox.NewCSignature(nil, nil, false)
	pkg.NewFunc(nil, "test", nil, nil, false).BodyStart(pkg).
		NewVar(cfn, "f").
		Val(ctxRef(pkg, "f")).Call(0).EndStmt().
		End()
	domTest(t, pkg, `package main

func test() {
	var f func()
	f()
}
`)
}

// ----------------------------------------------------------------------------
