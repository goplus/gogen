//go:build go1.18
// +build go1.18

/*
 Copyright 2022 The GoPlus Authors (goplus.org)
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
	targs := make([]types.Type, nidx)
	for i := 0; i < nidx; i++ {
		targs[i] = args[i+1].Type.(*TypeType).Type()
		p.ensureLoaded(targs[i])
	}
	srcExpr := getSrc(src)
	tyRet, err := types.Instantiate(p.ctxt, args[0].Type, targs, true)
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

type typesContext = types.Context

func newTypesContext() *typesContext {
	return types.NewContext()
}
