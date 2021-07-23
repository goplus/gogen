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
			if err, ok := e.(*gox.CodeError); ok {
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

func _TestErrDefineVar(t *testing.T) {
	sourceErrorTest(t, "foo redeclared in this block\n\tprevious declaration at ./foo.gop:1",
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart("foo").Val(1).EndInit(1).
				DefineVarStart("foo").Val("Hi").EndInit(1).
				End()
		})
}

func TestErrStructLit(t *testing.T) {
	sourceErrorTest(t, `./foo.gop:1:7 too many values in struct{x int; y string}{...}`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			tyStruc := types.NewStruct(fields, nil)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source(`1`, 1, 1)).
				Val("1", source(`"1"`, 1, 5)).
				Val(1, source(`1`, 1, 7)).
				StructLit(tyStruc, 3, false).
				EndStmt().
				End()
		})
	sourceErrorTest(t, `./foo.gop:1:1 too few values in struct{x int; y string}{...}`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			tyStruc := types.NewStruct(fields, nil)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source(`1`, 1, 1)).
				StructLit(tyStruc, 1, false).
				EndStmt().
				End()
		})
	sourceErrorTest(t, `./foo.gop:1:5 cannot use 1 (type untyped int) as type string in value of field y`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			tyStruc := types.NewStruct(fields, nil)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1).
				Val(1, source(`1`, 1, 5)).
				StructLit(tyStruc, 2, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t, `./foo.gop:1:1 cannot use "1" (type untyped string) as type int in value of field x`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			tyStruc := types.NewStruct(fields, nil)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("1", source(`"1"`, 1, 1)).
				Val(1, source(`1`, 1, 5)).
				StructLit(tyStruc, 2, false).
				EndStmt().
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

func TestErrArrayLit(t *testing.T) {
	sourceErrorTest(t, "./foo.gop:1:5 cannot use 32 (type untyped int) as type string in array literal",
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(32, source("32", 1, 5)).
				ArrayLit(tyArray, 2, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t, "./foo.gop:1:5 cannot use 1+2 (type untyped int) as type string in array literal",
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2", 1, 5)).
				ArrayLit(tyArray, 1).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:2:10 array index 1 out of bounds [0:1]`,
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 1)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("Hi", source(`"Hi"`, 1, 5)).
				Val("!", source(`"!"`, 2, 10)).
				ArrayLit(tyArray, 2, false).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:1:5 array index 12 (value 12) out of bounds [0:10]`,
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(12, source(`12`, 1, 5)).
				Val("!", source(`"!"`)).
				ArrayLit(tyArray, 2, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:2:10 array index 10 out of bounds [0:10]`,
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(9, source(`9`, 1, 5)).
				Val("!", source(`"!"`)).
				None().
				Val("!!", source(`"!!"`, 2, 10)).
				ArrayLit(tyArray, 4, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:1:5 cannot use "Hi" + "!" as index which must be non-negative integer constant`,
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 100)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("Hi", source(`"Hi"`)).
				Val("!", source(`"!"`)).
				BinaryOp(token.ADD, source(`"Hi" + "!"`, 1, 5)).
				Val("Hi", source(`"Hi"`)).
				ArrayLit(tyArray, 2, true).
				EndStmt().
				End()
		})
}

func TestErrSliceLit(t *testing.T) {
	sourceErrorTest(t,
		`./foo.gop:1:5 cannot use "10" as index which must be non-negative integer constant`,
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("10", source(`"10"`, 1, 5)).
				Val("Hi", source(`"Hi"`)).
				SliceLit(tySlice, 2, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t, "./foo.gop:1:5 cannot use 32 (type untyped int) as type string in slice literal",
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(10, source("10")).
				Val(32, source("32", 1, 5)).
				SliceLit(tySlice, 2, true).
				EndStmt().
				End()
		})
	sourceErrorTest(t, "./foo.gop:1:5 cannot use 1+2 (type untyped int) as type string in slice literal",
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2", 1, 5)).
				SliceLit(tySlice, 1).
				EndStmt().
				End()
		})
}

func TestErrSlice(t *testing.T) {
	sourceErrorTest(t,
		`./foo.gop:1:5 cannot slice true (type untyped bool)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(types.Universe.Lookup("true"), source("true", 1, 5)).
				Val(1).
				Val(3).
				Val(5).
				Slice(true, source("true[1:3:5]", 1, 5)).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:1:1 cannot slice x (type *byte)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.NewPointer(gox.TyByte), "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 1)).
				Val(1).
				Val(3).
				Val(5).
				Slice(true, source("x[1:3:5]", 1, 5)).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:1:5 invalid operation x[1:3:5] (3-index slice of string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x")).
				Val(1).
				Val(3).
				Val(5).
				Slice(true, source("x[1:3:5]", 1, 5)).
				EndStmt().
				End()
		})
}

func TestErrIndex(t *testing.T) {
	sourceErrorTest(t,
		`./foo.gop:1:5 invalid operation: true[1] (type untyped bool does not support indexing)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(types.Universe.Lookup("true"), source("true", 1, 5)).
				Val(1).
				Index(1, true, source("true[1]", 1, 5)).
				EndStmt().
				End()
		})
	sourceErrorTest(t,
		`./foo.gop:1:5 assignment mismatch: 2 variables but 1 values`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x")).
				Val(1).
				Index(1, true, source("x[1]", 1, 5)).
				EndStmt().
				End()
		})
}

func TestErrIndexRef(t *testing.T) {
	sourceErrorTest(t,
		`./foo.gop:1:5 cannot assign to x[1] (strings are immutable)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x")).
				Val(1).
				IndexRef(1, source("x[1]", 1, 5)).
				EndStmt().
				End()
		})
}
