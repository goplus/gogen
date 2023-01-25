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

// Term holds information about a structural type restriction.
type Term struct {
	tilde bool
	typ   types.Type
}

func (m *Term) Tilde() bool      { return m.tilde }
func (m *Term) Type() types.Type { return m.typ }
func (m *Term) String() string {
	pre := ""
	if m.tilde {
		pre = "~"
	}
	return pre + m.typ.String()
}

// NewTerm creates a new placeholder term type.
func NewTerm(tilde bool, typ types.Type) *Term {
	return &Term{tilde, typ}
}

// Union is a placeholder type, as type parameters are not supported at this Go
// version. Its methods panic on use.
type Union struct{ types.Type }

func (*Union) String() string         { panic(unsupported_typeparams) }
func (*Union) Underlying() types.Type { panic(unsupported_typeparams) }
func (*Union) Len() int               { return 0 }
func (*Union) Term(i int) *Term       { panic(unsupported_typeparams) }

// NewUnion is unsupported at this Go version, and panics.
func NewUnion(terms []*Term) *Union {
	panic(unsupported_typeparams)
}

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

func toFuncType(pkg *Package, sig *types.Signature) *ast.FuncType {
	params := toFieldList(pkg, sig.Params())
	results := toFieldList(pkg, sig.Results())
	if sig.Variadic() {
		n := len(params)
		if n == 0 {
			panic("TODO: toFuncType error")
		}
		toVariadic(params[n-1])
	}
	return &ast.FuncType{
		Params:  &ast.FieldList{List: params},
		Results: &ast.FieldList{List: results},
	}
}

func toUnionType(pkg *Package, t *Union) ast.Expr {
	panic(unsupported_typeparams)
}

// NewFuncType creates a new function type for the given receiver, name,
// receiver type parameters, type parameters, parameters, and results.
func (p *Package) NewFuncType(recv *Param, name string, recvTypeParams, typeParams []*TypeParam, params, results *Tuple, variadic bool) *Func {
	panic(unsupported_typeparams)
}

func setTypeParams(pkg *Package, typ *types.Named, spec *ast.TypeSpec, tparams []*TypeParam) {
}
