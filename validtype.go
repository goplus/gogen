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

package gogen

import (
	"go/types"
	_ "unsafe"
)

//go:linkname validType go/types.(*Checker).validType
func validType(check *types.Checker, typ *types.Named)

// ValidType verifies that the given type does not "expand" indefinitely
// producing a cycle in the type graph. Cycles are detected by marking
// defined types.
func (pkg *Package) ValidType(typ *types.Named) {
	conf := &types.Config{Error: pkg.conf.HandleErr}
	checker := types.NewChecker(conf, pkg.Fset, pkg.Types, nil)
	validType(checker, typ)
}
