/*
 Copyright 2021 The XGo Authors (xgo.dev)
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
	kind, _ = cb.Member("any", MemberFlagRef)
	if kind != MemberInvalid {
		t.Fatal("Member should return MemberInvalid for circular embedding")
	}
}

func TestFindMember(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()

	// create interface
	// type Impl interface {
	//     step__0()
	//     step__1()
	// }
	fn0 := types.NewFunc(token.NoPos, pkg.Types, "step__0", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	fn1 := types.NewFunc(token.NoPos, pkg.Types, "step__1", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	iface := types.NewInterfaceType([]*types.Func{fn0, fn1}, []types.Type{})
	tnamed := types.NewTypeName(token.NoPos, pkg.Types, "Impl", nil)
	named := types.NewNamed(tnamed, iface, nil)
	cb.Scope().Insert(named.Obj())

	// then call init to process overloads etc in the package
	InitThisGopPkg(pkg.Types)

	// create an interface that has method foo and embeds Impl
	// type Impl2 interface {
	//     foo()
	//     Impl
	// }
	var methods []*types.Func
	var embeddeds []types.Type
	mthd := types.NewFunc(token.NoPos, pkg.Types, "foo", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	methods = append(methods, mthd)
	embeddeds = append(embeddeds, named)
	iface2 := types.NewInterfaceType(methods, embeddeds)
	tnamed2 := types.NewTypeName(token.NoPos, pkg.Types, "Impl2", nil)
	named2 := types.NewNamed(tnamed2, iface2, nil)

	// push an element whose type is the interface
	cb.stk.Push(&Element{Type: named2})

	kind, _ := cb.Member("step", MemberFlagVal)
	if kind != MemberMethod {
		t.Fatalf("expected MemberMethod (1), got %v", kind)
	}
}
