/*
 Copyright 2024 The GoPlus Authors (goplus.org)
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

package goxdbg

import (
	"bytes"
	"go/ast"
	"go/token"
	"log"

	"github.com/goplus/gogen/internal/go/format"
)

// Format formats node in canonical gofmt style.
func Format(fset *token.FileSet, v ast.Node) string {
	var b bytes.Buffer
	err := format.Node(&b, fset, v)
	if err != nil {
		log.Fatalln("goxdbg.Format failed:", err)
	}
	return b.String()
}
