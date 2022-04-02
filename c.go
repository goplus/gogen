package gox

import (
	"go/ast"
	"go/token"
	"go/types"
)

// ----------------------------------------------------------------------------

type VFields interface { // virtual fields
	FindField(cb *CodeBuilder, t *types.Named, name string, arg *Element, src ast.Node) MemberKind
}

type vFieldsMgr struct {
	vfts map[*types.Named]VFields
}

func (p *CodeBuilder) findVField(t *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	if vft, ok := p.vfts[t]; ok {
		return vft.FindField(p, t, name, arg, src)
	}
	return MemberInvalid
}

func (p *Package) SetVFields(t *types.Named, vft VFields) {
	if p.cb.vfts == nil {
		p.cb.vfts = make(map[*types.Named]VFields)
	}
	p.cb.vfts[t] = vft
}

// ----------------------------------------------------------------------------

type BitField struct {
	Name    string // bit field name
	FldName string // real field name
	Off     int
	Bits    int
}

type BitFields struct {
	flds []*BitField
}

func NewBitFields(flds []*BitField) *BitFields {
	return &BitFields{flds: flds}
}

func (p *BitFields) FindField(
	cb *CodeBuilder, t *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	for _, v := range p.flds {
		if v.Name == name {
			o := t.Underlying().(*types.Struct)
			if kind := cb.field(o, v.FldName, "", MemberFlagVal, arg, src); kind != MemberInvalid {
				if v.Off != 0 {
					cb.Val(v.Off).BinaryOp(token.SHR)
				}
				cb.Val((1 << v.Bits) - 1).BinaryOp(token.AND)
				return kind
			}
		}
	}
	return MemberInvalid
}

// ----------------------------------------------------------------------------

type UnionField struct {
	Name string
	Off  int
	Type types.Type
}

type UnionFields struct {
	flds []*UnionField
}

func NewUnionFields(flds []*UnionField) *UnionFields {
	return &UnionFields{flds: flds}
}

func (p *UnionFields) FindField(
	cb *CodeBuilder, tfld *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	for _, v := range p.flds {
		if v.Name == name {
			obj := cb.stk.Pop()
			tobj, isPtr := obj.Type, false
			if tt, ok := tobj.(*types.Pointer); ok {
				tobj, isPtr = tt.Elem(), true
			}
			cb.Typ(types.NewPointer(v.Type)).Typ(types.Typ[types.UnsafePointer])
			if v.Off != 0 {
				cb.Typ(types.Typ[types.Uintptr]).Typ(types.Typ[types.UnsafePointer])
			}
			cb.stk.Push(obj)
			if tt, ok := tobj.(*types.Named); ok && tt == tfld { // it's an union type
				if !isPtr {
					cb.UnaryOp(token.AND)
				}
			} else { // it's a type contains a field with union type
				cb.MemberRef(tfld.Obj().Name()).UnaryOp(token.AND)
			}
			if v.Off != 0 {
				cb.Call(1).Call(1).Val(v.Off).BinaryOp(token.ADD) // => voidptr => uintptr
			}
			cb.Call(1).Call(1).Elem() // => voidptr => *type
			return MemberField
		}
	}
	return MemberInvalid
}

// ----------------------------------------------------------------------------
