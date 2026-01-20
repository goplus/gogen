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
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gogen"
)

func TestTupleMember(t *testing.T) {
	pkg := newMainPackage()
	x := types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false)
	y := types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.Int], false)
	typ := pkg.NewTuple(true, x, y)
	a := types.NewParam(token.NoPos, pkg.Types, "a", typ)
	pkg.NewFunc(nil, "foo", types.NewTuple(a), nil, false).BodyStart(pkg).
		Val(ctxRef(pkg, "a")).
		MemberRef("x").
		Val(ctxRef(pkg, "a")).
		MemberVal("y").
		Assign(1).
		EndStmt().
		Debug(func(cb *gogen.CodeBuilder) {
			cb.Val(ctxRef(pkg, "a"))
			cb.Member("unknown", gogen.MemberFlagRef)
			cb.Member("unknown", gogen.MemberFlagVal)
			cb.ResetStmt()
		}).
		End()
	domTest(t, pkg, `package main

func foo(a struct {
	_0 int
	_1 int
}) {
	a._0 = a._1
}
`)
}
