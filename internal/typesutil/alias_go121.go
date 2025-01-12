//go:build !go1.22
// +build !go1.22

package typesutil

import "go/types"

type Alias struct {
}

const unsupported = "typesAlias are unsupported at this go version"

func (t *Alias) Underlying() types.Type {
	panic(unsupported)
}

func (t *Alias) String() string {
	panic(unsupported)
}

func (t *Alias) Obj() *types.TypeName {
	panic(unsupported)
}

func NewAlias(obj *types.TypeName, rhs types.Type) *Alias {
	return &Alias{}
}
