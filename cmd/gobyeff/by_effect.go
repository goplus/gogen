package main

import (
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"

	"github.com/goplus/gox/packages"
)

type none = struct{}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: gobyeff pkgPath\n")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	fset := token.NewFileSet()
	imp := packages.NewImporter(fset)
	pkg, err := imp.Import(os.Args[1])
	check(err)
	checkSideEffect(imp, pkg, make(map[string]none))
}

func checkSideEffect(imp types.Importer, pkg *types.Package, checked map[string]none) {
	pkgPath := pkg.Path()
	if _, ok := checked[pkgPath]; ok {
		return
	}
	checked[pkgPath] = none{}
	fmt.Println("==> Checking", pkgPath)
	if !pkg.Complete() {
		pkg, _ = imp.Import(pkgPath)
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		switch o := scope.Lookup(name).(type) {
		case *types.Var:
			fmt.Println("var", o.Name(), o.Type())
		}
	}
	fmt.Println()
	for _, ref := range pkg.Imports() {
		checkSideEffect(imp, ref, checked)
	}
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
