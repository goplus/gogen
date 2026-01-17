//go:build go1.20
// +build go1.20

/*
 Copyright 2025 The XGo Authors (xgo.dev)
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
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gogen"
)

func TestUnsafeSlice(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(nil, "v").
		Val(1).Val(2).Val(3).ArrayLit(types.NewArray(types.Typ[types.Int], 3), 3).
		EndInit(1).
		NewVarStart(types.NewSlice(types.Typ[types.Int]), "s").
		Val(pkg.Unsafe().Ref("Slice")).VarVal("v").Val(0).Index(1, false).UnaryOp(token.AND).Val(3).CallWith(2, 0).
		EndInit(1).
		NewVarStart(types.NewPointer(types.Typ[types.Int]), "p").
		Val(pkg.Unsafe().Ref("SliceData")).VarVal("s").CallWith(1, 0).
		EndInit(1).
		End()

	domTest(t, pkg, `package main

import "unsafe"

func main() {
	var v = [3]int{1, 2, 3}
	var s []int = unsafe.Slice(&v[0], 3)
	var p *int = unsafe.SliceData(s)
}
`)
}

func TestUnsafeString(t *testing.T) {
	pkg := newXGoMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVarStart(nil, "v").
		Val('a').Val('b').Val('c').ArrayLit(types.NewArray(types.Typ[types.Byte], 3), 3).
		EndInit(1).
		NewVarStart(types.Typ[types.String], "s").
		Val(pkg.Unsafe().Ref("String")).VarVal("v").Val(0).Index(1, false).UnaryOp(token.AND).Val(3).CallWith(2, 0).
		EndInit(1).
		NewVarStart(types.NewPointer(types.Typ[types.Byte]), "p").
		Val(pkg.Unsafe().Ref("StringData")).VarVal("s").CallWith(1, 0).
		EndInit(1).
		End()

	domTest(t, pkg, `package main

import "unsafe"

func main() {
	var v = [3]uint8{'a', 'b', 'c'}
	var s string = unsafe.String(&v[0], 3)
	var p *uint8 = unsafe.StringData(s)
}
`)
}

func TestErrUnsafeData(t *testing.T) {
	codeErrorTest(t,
		`./foo.gop:7:18: first argument to unsafe.SliceData must be slice; have int`,
		func(pkg *gogen.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Unsafe()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyInt, "a").
				Val(builtin.Ref("SliceData")).VarVal("a").CallWith(1, 0, source(`unsafe.SliceData(a)`, 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:19: first argument to unsafe.StringData must be string; have int`,
		func(pkg *gogen.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Unsafe()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyInt, "a").
				Val(builtin.Ref("StringData")).VarVal("a").CallWith(1, 0, source(`unsafe.StringData(a)`, 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:15: first argument to unsafe.String must be *byte; have int`,
		func(pkg *gogen.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Unsafe()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyInt, "a").
				Val(builtin.Ref("String")).VarVal("a").Val(10).CallWith(2, 0, source(`unsafe.String(a, 10)`, 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:15: non-integer len argument in unsafe.String - untyped string`,
		func(pkg *gogen.Package) {
			tyByte := types.Typ[types.Byte]
			builtin := pkg.Unsafe()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVarStart(nil, "ar").
				Val('a').Val('b').Val('c').ArrayLit(types.NewArray(tyByte, 3), 3).EndInit(1).
				Val(builtin.Ref("String")).Val(ctxRef(pkg, "ar")).Val(0).Index(1, false).UnaryOp(token.AND).Val("hello").CallWith(2, 0, source(`unsafe.String(&ar[0],"hello")`, 7, 2)).EndStmt().
				End()
		})
}
