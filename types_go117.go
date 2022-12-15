//go:build !go1.18
// +build !go1.18

package gox

import (
	"go/ast"

	"github.com/goplus/gox/internal"
)

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	panic("type parameters are unsupported at this go version")
}
