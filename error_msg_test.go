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

package gox_test

import (
	"bytes"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

func sourceErrorTest(t *testing.T, msg string, source func(pkg *gox.Package)) {
	pkg := newMainPackage()
	defer func() {
		if e := recover(); e != nil {
			if err, ok := e.(*gox.SourceError); ok {
				pkg.CB().ResetStmt()
				if ret := err.Error(); ret != msg {
					t.Fatalf("\nError: \"%s\"\nExpected: \"%s\"\n", ret, msg)
				}
			} else {
				t.Fatal("Unexpected error:", e)
			}
		} else {
			t.Fatal("no error?")
		}
	}()
	source(pkg)
	var b bytes.Buffer
	gox.WriteTo(&b, pkg)
}

func TestFileLine(t *testing.T) {
	sourceErrorTest(t, "./foo.gop:1 func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam("v", gox.TyByte)
		pkg.CB().SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 1}, false)
		pkg.NewFunc(nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}

func TestErrInitFunc(t *testing.T) {
	sourceErrorTest(t, "func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam("v", gox.TyByte)
		pkg.NewFunc(nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}

func TestErrRecv(t *testing.T) {
	tySlice := types.NewSlice(gox.TyByte)
	sourceErrorTest(t, "invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", tySlice)
		pkg.NewFunc(recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	sourceErrorTest(t, "invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", types.NewPointer(tySlice))
		pkg.NewFunc(recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	sourceErrorTest(t, "invalid receiver type error (error is an interface type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", gox.TyError)
		pkg.NewFunc(recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
}

func TestErrLabel(t *testing.T) {
	sourceErrorTest(t, "./foo.gop:2 label foo already defined at ./foo.gop:1", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 1}, false).
			Label("foo").
			SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 2}, false).
			Label("foo").
			End()
	})
	sourceErrorTest(t, "./foo.gop:1 label foo is not defined", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 1}, false).
			Goto("foo").
			End()
	})
	sourceErrorTest(t, "./foo.gop:1 label foo defined and not used", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 1}, false).
			Label("foo").
			End()
	})
}

func TestErrNewVar(t *testing.T) {
	sourceErrorTest(t, "foo redeclared in this block\n\tprevious declaration at ./foo.gop:1",
		func(pkg *gox.Package) {
			var x *types.Var
			pkg.Fset.AddFile("./foo.gop", 1, 100).AddLine(10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewAutoVar(1, "foo", &x).
				NewAutoVar(11, "foo", &x).
				End()
		})
}

func TestErrMapLit(t *testing.T) {
	sourceErrorTest(t, "cannot use 1+2 (type untyped int) as type string in map key",
		func(pkg *gox.Package) {
			tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
			cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart("x")
			cb.ResetInit()
			cb.Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2")).
				Val(3).
				MapLit(tyMap, 2).
				End()
		})
	sourceErrorTest(t, `cannot use "Hi" + "!" (type untyped string) as type int in map value`,
		func(pkg *gox.Package) {
			tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("1").
				Val("Hi", source(`"Hi"`)).
				Val("!", source(`"!"`)).
				BinaryOp(token.ADD, source(`"Hi" + "!"`)).
				MapLit(tyMap, 2).
				EndStmt().
				End()
		})
}

func TestErrSliceLit(t *testing.T) {
	sourceErrorTest(t, "cannot use 1+2 (type untyped int) as type string in slice literal",
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2")).
				SliceLit(tySlice, 1).
				EndStmt().
				End()
		})
}
