//go:build genjs

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
	"bytes"
	"go/token"
	"go/types"
	"log"
	"syscall"
	"testing"

	"github.com/goplus/gogen"
)

func domTestJS(t *testing.T, pkg *gogen.Package, expectedGo, expectedJs string) {
	doDomTest(t, pkg, expectedGo, "")
	var b bytes.Buffer
	t.Helper()
	err := pkg.WriteJSTo(&b, "")
	if err != nil {
		t.Fatal("pkg.WriteJSTo failed:", err)
	}
	result := b.String()
	if result != expectedJs {
		t.Fatalf("\nWriteJSTo Result:\n%s\nExpected:\n%s\n", result, expectedJs)
	}
	log.Printf("====================== %s End =========================\n", t.Name())
}

func TestBasic(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	if pkg.Ref("main") == nil {
		t.Fatal("main not found")
	}
	domTestJS(t, pkg, `package main

func main()
`, `function main() {
}
`)
}

func TestDeleteVarDecl(t *testing.T) {
	pkg := newMainPackage()
	pkg.SetRedeclarable(true)
	scope := pkg.CB().Scope()
	defs := pkg.NewVarDefs(scope).SetComments(nil)
	decl := defs.New(token.NoPos, types.Typ[types.String], "a", "b")
	defs.New(token.NoPos, types.Typ[types.String], "c")
	if decl.Inited() {
		t.Fatal("TestDeleteVarDecl: inited?")
	}
	defs.Delete("b")
	defs.Delete("c")
	defs.NewAndInit(func(cb *gogen.CodeBuilder) int {
		cb.Val("10")
		return 1
	}, token.NoPos, types.Typ[types.String], "b")
	if err := defs.Delete("b"); err != syscall.EACCES {
		t.Fatal("defs.Delete b failed:", err)
	}
	if err := defs.Delete("unknown"); err != syscall.ENOENT {
		t.Fatal("defs.Delete unknown failed:", err)
	}
	domTestJS(t, pkg, `package main

var (
	a string
	b string
)
`, `let a
let b = "10"
`)
}

/*
func TestVarDecl(t *testing.T) {
	pkg := newMainPackage()
	scope := pkg.CB().Scope()
	decl := pkg.NewVarDefs(scope)
	decl.NewAndInit(func(cb *gogen.CodeBuilder) int {
		cb.Val(1).Val(2).BinaryOp(token.ADD).
			Val("1").Val("2").BinaryOp(token.ADD)
		return 2
	}, token.NoPos, nil, "n", "s")
	pkg.CB().NewVarStart(types.Typ[types.String], "x").
		Val("Hello, ").Val("XGo").BinaryOp(token.ADD).
		EndInit(1)
	if decl.New(token.NoPos, types.Typ[types.String], "y").Ref("y") == nil {
		t.Fatal("TestVarDecl failed: var y not found")
	}
	pkg.NewVarDefs(scope) // no variables, ignore
	domTestJS(t, pkg, `package main

var (
	n, s = 1 + 2, "1" + "2"
	y    string
)
var x string = "Hello, " + "XGo"
`, ``)
}
*/

func TestZeroLitAlias(t *testing.T) {
	pkg := newPackage("main")
	bar := pkg.AliasType("bar", types.Typ[types.Float64])
	results := types.NewTuple(types.NewVar(token.NoPos, pkg.Types, "", bar))
	pkg.NewFunc(nil, "foo", nil, results, false).BodyStart(pkg).
		ZeroLit(bar).Return(1).End()
	domTestJS(t, pkg, `package main

type bar = float64

func foo() bar
`, `function foo() {
	return 0;
}
`)
}

func TestFuncCall(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1, false).EndStmt().
		End()
	domTestJS(t, pkg, `package main

func main()
`, `import { Println } from "fmt";

function main() {
	Println("Hello");
}
`)
}

func TestImport(t *testing.T) {
	pkg := newMainPackage()
	fmt := pkg.Import("fmt")

	v := pkg.NewParam(token.NoPos, "v", types.NewSlice(gogen.TyByte))
	pkg.NewFunc(nil, "Println", types.NewTuple(v), nil, false).BodyStart(pkg).End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).EndStmt().
		End()
	domTestJS(t, pkg, `package main

func Println(v []byte)
func main()
`, `import { Println as Println1 } from "fmt";

function Println(v) {
}
function main() {
	Println1("Hello");
}
`)
}

func TestLoopFor(t *testing.T) {
	pkg := newMainPackage()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		/**/ For().None().Then(). // for {
		/******/ Val(pkg.Import("fmt").Ref("Println")).Val("Hi").Call(1).EndStmt().
		/**/ End().
		End()
	domTestJS(t, pkg, `package main

func main()
`, `import { Println } from "fmt";

function main() {
	for (;;) {
		Println("Hi");
	}
}
`)
}
