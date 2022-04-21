package main

import (
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"

	"github.com/goplus/gox/packages"
)

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
	checkSideEffect(imp, pkg)
}

func checkSideEffect(imp types.Importer, pkg *types.Package) {
	fmt.Println("==> Checking", pkg.Path())
	if !pkg.Complete() {
		pkg, _ = imp.Import(pkg.Path())
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
		checkSideEffect(imp, ref)
	}
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
