package gox_test

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

// ----------------------------------------------------------------------------

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
	pkg.NewFunc(nil, "test", nil, nil, false).BodyStart(pkg).
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
}
`)
}

// ----------------------------------------------------------------------------
