//go:build !go1.18
// +build !go1.18

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

	"github.com/goplus/gox/internal"
)

func (p *CodeBuilder) instantiate(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	panic("type parameters are unsupported at this go version")
}

type typesContext struct{}

func newTypesContext() *typesContext {
	return &typesContext{}
}

func toNamedType(pkg *Package, t *types.Named) ast.Expr {
	return toObjectExpr(pkg, t.Obj())
}
