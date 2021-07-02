package gox_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/goplus/gox"
	"golang.org/x/tools/go/gcexportdata"
)

func domTest(t *testing.T, pkg *gox.Package, expected string) {
	var b bytes.Buffer
	err := gox.WriteTo(&b, pkg)
	if err != nil {
		t.Fatal("conv.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\nResult:\n%s\nExpected:%s\n", result, expected)
	}
}

func TestBasic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func main() {
}
`)
}

func TestFuncBasic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", gox.TyByte)
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, false).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v byte) {
}
func main() {
}
`)
}

func TestFuncVariadic(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	v := pkg.NewParam("v", types.NewSlice(gox.TyByte))
	pkg.NewFunc(nil, "foo", gox.NewTuple(v), nil, true).BodyStart(pkg).End()
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).End()
	domTest(t, pkg, `package main

func foo(v ...byte) {
}
func main() {
}
`)
}

func TestGoTypesPkg(t *testing.T) {
	const src = `package foo

type mytype = byte

func bar(v mytype) rune {
	return 0
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "foo.go", src, 0)
	if err != nil {
		t.Fatal("parser.ParseFile:", err)
	}

	packages := make(map[string]*types.Package)
	imp := gcexportdata.NewImporter(fset, packages)
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check("foo", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal("conf.Check:", err)
	}
	bar := pkg.Scope().Lookup("bar")
	if bar.String() != "func foo.bar(v byte) rune" {
		t.Fatal("bar.Type:", bar)
	}
}

func TestFuncCall(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	fmt.Println("Hello")
}
`)
}

func TestImport(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")

	v := pkg.NewParam("v", types.NewSlice(gox.TyByte))
	pkg.NewFunc(nil, "fmt", gox.NewTuple(v), nil, false).BodyStart(pkg).End()

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		Val(fmt.Ref("Println")).Val("Hello").Call(1).EndStmt().
		End()
	domTest(t, pkg, `package main

import fmt1 "fmt"

func fmt(v []byte) {
}
func main() {
	fmt1.Println("Hello")
}
`)
}

func TestAssign(t *testing.T) {
	var a, b, c, d, e, f, g *gox.Var
	pkg := gox.NewPackage("", "main", nil)
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewVar("a", &a).NewVar("b", &b).NewVar("c", &c).NewVar("d", &d).
		NewVar("e", &e).NewVar("f", &f).NewVar("g", &g).
		VarRef(a).VarRef(b).VarRef(d).VarRef(e).VarRef(f).VarRef(g).
		Val("Hi").Val(3).Val(true).Val('!').Val(1.2).Val(&ast.BasicLit{Kind: token.FLOAT, Value: "12.3"}).
		Assign(6).EndStmt().
		VarRef(c).Val(b).Assign(1).EndStmt().
		End()
	domTest(t, pkg, `package main

func main() {
	var a string
	var b int
	var c int
	var d bool
	var e rune
	var f float64
	var g float64
	a, b, d, e, f, g = "Hi", 3, true, '!', 1.2, 12.3
	c = b
}
`)
}

func TestClosure(t *testing.T) {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	paramV := pkg.NewParam("v", types.Typ[types.String]) // v string
	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		NewClosure(gox.NewTuple(paramV), nil, false).BodyStart(pkg).
		/**/ Val(fmt.Ref("Println")).Val(paramV).Call(1).EndStmt().
		/**/ End().
		/**/ Val("Hello").Call(1).EndStmt(). // func(v string) { fmt.Println(v) } ("Hello")
		End()
	domTest(t, pkg, `package main

import fmt "fmt"

func main() {
	func(v string) {
		fmt.Println(v)
	}("Hello")
}
`)
}
