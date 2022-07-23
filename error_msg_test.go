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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

func (p nodeInterp) Caller(node ast.Node) string {
	t := node.(*txtNode)
	idx := strings.Index(t.Msg, "(")
	if idx > 0 {
		return t.Msg[:idx]
	}
	return t.Msg
}

func (p nodeInterp) LoadExpr(node ast.Node) (src string, pos token.Position) {
	t := node.(*txtNode)
	if t.pos != nil {
		pos = *t.pos
	}
	src = t.Msg
	return
}

func codeErrorTest(t *testing.T, msg string, source func(pkg *gox.Package), disableRecover ...bool) {
	t.Run(msg, func(t *testing.T) {
		codeErrorTestDo(t, msg, source, disableRecover...)
	})
}

func codeErrorTestDo(t *testing.T, msg string, source func(pkg *gox.Package), disableRecover ...bool) {
	pos2Positions = map[token.Pos]token.Position{}
	pkg := newMainPackage()
	if !(disableRecover != nil && disableRecover[0]) {
		defer func() {
			if e := recover(); e != nil {
				switch err := e.(type) {
				case *gox.CodeError, *gox.MatchError:
					defer recover()
					pkg.CB().ResetStmt()
					if ret := err.(error).Error(); ret != msg {
						t.Fatalf("\nError: \"%s\"\nExpected: \"%s\"\n", ret, msg)
					}
				case *gox.ImportError:
					if ret := err.Error(); ret != msg {
						t.Fatalf("\nError: \"%s\"\nExpected: \"%s\"\n", ret, msg)
					}
				default:
					t.Fatal("Unexpected error:", e)
				}
			} else {
				t.Fatal("no error?")
			}
		}()
	}
	source(pkg)
	var b bytes.Buffer
	gox.WriteTo(&b, pkg, "")
}

func newFunc(
	pkg *gox.Package, line, column int, rline, rcolumn int,
	recv *gox.Param, name string, params, results *types.Tuple, variadic bool) *gox.Func {
	pos := position(line, column)
	fn, err := pkg.NewFuncWith(
		pos, name, types.NewSignature(recv, params, results, variadic), func() token.Pos {
			return position(rline, rcolumn)
		})
	if err != nil {
		panic(err)
	}
	return fn
}

func TestErrSwitch(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:5: cannot use 1 (type untyped int) as type string",
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Switch().Val(`"x"`, source("x", 1, 3)).Then(). // switch "x"
				Val(1, source("1", 2, 5)).Val(2).Case(2).      // case 1, 2:
				/**/ End().
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: cannot use 1 (type untyped int) as type bool",
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Switch().None().Then().                   // switch
				Val(1, source("1", 2, 5)).Val(2).Case(2). // case 1, 2:
				/**/ End().
				End()
		})
}

func TestErrTypeRedefined(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:5: foo redeclared in this block\n\tprevious declaration at ./foo.gop:1:5", func(pkg *gox.Package) {
		typ := pkg.NewType("foo", position(1, 5))
		if typ.Inited() {
			t.Fatal("NewType failed: inited?")
		}
		pkg.NewType("foo", position(2, 5))
	})
}

func TestErrTypeSwitch(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:9: impossible type switch case: v (type interface{Bar()}) cannot have dynamic type int (missing Bar method)",
		func(pkg *gox.Package) {
			methods := []*types.Func{
				types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
			}
			tyInterf := types.NewInterfaceType(methods, nil).Complete()
			v := pkg.NewParam(token.NoPos, "v", tyInterf)
			pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
				/**/ TypeSwitch("t").Val(v, source("v", 1, 5)).TypeAssertThen().
				/**/ Typ(types.Typ[types.Int], source("int", 2, 9)).TypeCase(1).
				/**/ End().
				End()
		})
	codeErrorTest(t, "./foo.gop:2:9: 1 (type untyped int) is not a type",
		func(pkg *gox.Package) {
			v := pkg.NewParam(token.NoPos, "v", gox.TyEmptyInterface)
			pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
				/**/ TypeSwitch("t").Val(v).TypeAssertThen().
				/**/ Val(1, source("1", 2, 9)).TypeCase(1).
				/**/ End().
				End()
		})
}

func TestErrBinaryOp(t *testing.T) {
	codeErrorTest(t, `-: invalid operation: * (mismatched types int and float64)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a").
				NewVar(types.Typ[types.Float64], "b").
				Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.MUL).EndStmt().
				End()
		})
	codeErrorTest(t, `./foo.gop:2:9: invalid operation: a * b (mismatched types int and float64)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a").
				NewVar(types.Typ[types.Float64], "b").
				Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).BinaryOp(token.MUL, source(`a * b`, 2, 9)).EndStmt().
				End()
		})
	codeErrorTest(t, `./foo.gop:2:9: invalid operation: v != 3 (mismatched types interface{Bar()} and untyped int)`,
		func(pkg *gox.Package) {
			methods := []*types.Func{
				types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
			}
			tyInterf := types.NewInterfaceType(methods, nil).Complete()
			params := types.NewTuple(pkg.NewParam(token.NoPos, "v", tyInterf))
			pkg.NewFunc(nil, "foo", params, nil, false).BodyStart(pkg).
				/**/ If().Val(ctxRef(pkg, "v")).Val(3).BinaryOp(token.NEQ, source(`v != 3`, 2, 9)).Then().
				/**/ End().
				End()
		})
	codeErrorTest(t, `./foo.gop:2:9: invalid operation: sl == v (mismatched types []int and interface{Bar()})`,
		func(pkg *gox.Package) {
			methods := []*types.Func{
				types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
			}
			tyInterf := types.NewInterfaceType(methods, nil).Complete()
			params := types.NewTuple(pkg.NewParam(token.NoPos, "v", tyInterf))
			pkg.NewFunc(nil, "foo", params, nil, false).BodyStart(pkg).
				NewVar(types.NewSlice(types.Typ[types.Int]), "sl").
				/**/ If().Val(ctxRef(pkg, "sl")).Val(ctxRef(pkg, "v")).BinaryOp(token.EQL, source(`sl == v`, 2, 9)).Then().
				/**/ End().
				End()
		})
	codeErrorTest(t, `./foo.gop:2:9: invalid operation: 3 == "Hi" (mismatched types untyped int and untyped string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				/**/ If().Val(3).Val("Hi").BinaryOp(token.EQL, source(`3 == "Hi"`, 2, 9)).Then().
				/**/ End().
				End()
		})
}

func TestErrTypeAssert(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:9: impossible type assertion:\n\tstring does not implement bar (missing Bar method)",
		func(pkg *gox.Package) {
			methods := []*types.Func{
				types.NewFunc(token.NoPos, pkg.Types, "Bar", types.NewSignature(nil, nil, nil, false)),
			}
			tyInterf := types.NewInterfaceType(methods, nil).Complete()
			bar := pkg.NewType("bar").InitType(pkg, tyInterf)
			params := types.NewTuple(pkg.NewParam(token.NoPos, "v", bar))
			pkg.NewFunc(nil, "foo", params, nil, false).BodyStart(pkg).
				DefineVarStart(0, "x").Val(ctxRef(pkg, "v")).
				TypeAssert(types.Typ[types.String], false, source("v.(string)", 2, 9)).EndInit(1).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:9: invalid type assertion: v.(string) (non-interface type int on left)",
		func(pkg *gox.Package) {
			params := types.NewTuple(pkg.NewParam(token.NoPos, "v", types.Typ[types.Int]))
			pkg.NewFunc(nil, "foo", params, nil, false).BodyStart(pkg).
				DefineVarStart(0, "x").Val(ctxRef(pkg, "v")).
				TypeAssert(types.Typ[types.String], false, source("v.(string)", 2, 9)).EndInit(1).
				End()
		})
}

func TestErrConst(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:9: cannot use 1 (type untyped int) as type string in assignment",
		func(pkg *gox.Package) {
			pkg.NewConstStart(pkg.Types.Scope(), position(2, 7), types.Typ[types.String], "a").Val(1, source("1", 2, 9)).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: missing value in const declaration",
		func(pkg *gox.Package) {
			pkg.NewConstStart(pkg.Types.Scope(), position(2, 7), nil, "a", "b").Val(1).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: extra expression in const declaration",
		func(pkg *gox.Package) {
			pkg.NewConstStart(pkg.Types.Scope(), position(2, 7), nil, "a").Val(1).Val(2).EndInit(2)
		})
	codeErrorTest(t, "./foo.gop:2:7: a redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(1, 5), nil, "a").Val(1).EndInit(1)
			pkg.NewConstStart(pkg.Types.Scope(), position(2, 7), nil, "a").Val(2).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: a redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			scope := pkg.Types.Scope()
			pkg.NewConstStart(scope, position(1, 5), nil, "a").Val(2).EndInit(1)
			pkg.NewVarDefs(scope).New(position(2, 7), types.Typ[types.Int], "a").InitStart(pkg).Val(1).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: const initializer len(a) is not a constant",
		func(pkg *gox.Package) {
			pkg.NewVar(position(1, 5), types.NewSlice(types.Typ[types.Int]), "a")
			pkg.NewConstStart(pkg.Types.Scope(), position(2, 7), nil, "b").
				Val(ctxRef(pkg, "len")).Val(ctxRef(pkg, "a")).CallWith(1, 0, source("len(a)", 2, 10)).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:9: a redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(1, 5), nil, "a").Val(1).EndInit(1)
			pkg.NewConstDecl(pkg.Types.Scope()).
				New(func(cb *gox.CodeBuilder) int {
					cb.Val(2)
					return 1
				}, 0, position(2, 7), nil, "_").
				Next(1, position(2, 9), "a")
		})
	codeErrorTest(t, "./foo.gop:2:9: extra expression in const declaration",
		func(pkg *gox.Package) {
			pkg.NewConstDecl(pkg.Types.Scope()).
				New(func(cb *gox.CodeBuilder) int {
					cb.Val(2)
					cb.Val(ctxRef(pkg, "iota"))
					return 2
				}, 0, position(2, 7), nil, "a", "b").
				Next(1, position(2, 9), "c")
		})
	codeErrorTest(t, "./foo.gop:2:9: missing value in const declaration",
		func(pkg *gox.Package) {
			pkg.NewConstDecl(pkg.Types.Scope()).
				New(func(cb *gox.CodeBuilder) int {
					cb.Val(2)
					cb.Val(ctxRef(pkg, "iota"))
					return 2
				}, 0, position(2, 7), nil, "a", "b").
				Next(1, position(2, 9), "c", "d", "e")
		})
}

func TestErrNewVar(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:6: foo redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			var x *types.Var
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewAutoVar(position(1, 5), "foo", &x).
				NewAutoVar(position(2, 6), "foo", &x).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:9: cannot use 1 (type untyped int) as type string in assignment",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(2, 7), types.Typ[types.String], "a").Val(1, source("1", 2, 9)).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: assignment mismatch: 1 variables but fmt.Println returns 2 values",
		func(pkg *gox.Package) {
			fmt := pkg.Import("fmt")
			pkg.NewVarStart(position(2, 7), nil, "a").
				Val(fmt.Ref("Println")).Val(2).CallWith(1, 0, source("fmt.Println(2)", 2, 11)).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: assignment mismatch: 1 variables but 2 values",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(2, 7), nil, "a").Val(1).Val(2).EndInit(2)
		})
	codeErrorTest(t, "./foo.gop:2:7: assignment mismatch: 2 variables but 1 values",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(2, 7), nil, "a", "b").Val(2).EndInit(1)
		})
	codeErrorTest(t, "./foo.gop:2:7: a redeclared in this block\n\tprevious declaration at ./foo.gop:1:5",
		func(pkg *gox.Package) {
			pkg.NewVarStart(position(1, 5), nil, "a").Val(1).EndInit(1)
			pkg.NewVarStart(position(2, 7), nil, "a").Val(2).EndInit(1)
		})
}

func TestErrDefineVar(t *testing.T) {
	handleErr = func(err error) {
		if err.Error() != "./foo.gop:2:1: no new variables on left side of :=" {
			t.Fatal("TestErrDefineVar:", err)
		}
	}
	codeErrorTest(t, `./foo.gop:2:6: cannot use "Hi" (type untyped string) as type int in assignment`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "foo").Val(1).EndInit(1).
				DefineVarStart(position(2, 1), "foo").Val("Hi", source(`"Hi"`, 2, 6)).EndInit(1).
				End()
		})
}

func TestErrForRange(t *testing.T) {
	codeErrorTest(t, `./foo.gop:1:17: can't use return/continue/break/goto in for range of udt.Gop_Enum(callback)`,
		func(pkg *gox.Package) {
			foo := pkg.Import("github.com/goplus/gox/internal/foo")
			bar := foo.Ref("Foo2").Type()
			v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
			pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
				ForRange("a", "b").
				Val(v, source("v", 1, 9)).
				RangeAssignThen(position(1, 17)).
				Return(0).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: cannot range over v (type *github.com/goplus/gox/internal/foo.Foo4)`,
		func(pkg *gox.Package) {
			foo := pkg.Import("github.com/goplus/gox/internal/foo")
			bar := foo.Ref("Foo4").Type()
			v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
			pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
				ForRange().
				Val(v, source("v", 1, 9)).
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: cannot range over v (type *github.com/goplus/gox/internal/foo.Foo3)`,
		func(pkg *gox.Package) {
			foo := pkg.Import("github.com/goplus/gox/internal/foo")
			bar := foo.Ref("Foo3").Type()
			v := pkg.NewParam(token.NoPos, "v", types.NewPointer(bar))
			pkg.NewFunc(nil, "foo", types.NewTuple(v), nil, false).BodyStart(pkg).
				ForRange("a", "b").
				Val(v, source("v", 1, 9)).
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: cannot range over 13 (type untyped int)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				ForRange("a", "b").
				Val(13, source("13", 1, 9)).
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: cannot range over 13 (type untyped int)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a").
				ForRange().
				VarRef(ctxRef(pkg, "a")).
				Val(13, source("13", 1, 9)).
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: too many variables in range`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a", "b", "c").
				ForRange().
				VarRef(ctxRef(pkg, "a")).
				VarRef(ctxRef(pkg, "b")).
				VarRef(ctxRef(pkg, "c")).
				Val("Hello").
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:17: too many variables in range`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				ForRange("a", "b", "c").
				Val("Hello").
				RangeAssignThen(position(1, 17)).
				End().
				End()
		})
	codeErrorTest(t, `./foo.gop:1:5: cannot assign type string to a (type int) in range`,
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "a").Val(1).EndInit(1).
				NewVar(tySlice, "b").
				ForRange().
				VarRef(nil).
				VarRef(ctxRef(pkg, "a"), source("a", 1, 5)).
				Val(ctxRef(pkg, "b"), source("b", 1, 9)).
				RangeAssignThen(token.NoPos).
				End().
				End()
		})
}

func TestErrAssign(t *testing.T) {
	codeErrorTest(t, "./foo.gop:1:3: assignment mismatch: 1 variables but bar returns 2 values",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			newFunc(pkg, 3, 5, 3, 7, nil, "bar", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).End()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "x").
				VarRef(ctxRef(pkg, "x")).
				Val(ctxRef(pkg, "bar")).
				CallWith(0, 0, source("bar()", 1, 5)).
				AssignWith(1, 1, source("x = bar()", 1, 3)).
				End()
		})
	codeErrorTest(t, "./foo.gop:1:3: assignment mismatch: 1 variables but 2 values",
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "x").
				VarRef(ctxRef(pkg, "x")).
				Val(1).Val(2).
				AssignWith(1, 2, source("x = 1, 2", 1, 3)).
				End()
		})
}

func TestErrFuncCall(t *testing.T) {
	codeErrorTest(t, `./foo.gop:2:10: cannot call non-function a() (type int)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a").
				Val(ctxRef(pkg, "a")).CallWith(0, 0, source("a()", 2, 10)).
				End()
		})
	codeErrorTest(t, `./foo.gop:2:10: invalid use of ... in call to foo(a...)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "foo", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.Int], "a").
				Val(ctxRef(pkg, "foo"), source("foo", 2, 2)).Val(ctxRef(pkg, "a")).CallWith(1, 1, source("foo(a...)", 2, 10)).
				End()
		})
	codeErrorTest(t, `./foo.gop:3:5: cannot use a (type bool) as type int in argument to foo(a)`,
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", types.NewTuple(retInt), nil, false).BodyStart(pkg).
				End()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Debug(func(cb *gox.CodeBuilder) {
					pkg.NewVar(position(2, 9), types.Typ[types.Bool], "a")
				}).
				Val(ctxRef(pkg, "foo")).Val(ctxRef(pkg, "a"), source("a", 3, 5)).
				CallWith(1, 0, source("foo(a)", 3, 10)).
				End()
		})
}

func TestErrReturn(t *testing.T) {
	codeErrorTest(t, `./foo.gop:2:9: cannot use "Hi" (type untyped string) as type error in return argument`,
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).
				Val(1, source("1", 2, 7)).
				Val("Hi", source(`"Hi"`, 2, 9)).
				Return(2, source(`return 1, "Hi"`, 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: cannot use byte value as type error in return argument",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			retInt2 := pkg.NewParam(position(3, 10), "", types.Typ[types.Int])
			retByte := pkg.NewParam(position(3, 15), "", gox.TyByte)
			newFunc(pkg, 3, 5, 3, 7, nil, "bar", nil, types.NewTuple(retInt2, retByte), false).BodyStart(pkg).End()
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).
				Val(ctxRef(pkg, "bar")).
				CallWith(0, 0, source("bar()", 2, 9)).
				Return(1, source("return bar()", 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: too many arguments to return\n\thave (untyped int, untyped int, untyped int)\n\twant (int, error)",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).
				Val(1, source("1", 2, 7)).
				Val(2, source("2", 2, 9)).
				Val(3, source("3", 2, 11)).
				Return(3, source("return 1, 2, 3", 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: too few arguments to return\n\thave (untyped int)\n\twant (int, error)",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).
				Val(1, source("1", 2, 7)).
				Return(1, source("return 1", 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: too few arguments to return\n\thave (byte)\n\twant (int, error)",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(1, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(1, 15), "", gox.TyError)
			ret := pkg.NewParam(position(3, 10), "", gox.TyByte)
			newFunc(pkg, 3, 5, 3, 7, nil, "bar", nil, types.NewTuple(ret), false).BodyStart(pkg).End()
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).
				Val(ctxRef(pkg, "bar")).
				CallWith(0, 0, source("bar()", 2, 9)).
				Return(1, source("return bar()", 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: too many arguments to return\n\thave (int, error)\n\twant (byte)",
		func(pkg *gox.Package) {
			retInt := pkg.NewParam(position(3, 10), "", types.Typ[types.Int])
			retErr := pkg.NewParam(position(3, 15), "", gox.TyError)
			newFunc(pkg, 3, 5, 3, 7, nil, "bar", nil, types.NewTuple(retInt, retErr), false).BodyStart(pkg).End()
			ret := pkg.NewParam(position(1, 10), "", gox.TyByte)
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(ret), false).BodyStart(pkg).
				Val(ctxRef(pkg, "bar")).
				CallWith(0, 0, source("bar()", 2, 9)).
				Return(1, source("return bar()", 2, 5)).
				End()
		})
	codeErrorTest(t, "./foo.gop:2:5: not enough arguments to return\n\thave ()\n\twant (byte)",
		func(pkg *gox.Package) {
			ret := pkg.NewParam(position(1, 10), "", gox.TyByte)
			newFunc(pkg, 1, 5, 1, 7, nil, "foo", nil, types.NewTuple(ret), false).BodyStart(pkg).
				Return(0, source("return", 2, 5)).
				End()
		})
}

func TestErrInitFunc(t *testing.T) {
	codeErrorTest(t, "./foo.gop:1:5: func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam(token.NoPos, "v", gox.TyByte)
		newFunc(pkg, 1, 5, 1, 7, nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}

func TestErrRecv(t *testing.T) {
	tySlice := types.NewSlice(gox.TyByte)
	codeErrorTest(t, "./foo.gop:1:9: invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam(position(1, 7), "p", tySlice)
		newFunc(pkg, 1, 5, 1, 9, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:2:9: invalid receiver type []byte ([]byte is not a defined type)", func(pkg *gox.Package) {
		recv := pkg.NewParam(position(2, 7), "p", types.NewPointer(tySlice))
		newFunc(pkg, 2, 6, 2, 9, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:3:10: invalid receiver type error (error is an interface type)", func(pkg *gox.Package) {
		recv := pkg.NewParam(position(3, 9), "p", gox.TyError)
		newFunc(pkg, 3, 7, 3, 10, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
	codeErrorTest(t, "./foo.gop:3:10: invalid receiver type recv (recv is a pointer type)", func(pkg *gox.Package) {
		t := pkg.NewType("recv").InitType(pkg, types.NewPointer(gox.TyByte))
		recv := pkg.NewParam(position(3, 9), "p", t)
		newFunc(pkg, 3, 7, 3, 10, recv, "foo", nil, nil, false).BodyStart(pkg).End()
	})
}

func TestErrLabel(t *testing.T) {
	codeErrorTest(t, "./foo.gop:2:1: label foo already defined at ./foo.gop:1:1", func(pkg *gox.Package) {
		cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg)
		l := cb.NewLabel(position(1, 1), "foo")
		cb.NewLabel(position(2, 1), "foo")
		cb.Goto(l)
		cb.End()
	})
	codeErrorTest(t, "./foo.gop:1:1: label foo defined and not used", func(pkg *gox.Package) {
		cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg)
		cb.NewLabel(position(1, 1), "foo")
		cb.End()
	})
	codeErrorTest(t, "./foo.gop:1:1: syntax error: non-declaration statement outside function body", func(pkg *gox.Package) {
		pkg.CB().NewLabel(position(1, 1), "foo")
	})
	/*	codeErrorTest(t, "./foo.gop:1:1: label foo is not defined", func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Goto("foo", source("goto foo", 1, 1)).
				End()
		})
	*/
}

func TestErrStructLit(t *testing.T) {
	codeErrorTest(t, `./foo.gop:1:7: too many values in struct{x int; y string}{...}`,
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
	codeErrorTest(t, `./foo.gop:1:1: too few values in struct{x int; y string}{...}`,
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
	codeErrorTest(t, `./foo.gop:1:5: cannot use 1 (type untyped int) as type string in value of field y`,
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
	codeErrorTest(t, `./foo.gop:1:1: cannot use "1" (type untyped string) as type int in value of field x`,
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
	codeErrorTest(t, "./foo.gop:2:6: cannot use 1+2 (type untyped int) as type string in map key",
		func(pkg *gox.Package) {
			tyMap := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
			cb := pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				DefineVarStart(0, "x")
			cb.ResetInit()
			cb.Val(1, source("1")).
				Val(2, source("2")).
				BinaryOp(token.ADD, source("1+2", 2, 6)).
				Val(3).
				MapLit(tyMap, 2).
				End()
		})
	codeErrorTest(t, `./foo.gop:1:5: cannot use "Hi" + "!" (type untyped string) as type int in map value`,
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
	codeErrorTest(t, "./foo.gop:1:5: cannot use 32 (type untyped int) as type string in array literal",
		func(pkg *gox.Package) {
			tyArray := types.NewArray(types.Typ[types.String], 10)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1, source("1")).
				Val(32, source("32", 1, 5)).
				ArrayLit(tyArray, 2, true).
				EndStmt().
				End()
		})
	codeErrorTest(t, "./foo.gop:1:5: cannot use 1+2 (type untyped int) as type string in array literal",
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
		`./foo.gop:2:10: array index 1 out of bounds [0:1]`,
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
		`./foo.gop:1:5: array index 12 (value 12) out of bounds [0:10]`,
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
		`./foo.gop:2:10: array index 10 out of bounds [0:10]`,
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
		`./foo.gop:1:5: cannot use "Hi" + "!" as index which must be non-negative integer constant`,
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
		`./foo.gop:1:5: cannot use "10" as index which must be non-negative integer constant`,
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val("10", source(`"10"`, 1, 5)).
				Val("Hi", source(`"Hi"`)).
				SliceLit(tySlice, 2, true).
				EndStmt().
				End()
		})
	codeErrorTest(t, "./foo.gop:1:5: cannot use 32 (type untyped int) as type string in slice literal",
		func(pkg *gox.Package) {
			tySlice := types.NewSlice(types.Typ[types.String])
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(10, source("10")).
				Val(32, source("32", 1, 5)).
				SliceLit(tySlice, 2, true).
				EndStmt().
				End()
		})
	codeErrorTest(t, "./foo.gop:1:5: cannot use 1+2 (type untyped int) as type string in slice literal",
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
		`./foo.gop:1:5: cannot slice true (type untyped bool)`,
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
		`./foo.gop:1:1: cannot slice x (type *byte)`,
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
		`./foo.gop:1:5: invalid operation x[1:3:5] (3-index slice of string)`,
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
		`./foo.gop:1:5: invalid operation: true[1] (type untyped bool does not support indexing)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(types.Universe.Lookup("true"), source("true", 1, 5)).
				Val(1).
				Index(1, true, source("true[1]", 1, 5)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5: assignment mismatch: 2 variables but 1 values`,
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
		`./foo.gop:1:5: cannot assign to x[1] (strings are immutable)`,
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
		`./foo.gop:1:5: invalid indirect of x (type string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				ElemRef(source("*x", 1, 4)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5: invalid indirect of x (type string)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Elem(source("*x", 1, 4)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5: invalid indirect of x (type string)`,
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
		`./foo.gop:1:5: T.x undefined (type T has no method x)`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			pkg.NewType("T").InitType(pkg, types.NewStruct(fields, nil))
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(ctxRef(pkg, "T")).
				Debug(func(cb *gox.CodeBuilder) {
					_, err := cb.Member("x", gox.MemberFlagVal, source("T.x", 1, 5))
					if err != nil {
						panic(err)
					}
				}).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:7: x.y undefined (type string has no field or method y)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Debug(func(cb *gox.CodeBuilder) {
					_, err := cb.Member("y", gox.MemberFlagVal, source("x.y", 1, 7))
					if err != nil {
						panic(err)
					}
				}).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:5: x.y undefined (type string has no field or method y)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				Debug(func(cb *gox.CodeBuilder) {
					_, err := cb.Member("y", gox.MemberFlagVal, source("x.y", 1, 5))
					if err != nil {
						panic(err)
					}
				}).
				EndStmt().
				End()
		})
}

func TestErrMemberRef(t *testing.T) {
	codeErrorTest(t,
		`./foo.gop:1:7: x.y undefined (type string has no field or method y)`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(types.Typ[types.String], "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				MemberRef("y", source("x.y", 1, 7)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:7: x.y undefined (type aaa has no field or method y)`,
		func(pkg *gox.Package) {
			t := pkg.NewType("aaa").InitType(pkg, gox.TyByte)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(t, "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				MemberRef("y", source("x.y", 1, 7)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:7: x.z undefined (type struct{x int; y string} has no field or method z)`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			t := types.NewStruct(fields, nil)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(t, "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				MemberRef("z", source("x.z", 1, 7)).
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:7: x.z undefined (type aaa has no field or method z)`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			tyStruc := types.NewStruct(fields, nil)
			t := pkg.NewType("aaa").InitType(pkg, tyStruc)
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(t, "x").
				Val(ctxRef(pkg, "x"), source("x", 1, 5)).
				MemberRef("z", source("x.z", 1, 7)).
				EndStmt().
				End()
		})
}

func TestErrUnsafe(t *testing.T) {
	codeErrorTest(t,
		`./foo.gop:6:15: missing argument to function call: unsafe.Sizeof()`,
		func(pkg *gox.Package) {
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(builtin.Ref("Sizeof")).CallWith(0, 0, source("unsafe.Sizeof()", 6, 2)).EndStmt().
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:6:15: too many arguments to function call: unsafe.Sizeof(1, 2)`,
		func(pkg *gox.Package) {
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(builtin.Ref("Sizeof")).Val(1).Val(2).CallWith(2, 0, source("unsafe.Sizeof(1, 2)", 6, 2)).EndStmt().
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:6:17: invalid expression unsafe.Offsetof(1)`,
		func(pkg *gox.Package) {
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(builtin.Ref("Offsetof")).Val(1).CallWith(1, 0, source("unsafe.Offsetof(1)", 6, 2)).EndStmt().
				EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:14:17: invalid expression unsafe.Offsetof(a.Bar): argument is a method value`,
		func(pkg *gox.Package) {
			fields := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
			}
			typ := types.NewStruct(fields, nil)
			foo := pkg.NewType("foo").InitType(pkg, typ)
			recv := pkg.NewParam(token.NoPos, "a", foo)
			pkg.NewFunc(recv, "Bar", nil, nil, false).BodyStart(pkg).End()
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(foo, "a").
				Val(builtin.Ref("Offsetof")).Val(ctxRef(pkg, "a")).MemberVal("Bar").CallWith(1, 0, source("unsafe.Offsetof(a.Bar)", 14, 2)).EndStmt().
				EndStmt().
				End()
		})
	codeErrorTest(t, `./foo.gop:17:26: invalid expression unsafe.Offsetof(t.M.m): selector implies indirection of embedded t.M`,
		func(pkg *gox.Package) {
			builtin := pkg.Builtin()
			fieldsM := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "m", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "n", types.Typ[types.String], false),
			}
			typM := types.NewStruct(fieldsM, nil)
			tyM := pkg.NewType("M").InitType(pkg, typM)
			fieldsT := []*types.Var{
				types.NewField(token.NoPos, pkg.Types, "x", types.Typ[types.Int], false),
				types.NewField(token.NoPos, pkg.Types, "y", types.Typ[types.String], false),
				types.NewField(token.NoPos, pkg.Types, "", types.NewPointer(tyM), true),
			}
			typT := types.NewStruct(fieldsT, nil)
			tyT := pkg.NewType("T").InitType(pkg, typT)
			pkg.CB().NewVar(tyT, "t")
			pkg.CB().NewConstStart(nil, "c").
				Val(builtin.Ref("Offsetof")).Val(ctxRef(pkg, "t"), source("t", 17, 27)).MemberVal("m").CallWith(1, 0, source("unsafe.Offsetof(t.m)", 17, 11)).EndInit(1)
		})
	codeErrorTest(t,
		`./foo.gop:7:12: cannot use a (type int) as type unsafe.Pointer in argument to unsafe.Add`,
		func(pkg *gox.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyInt, "a").
				Val(builtin.Ref("Add")).Val(ctxRef(pkg, "a"), source("a", 7, 14)).Val(10).CallWith(2, 0, source("unsafe.Add(a, 10)", 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:12: cannot use "hello" (type untyped string) as type int`,
		func(pkg *gox.Package) {
			tyUP := types.Typ[types.UnsafePointer]
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyUP, "a").
				Val(builtin.Ref("Add")).Val(ctxRef(pkg, "a"), source("a", 7, 14)).Val("hello", source(`"hello"`, 7, 16)).CallWith(2, 0, source("unsafe.Add(a, 10)", 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:14: first argument to unsafe.Slice must be pointer; have int`,
		func(pkg *gox.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(tyInt, "a").
				Val(builtin.Ref("Slice")).Val(ctxRef(pkg, "a")).Val(10).CallWith(2, 0, source(`unsafe.Slice(a, 10)`, 7, 2)).EndStmt().
				End()
		})
	codeErrorTest(t,
		`./foo.gop:7:14: non-integer len argument in unsafe.Slice - untyped string`,
		func(pkg *gox.Package) {
			tyInt := types.Typ[types.Int]
			builtin := pkg.Builtin()
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVarStart(nil, "ar").
				Val(1).Val(2).Val(3).ArrayLit(types.NewArray(tyInt, 3), 3).EndInit(1).
				Val(builtin.Ref("Slice")).Val(ctxRef(pkg, "ar")).Val(0).Index(1, false).UnaryOp(token.AND).Val("hello").CallWith(2, 0, source(`unsafe.Slice(&a[0],"hello")`, 7, 2)).EndStmt().
				End()
		})
}

func TestImportPkgError(t *testing.T) {
	codeErrorTest(t,
		fmt.Sprintf(`./foo.gop:1:7: package bar2 is not in GOROOT (%v)
`, filepath.Join(runtime.GOROOT(), "src", "bar2")),
		func(pkg *gox.Package) {
			spec := &ast.ImportSpec{
				Path: &ast.BasicLit{ValuePos: position(1, 7), Kind: token.STRING, Value: strconv.Quote("bar")},
			}
			pkg.Import("bar2", spec)
		})
}

func TestDivisionByZero(t *testing.T) {
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1).Val(0, source("0", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(1.1).Val(0.0, source("0.0", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				Val(&ast.BasicLit{Kind: token.IMAG, Value: "1i"}).Val(&ast.BasicLit{Kind: token.IMAG, Value: "0i"}, source("0i", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				Val(ctxRef(pkg, "a")).Val(0, source("0", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				Val(ctxRef(pkg, "a")).Val(0.0, source("0.0", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				Val(ctxRef(pkg, "a")).Val(&ast.BasicLit{Kind: token.IMAG, Value: "0i"}, source("0i", 1, 3)).BinaryOp(token.QUO).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				VarRef(ctxRef(pkg, "a")).Val(0, source("0", 1, 3)).AssignOp(token.QUO_ASSIGN).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				VarRef(ctxRef(pkg, "a")).Val(0.0, source("0.0", 1, 3)).AssignOp(token.QUO_ASSIGN).
				End()
		})
	codeErrorTest(t,
		`./foo.gop:1:3: invalid operation: division by zero`,
		func(pkg *gox.Package) {
			typ := types.Typ[types.Int]
			pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
				NewVar(typ, "a").
				VarRef(ctxRef(pkg, "a")).Val(&ast.BasicLit{Kind: token.IMAG, Value: "0i"}, source("0i", 1, 3)).AssignOp(token.QUO_ASSIGN).
				End()
		})
}
