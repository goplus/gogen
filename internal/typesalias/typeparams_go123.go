//go:build go1.23
// +build go1.23

package typesalias

import "go/types"

func TypeArgs(t *Alias) *types.TypeList {
	return t.TypeArgs()
}

func TypeParams(t *Alias) *types.TypeParamList {
	return t.TypeParams()
}
