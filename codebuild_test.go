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

package gogen

import (
	"go/token"
	"go/types"
	"testing"
)

func TestCircularEmbeddedFieldLookup(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()

	typeA := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "A", nil), nil, nil)
	typeB := types.NewNamed(types.NewTypeName(token.NoPos, pkg.Types, "B", nil), nil, nil)

	// Creates a circular embedding relationship between type A and B.
	typeA.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "", typeB, true), // Embed B.
	}, nil))
	typeB.SetUnderlying(types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, pkg.Types, "", typeA, true), // Embed A.
	}, nil))

	cb.stk.Push(&Element{Type: typeA})
	kind, _ := cb.Member("any", MemberFlagVal)
	if kind != MemberInvalid {
		t.Fatal("Member should return MemberInvalid for circular embedding")
	}
}
