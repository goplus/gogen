/*
 Copyright 2022 The GoPlus Authors (goplus.org)
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

package main

import (
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"

	"github.com/goplus/gogen/packages"
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
