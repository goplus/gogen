//go:build !go1.22
// +build !go1.22

package typesalias

import "go/types"

const Support = false

type Alias struct {
	obj *types.TypeName
}

const unsupported = "typesAlias are unsupported at this go version"

func (t *Alias) Underlying() types.Type {
	panic(unsupported)
}

func (t *Alias) String() string {
	panic(unsupported)
}

func (t *Alias) Obj() *types.TypeName {
	return t.obj
}

func NewAlias(obj *types.TypeName, rhs types.Type) *Alias {
	return &Alias{obj}
}

func Unalias(t types.Type) types.Type {
	return t
}
