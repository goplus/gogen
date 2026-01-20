/*
 Copyright 2026 The XGo Authors (xgo.dev)
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

package gogen

import (
	"go/ast"
	"go/types"
	"strconv"
)

// ----------------------------------------------------------------------------

/*
// vFields defines the interface for virtual fields of a struct type.
type vFields interface { // virtual fields
	FindField(cb *CodeBuilder, t *types.Struct, name string, arg *Element, src ast.Node) bool
	FieldRef(cb *CodeBuilder, t *types.Struct, name string, x ast.Expr, src ast.Node) bool
}
*/

type vFields = *tupleFields

type vFieldsMgr struct {
	vfts map[*types.Struct]vFields
}

func (p *CodeBuilder) refVField(t *types.Struct, name string, x ast.Expr, src ast.Node) bool {
	if vft, ok := p.vfts[t]; ok {
		return vft.FieldRef(p, t, name, x, src)
	}
	return false
}

func (p *CodeBuilder) findVField(t *types.Struct, name string, arg *Element, src ast.Node) bool {
	if vft, ok := p.vfts[t]; ok {
		return vft.FindField(p, t, name, arg, src)
	}
	return false
}

func (p *Package) setVFields(t *types.Struct, vft vFields) {
	if p.cb.vfts == nil {
		p.cb.vfts = make(map[*types.Struct]vFields)
	}
	p.cb.vfts[t] = vft
}

/*
func (p *Package) vFields(t *types.Struct) (vft vFields, ok bool) {
	vft, ok = p.cb.vfts[t]
	return
}
*/

// ----------------------------------------------------------------------------

type tupleFields struct {
	fields []*types.Var
}

func (p *tupleFields) FindField(cb *CodeBuilder, t *types.Struct, name string, arg *Element, src ast.Node) bool {
	for i, fld := range p.fields {
		if fld.Name() == name {
			cb.stk.Ret(1, &Element{
				Val:  selector(arg, tupleFieldName(i)),
				Type: fld.Type(),
				Src:  src,
			})
			if cb.rec != nil {
				cb.rec.Member(src, fld)
			}
			return true
		}
	}
	return false
}

func (p *tupleFields) FieldRef(cb *CodeBuilder, t *types.Struct, name string, x ast.Expr, src ast.Node) bool {
	for i, fld := range p.fields {
		if fld.Name() == name {
			if cb.rec != nil {
				cb.rec.Member(src, fld)
			}
			ordName := tupleFieldName(i)
			cb.stk.Ret(1, &Element{
				Val:  &ast.SelectorExpr{X: x, Sel: ident(ordName)},
				Type: &refType{typ: fld.Type()},
			})
			return true
		}
	}
	return false
}

// NewTuple creates a tuple type with the given fields.
// The fields are named as _0, _1, ...
// If withName is true, the original fields can also be accessed through
// virtual fields mechanism.
func (p *Package) NewTuple(withName bool, fields ...*types.Var) *types.Struct {
	ordinals := make([]*types.Var, len(fields))
	for i, fld := range fields {
		name := tupleFieldName(i)
		ordinals[i] = types.NewVar(fld.Pos(), fld.Pkg(), name, fld.Type())
	}
	ret := types.NewStruct(ordinals, nil)
	if withName {
		p.setVFields(ret, &tupleFields{fields})
	}
	return ret
}

func tupleFieldName(i int) string {
	return "_" + strconv.Itoa(i)
}

// ----------------------------------------------------------------------------
