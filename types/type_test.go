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

package types

import (
	"go/token"
	"go/types"
	"testing"
)

func TestSubstVar(t *testing.T) {
	pkg := types.NewPackage("", "foo")
	a := types.NewParam(0, pkg, "a", types.Typ[types.Int])
	scope := pkg.Scope()
	scope.Insert(NewSubst(token.NoPos, pkg, "bar", a))
	o := Lookup(scope, "bar")
	if o != a {
		t.Fatal("TestSubstVar:", o)
	}
	_, o = LookupParent(scope, "bar", token.NoPos)
	if o != a {
		t.Fatal("TestSubstVar:", o)
	}
	scope.Insert(a)
	_, o2 := LookupParent(scope, "a", token.NoPos)
	if o != o2 {
		t.Fatal("TestSubstVar:", o2)
	}
	o2 = Lookup(scope, "a")
	if o != o2 {
		t.Fatal("TestSubstVar:", o2)
	}
	LookupParent(scope, "b", token.NoPos)
	Lookup(scope, "b")
}
