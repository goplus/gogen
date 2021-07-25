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
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

type txtNode struct {
	Msg string
	pos *token.Position
}

func (p *txtNode) Pos() token.Pos {
	return 1
}

func (p *txtNode) End() token.Pos {
	return 2
}

var (
	pos2Positions = map[token.Pos]token.Position{}
)

// text, line, column
func source(text string, args ...interface{}) ast.Node {
	if len(args) < 2 {
		return &txtNode{Msg: text}
	}
	pos := &token.Position{Filename: "./foo.gop", Line: args[0].(int), Column: args[1].(int)}
	return &txtNode{Msg: text, pos: pos}
}

func position(line, column int) token.Pos {
	pos := token.Pos(len(pos2Positions) + 1)
	pos2Positions[pos] = token.Position{Filename: "./foo.gop", Line: line, Column: column}
	return pos
}

type nodeInterp struct{}

func (p nodeInterp) Position(pos token.Pos) (ret token.Position) {
	return pos2Positions[pos]
}

func (p nodeInterp) LoadExpr(node ast.Node) (src string, pos token.Position) {
	t := node.(*txtNode)
	if t.pos != nil {
		pos = *t.pos
	}
	src = t.Msg
	return
}

func codeErrorTest(t *testing.T, msg string, source func(pkg *gox.Package)) {
	pos2Positions = map[token.Pos]token.Position{}
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

func newFunc(
	pkg *gox.Package, line, column int,
	recv *gox.Param, name string, params, results *types.Tuple, variadic bool) *gox.Func {
	pos := position(line, column)
	fn, err := pkg.NewFuncWith(pos, name, types.NewSignature(recv, params, results, variadic))
	if err != nil {
		panic(err)
	}
	return fn
}

func TestErrInitFunc(t *testing.T) {
	codeErrorTest(t, "./foo.gop:1:5 func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam("v", gox.TyByte)
		newFunc(pkg, 1, 5, nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}

func TestErrRecv(t *testing.T) {
	tySlice := types.NewSlice(gox.TyByte)
	codeErrorTest(t, "./foo.gop:1:5 invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", tySlice)
		newFunc(pkg, 1, 5, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:2:6 invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", types.NewPointer(tySlice))
		newFunc(pkg, 2, 6, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:3:7 invalid receiver type error (error is an interface type)", func(pkg *gox.Package) {
		recv := pkg.NewParam("p", gox.TyError)
		newFunc(pkg, 3, 7, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:3:7 invalid receiver type recv (recv is a pointer type)", func(pkg *gox.Package) {
		t := pkg.NewType("recv").InitType(pkg, types.NewPointer(gox.TyByte))
		recv := pkg.NewParam("p", t)
		newFunc(pkg, 3, 7, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
}

func TestErrLabel(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:1 label foo already defined at ./foo.gop:1:1", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			Label("foo", source("foo:", 1, 1)).
			Label("foo", source("foo:", 2, 1)).
			End()
	})
	codeErrorTest(t, "./foo.gop:1:1 label foo is not defined", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			Goto("foo", source("goto foo", 1, 1)).
			End()
	})
	codeErrorTest(t, "./foo.gop:1:1 label foo defined and not used", func(pkg *gox.Package) {
		pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
			Label("foo", source("foo:", 1, 1)).
			End()
	})
}

func TestErrNewVar(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:6 foo redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			var x *types.Var
			pkg.Fset.AddFile("./foo.gop", 1, 100).AddLine(10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewAutoVar(position(1, 5), "foo", &x).
				NewAutoVar(position(2, 6), "foo", &x).
				End()
		})
}

func _TestErrDefineVar(t *testing.T) {
	codeErrorTest(t, "foo redeclared in this block\n\tprevious declaration at ./foo.gop:1",
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart("foo").Val(1).EndInit(1).
				DefineVarStart("foo").Val("Hi").EndInit(1).
				End()
		})
}

func TestErrStructLit(t *testing.T) {
	codeErrorTest(t, `./foo.gop:1:7 too many values in struct{x int; y string}{...}`,
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
	codeErrorTest(t, `./foo.gop:1:1 too few values in struct{x int; y string}{...}`,
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
	codeErrorTest(t, `./foo.gop:1:5 cannot use 1 (type untyped int) as type string in value of field y`,
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
	codeErrorTest(t, `./foo.gop:1:1 cannot use "1" (type untyped string) as type int in value of field x`,
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
	codeErrorTest(t, "./foo.gop:2:6 cannot use 1+2 (type untyped int) as type string in map key",
		func(pkg *gox.Package) {
			tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
			cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart("x")
			cb.ResetInit()
			cb.Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2", 2, 6)).
				Val(3).
				MapLit(tyMap, 2).
				End()
		})
	codeErrorTest(t, `./foo.gop:1:5 cannot use "Hi" + "!" (type untyped string) as type int in map value`,
		func(pkg *gox.Package) {
			tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("1").
				Val("Hi", source(`"Hi"`)).
				Val("!", source(`"!"`)).
				BinaryOp(token.ADD, source(`"Hi" + "!"`, 1, 5)).
				MapLit(tyMap, 2).
				EndStmt().
				End()
		})
}

func TestErrArrayLit(t *testing.T) {
	codeErrorTest(t, "./foo.gop:1:5 cannot use 32 (type untyped int) as type string in array literal",
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(32, source("32", 1, 5)).
				ArrayLit(tyArray, 2, true).
				EndStmt().
				End()
		})
	codeErrorTest(t, "./foo.gop:1:5 cannot use 1+2 (type untyped int) as type string in array literal",
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t, "./foo.gop:1:5 cannot use 32 (type untyped int) as type string in slice literal",
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(10, source("10")).
				Val(32, source("32", 1, 5)).
				SliceLit(tySlice, 2, true).
				EndStmt().
				End()
		})
	codeErrorTest(t, "./foo.gop:1:5 cannot use 1+2 (type untyped int) as type string in slice literal",
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t,
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
	codeErrorTest(t,
		`./foo.gop:1:5 invalid operation: true[1] (type untyped bool does not support indexing)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(types.Universe.Lookup("true"), source("true", 1, 5)).
				Val(1).
				Index(1, true, source("true[1]", 1, 5)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
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
	codeErrorTest(t,
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

func TestErrStar(t *testing.T) {
	codeErrorTest(t,
		`./foo.gop:1:5 invalid indirect of x (type string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				ElemRef(source("*x", 1, 4)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5 invalid indirect of x (type string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Elem(source("*x", 1, 4)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5 invalid indirect of x (type string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Star(source("*x", 1, 4)).
				EndStmt().
				End()
		})
}

func TestErrMember(t *testing.T) {
	codeErrorTest(t,
		`-  undefined (type string has no field or method y)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				MemberVal("y").
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5 x.y undefined (type string has no field or method y)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Debug(func(cb *gox.CodeBuilder) {
					_, err := cb.Member("y", source("x.y", 1, 5))
					if err != nil {
						panic(err)
					}
				}).
				EndStmt().
				End()
		})
}
