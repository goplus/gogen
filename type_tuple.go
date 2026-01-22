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
	"go/token"
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

// IsTupleType reports whether typ is a tuple type.
func (p *CodeBuilder) IsTupleType(typ types.Type) bool {
	return checkTupleType(typ) != nil
}

func checkTupleType(typ types.Type) (result *types.Struct) {
	result, _ = typ.Underlying().(*types.Struct)
	if result != nil {
		if result.NumFields() > 0 && result.Field(0).Name() != tupleField0 {
			result = nil
		}
	}
	return
}

func (p *CodeBuilder) tryUnpackTuple() int {
	e := p.stk.Get(-1)
	tuple := e.Type
	if t := checkTupleType(tuple); t != nil {
		n := t.NumFields()
		p.stk.PopN(1)
		val := e.Val
		if _, ok := val.(*ast.Ident); ok {
			for i := 0; i < n; i++ {
				p.stk.Push(e)
				p.MemberVal(tupleFieldName(i))
			}
			return n
		} else {
			pkg := p.pkg
			pkgType := pkg.Types
			arg := types.NewParam(token.NoPos, pkgType, "v", tuple)
			result := make([]*types.Var, n)
			for i := 0; i < n; i++ {
				result[i] = types.NewParam(token.NoPos, pkgType, "", t.Field(i).Type())
			}
			p.NewClosure(types.NewTuple(arg), types.NewTuple(result...), false).BodyStart(pkg)
			for i := 0; i < n; i++ {
				p.Val(arg).MemberVal(tupleFieldName(i))
			}
			p.Return(n).End()
			p.stk.Push(e)
			p.Call(1)
		}
	}
	return 1
}

// LookupField looks up a field by name in the given struct type t.
// It returns the field index if found, or -1 if not found.
// It checks both the original fields and the virtual fields (e.g. tuple
// fields).
func (p *CodeBuilder) LookupField(t *types.Struct, name string) int {
	for i, n := 0, t.NumFields(); i < n; i++ {
		if fld := t.Field(i); fld.Name() == name {
			return i
		}
	}
	if vft, ok := p.vfts[t]; ok {
		for i, fld := range vft.fields {
			if fld.Name() == name {
				return i
			}
		}
	}
	return -1
}

// TupleLit creates a tuple literal.
func (p *CodeBuilder) TupleLit(typ types.Type, arity int, src ...ast.Node) *CodeBuilder {
	if typ == nil {
		pkg := p.pkg
		pkgTypes := pkg.Types
		args := p.stk.GetArgs(arity)
		flds := make([]*types.Var, arity)
		for i := 0; i < arity; i++ {
			fldt := types.Default(args[i].Type)
			flds[i] = types.NewField(token.NoPos, pkgTypes, "", fldt, false)
		}
		typ = pkg.NewTuple(false, flds...)
	}
	return p.StructLit(typ, arity, false, src...)
}

// NewTuple creates a tuple type with the given fields.
// The fields are named as X_0, X_1, ...
// If withName is true, the original fields can also be accessed by their
// original names in addition to the ordinal names (X_0, X_1, ...).
// For example, a field named "x" can be accessed as both tuple.X_0 and
// tuple.x.
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

const tupleField0 = "X_0"

func tupleFieldName(i int) string {
	return "X_" + strconv.Itoa(i)
}

// ----------------------------------------------------------------------------
