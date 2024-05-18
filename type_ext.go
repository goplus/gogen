/*
 Copyright 2024 The GoPlus Authors (goplus.org)
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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// ----------------------------------------------------------------------------

// Go+ overload extended types
type OverloadType interface {
	At(i int) types.Object
	Len() int
}

var (
	_ OverloadType = (*TyOverloadNamed)(nil)
	_ OverloadType = (*TyOverloadFunc)(nil)
	_ OverloadType = (*TyOverloadMethod)(nil)
)

// ----------------------------------------------------------------------------

type iSubstType interface {
	Obj() types.Object
}

var (
	_ iSubstType = (*TyTemplateRecvMethod)(nil)
	_ iSubstType = (*TySubst)(nil)
)

// ----------------------------------------------------------------------------

// TyTypeEx is a TypeEx type.
type TyTypeEx interface {
	types.Type
	typeEx()
}

var (
	_ TyTypeEx = (*TyOverloadNamed)(nil)
	_ TyTypeEx = (*TyInstruction)(nil)
	_ TyTypeEx = (*TySubst)(nil)
)

// ----------------------------------------------------------------------------

// IsTypeEx returns if t is a gogen extended type or not.
func IsTypeEx(t types.Type) (ok bool) {
	switch v := t.(type) {
	case *types.Signature:
		_, ok = CheckFuncEx(v)
		return
	case TyTypeEx:
		return true
	}
	return false
}

// ----------------------------------------------------------------------------

type TyOverloadNamed struct {
	Types []*types.Named
	Obj   *types.TypeName
}

func (p *TyOverloadNamed) At(i int) types.Object { return p.Types[i].Obj() }
func (p *TyOverloadNamed) Len() int              { return len(p.Types) }

func (p *TyOverloadNamed) typeEx()                {}
func (p *TyOverloadNamed) Underlying() types.Type { return p }
func (p *TyOverloadNamed) String() string {
	o := p.Obj
	return fmt.Sprintf("TyOverloadNamed{%s.%s}", o.Pkg().Path(), o.Name())
}

func NewOverloadNamed(pos token.Pos, pkg *types.Package, name string, typs ...*types.Named) *types.TypeName {
	t := &TyOverloadNamed{Types: typs}
	sig := sigFuncEx(pkg, nil, t)
	o := types.NewTypeName(pos, pkg, name, sig)
	t.Obj = o
	return o
}

// CheckOverloadNamed returns if specified type is a TyOverloadNamed or not.
func CheckOverloadNamed(typ types.Type) (on *TyOverloadNamed, ok bool) {
	if sig, is := typ.(*types.Signature); is {
		if typ, is := CheckSigFuncEx(sig); is {
			on, ok = typ.(*TyOverloadNamed)
		}
	}
	return
}

type TyInstruction struct {
	instr Instruction
}

func (p *TyInstruction) typeEx()                {}
func (p *TyInstruction) Underlying() types.Type { return p }
func (p *TyInstruction) String() string {
	return fmt.Sprintf("TyInstruction{%T}", p.instr)
}

func NewInstruction(pos token.Pos, pkg *types.Package, name string, instr Instruction) *types.TypeName {
	return types.NewTypeName(pos, pkg, name, &TyInstruction{instr})
}

// Deprecated: use TySubst instead of SubstType.
type SubstType = TySubst

type TySubst struct {
	Real types.Object
}

func (p *TySubst) Obj() types.Object { return p.Real }

func (p *TySubst) typeEx()                {}
func (p *TySubst) Underlying() types.Type { return p }
func (p *TySubst) String() string {
	return fmt.Sprintf("substType{real: %v}", p.Real)
}

// TODO(xsw): check only c2go uses this function.
func NewSubst(pos token.Pos, pkg *types.Package, name string, real types.Object) *types.Var {
	return types.NewVar(pos, pkg, name, &TySubst{Real: real})
}

func LookupParent(scope *types.Scope, name string, pos token.Pos) (at *types.Scope, obj types.Object) {
	if at, obj = scope.LookupParent(name, pos); obj != nil {
		if t, ok := obj.Type().(*TySubst); ok {
			obj = t.Real
		}
	}
	return
}

func Lookup(scope *types.Scope, name string) (obj types.Object) {
	if obj = scope.Lookup(name); obj != nil {
		if t, ok := obj.Type().(*TySubst); ok {
			obj = t.Real
		}
	}
	return
}

// ----------------------------------------------------------------------------

var (
	TyByte = types.Universe.Lookup("byte").Type().(*types.Basic)
	TyRune = types.Universe.Lookup("rune").Type().(*types.Basic)
)

var (
	TyEmptyInterface = types.NewInterfaceType(nil, nil)
	TyError          = types.Universe.Lookup("error").Type()
)

// refType: &T
type refType struct {
	typ types.Type
}

func (p *refType) Elem() types.Type {
	return p.typ
}

func (p *refType) Underlying() types.Type { return p }
func (p *refType) String() string {
	return fmt.Sprintf("refType{typ: %v}", p.typ)
}

func DerefType(typ types.Type) (types.Type, bool) {
	switch t := typ.(type) {
	case *refType:
		return t.Elem(), true
	}
	return typ, false
}

// unboundType: unbound type
type unboundType struct {
	tBound types.Type
	ptypes []*ast.Expr
}

func (p *unboundType) boundTo(pkg *Package, arg types.Type) {
	if p.tBound != nil {
		fatal("TODO: type is already bounded")
	}
	p.tBound = arg
	for _, pt := range p.ptypes {
		*pt = toType(pkg, arg)
	}
	p.ptypes = nil
}

func (p *unboundType) Underlying() types.Type { return p }
func (p *unboundType) String() string {
	return fmt.Sprintf("unboundType{typ: %v}", p.tBound)
}

func realType(typ types.Type) types.Type {
	switch t := typ.(type) {
	case *unboundType:
		if t.tBound != nil {
			return t.tBound
		}
	case *types.Named:
		if tn := t.Obj(); tn.IsAlias() {
			return tn.Type()
		}
	}
	return typ
}

type unboundMapElemType struct {
	key types.Type
	typ *unboundType
}

func (p *unboundMapElemType) Underlying() types.Type { return p }
func (p *unboundMapElemType) String() string {
	return fmt.Sprintf("unboundMapElemType{key: %v}", p.key)
}

// ----------------------------------------------------------------------------

type btiMethodType struct {
	types.Type
	eargs []interface{}
}

// ----------------------------------------------------------------------------

type TypeType struct {
	typ types.Type
}

func NewTypeType(typ types.Type) *TypeType {
	return &TypeType{typ: typ}
}

func (p *TypeType) Pointer() *TypeType {
	return &TypeType{typ: types.NewPointer(p.typ)}
}

func (p *TypeType) Type() types.Type {
	return p.typ
}

func (p *TypeType) Underlying() types.Type { return p }
func (p *TypeType) String() string {
	return fmt.Sprintf("TypeType{typ: %v}", p.typ)
}

// ----------------------------------------------------------------------------
