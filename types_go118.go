//go:build go1.18
// +build go1.18

package gox

import (
	"go/ast"
	"go/types"
	"log"

	"github.com/goplus/gox/internal"
)

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		log.Println("Instantiate", nidx)
	}
	sig := args[0].Type.(*types.Signature)
	targs := make([]types.Type, nidx)
	for i := 0; i < nidx; i++ {
		targs[i] = args[i+1].Type.(*TypeType).Type()
	}
	srcExpr := getSrc(src)
	tyRet, err := types.Instantiate(nil, args[0].Type, targs, sig.Variadic())
	if err != nil {
		_, pos := p.loadExpr(srcExpr)
		p.panicCodeErrorf(&pos, "instantiate error: %v", err)
	}
	elem := &internal.Elem{
		Val: &ast.IndexExpr{X: args[0].Val, Index: args[1].Val}, Type: tyRet, Src: srcExpr,
	}
	p.stk.Ret(nidx+1, elem)
	return p
}
