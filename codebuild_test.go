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
