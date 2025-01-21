//go:build go1.22
// +build go1.22

package typesutil

import "go/types"

type Alias = types.Alias

func NewAlias(obj *types.TypeName, rhs types.Type) *Alias {
	return types.NewAlias(obj, rhs)
}
