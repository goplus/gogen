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
	"go/token"
	"go/types"

	"github.com/goplus/gox/internal"
)

const enableTypeParams = false
const unsupported_typeparams = "type parameters are unsupported at this go version"

type TypeParam struct{ types.Type }

func (*TypeParam) String() string           { panic(unsupported_typeparams) }
func (*TypeParam) Underlying() types.Type   { panic(unsupported_typeparams) }
func (*TypeParam) Index() int               { panic(unsupported_typeparams) }
func (*TypeParam) Constraint() types.Type   { panic(unsupported_typeparams) }
func (*TypeParam) SetConstraint(types.Type) { panic(unsupported_typeparams) }
func (*TypeParam) Obj() *types.TypeName     { panic(unsupported_typeparams) }

func (p *CodeBuilder) inferType(nidx int, args []*internal.Elem, src ...ast.Node) *CodeBuilder {
	panic(unsupported_typeparams)
}

type typesContext struct{}

func newTypesContext() *typesContext {
	return &typesContext{}
}

func toNamedType(pkg *Package, t *types.Named) ast.Expr {
	return toObjectExpr(pkg, t.Obj())
}

type positioner interface {
	Pos() token.Pos
}

func inferFunc(pkg *Package, fn *internal.Elem, sig *types.Signature, targs []types.Type, args []*internal.Elem, flags InstrFlags) (types.Type, error) {
	panic(unsupported_typeparams)
}

func funcHasTypeParams(t *types.Signature) bool {
	return false
}

type inferFuncType struct {
}

func (p *inferFuncType) Type() types.Type {
	panic(unsupported_typeparams)
}

func (p *inferFuncType) Underlying() types.Type {
	panic(unsupported_typeparams)
}

func (p *inferFuncType) String() string {
	panic(unsupported_typeparams)
}

func (p *inferFuncType) Instance() *types.Signature {
	panic(unsupported_typeparams)
}

func (p *inferFuncType) InstanceWithArgs(args []*internal.Elem, flags InstrFlags) *types.Signature {
	panic(unsupported_typeparams)
}
