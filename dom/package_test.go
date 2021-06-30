package dom_test

import (
	"bytes"
	"testing"

	"github.com/goplus/gox"
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
	var a, b, c *gox.Var

	pkg := gox.NewPackage("main", nil)
	fmt := pkg.Import("fmt")

	pkg.NewFunc("main").BodyStart(pkg).
		NewVar("a", &a).NewVar("b", &b).NewVar("c", &c).                // type of variables will be auto detected
		VarRef(a).VarRef(b).Val("Hi").Val(3).Assign(2).EndStmt().       // a, b = "Hi", 3
		VarRef(c).Val(b).Assign(1).EndStmt().                           // c = b
		Val(fmt.Ref("Println")).Val(a).Val(b).Val(c).Call(3).EndStmt(). // fmt.Println(a, b, c)
		End()

	domTest(t, pkg, ``)
}
