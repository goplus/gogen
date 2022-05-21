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
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/tools/go/gcexportdata"
)

func main() {
	val := filepath.Join(runtime.GOROOT(), "pkg/mod")
	os.Setenv("GOMODCACHE", val)
	fmt.Println("GOMODCACHE:", val)

	var fset = token.NewFileSet()
	var imp types.Importer
	if true {
		imp = importer.ForCompiler(fset, "source", nil)
	} else {
		packages := make(map[string]*types.Package)
		imp = gcexportdata.NewImporter(fset, packages)
	}

	_, err := imp.Import("go/types")
	fmt.Println("Import result:", err)

	_, err = imp.Import("golang.org/x/tools/go/gcexportdata")
	fmt.Println("Import result:", err)
}
