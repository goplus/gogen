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

package gogen

import (
	"go/ast"
	"go/types"
)

/*
import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/goplus/gogen/internal"
)

// ----------------------------------------------------------------------------

type VFields interface { // virtual fields
	FindField(cb *CodeBuilder, t *types.Named, name string, arg *Element, src ast.Node) MemberKind
	FieldRef(cb *CodeBuilder, t *types.Named, name string, src ast.Node) MemberKind
}

type none = struct{}

type vFieldsMgr struct {
	vfts map[*types.Named]VFields
	pubs map[*types.Named]none
}

func CPubName(name string) string {
	if r := name[0]; 'a' <= r && r <= 'z' {
		r -= 'a' - 'A'
		return string(r) + name[1:]
	} else if r == '_' {
		return "X" + name
	}
	return name
}

func (p *CodeBuilder) getFieldName(t *types.Named, name string) string {
	if _, ok := p.pubs[t]; ok {
		return CPubName(name)
	}
	return name
}

func (p *CodeBuilder) refVField(t *types.Named, name string, src ast.Node) MemberKind {
	if vft, ok := p.vfts[t]; ok {
		return vft.FieldRef(p, t, name, src)
	}
	return MemberInvalid
}

func (p *CodeBuilder) findVField(t *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	if vft, ok := p.vfts[t]; ok {
		return vft.FindField(p, t, name, arg, src)
	}
	return MemberInvalid
}

func (p *Package) ExportFields(t *types.Named) {
	if p.cb.pubs == nil {
		p.cb.pubs = make(map[*types.Named]none)
	}
	p.cb.pubs[t] = none{}
}

func (p *Package) SetVFields(t *types.Named, vft VFields) {
	if p.cb.vfts == nil {
		p.cb.vfts = make(map[*types.Named]VFields)
	}
	p.cb.vfts[t] = vft
}

func (p *Package) VFields(t *types.Named) (vft VFields, ok bool) {
	vft, ok = p.cb.vfts[t]
	return
}

// ----------------------------------------------------------------------------

type BitField struct {
	Name    string // bit field name
	FldName string // real field name
	Off     int
	Bits    int
	Pos     token.Pos
}

type BitFields struct {
	flds []*BitField
}

func NewBitFields(flds []*BitField) *BitFields {
	return &BitFields{flds: flds}
}

func (p *BitFields) At(i int) *BitField {
	return p.flds[i]
}

func (p *BitFields) Len() int {
	return len(p.flds)
}

func (p *BitFields) FindField(
	cb *CodeBuilder, t *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	for _, v := range p.flds {
		if v.Name == name {
			o := t.Underlying().(*types.Struct)
			if kind := cb.field(o, v.FldName, "", MemberFlagVal, arg, src); kind != MemberInvalid {
				tfld := cb.stk.Get(-1).Type.(*types.Basic)
				if (tfld.Info() & types.IsUnsigned) != 0 {
					if v.Off != 0 {
						cb.Val(v.Off).BinaryOp(token.SHR)
					}
					cb.Val((1 << v.Bits) - 1).BinaryOp(token.AND)
				} else {
					bits := int(std.Sizeof(tfld)<<3) - v.Bits
					cb.Val(bits - v.Off).BinaryOp(token.SHL).Val(bits).BinaryOp(token.SHR)
				}
				return kind
			}
		}
	}
	return MemberInvalid
}

func (p *bfRefType) assign(cb *CodeBuilder, lhs, rhs *ast.Expr) {
	// *addr = *addr &^ ((1 << bits) - 1) << off) | ((rhs & (1 << bits) - 1)) << off)
	tname := cb.pkg.autoName()
	tvar := ident(tname)
	addr := &ast.UnaryExpr{Op: token.AND, X: *lhs}
	stmt := &ast.AssignStmt{Lhs: []ast.Expr{tvar}, Tok: token.DEFINE, Rhs: []ast.Expr{addr}}
	cb.emitStmt(stmt)
	mask0 := (1 << p.bits) - 1
	mask := mask0 << p.off
	maskLit0 := &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(mask0)}
	maskLit := &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(mask)}
	valMask := &ast.BinaryExpr{X: &ast.StarExpr{X: tvar}, Op: token.AND_NOT, Y: maskLit}
	rhsExpr := &ast.BinaryExpr{X: *rhs, Op: token.AND, Y: maskLit0}
	if p.off != 0 {
		offLit := &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(p.off)}
		rhsExpr = &ast.BinaryExpr{X: rhsExpr, Op: token.SHL, Y: offLit}
	}
	*lhs = &ast.StarExpr{X: tvar}
	*rhs = &ast.BinaryExpr{X: valMask, Op: token.OR, Y: rhsExpr}
}

func (p *BitFields) FieldRef(cb *CodeBuilder, t *types.Named, name string, src ast.Node) MemberKind {
	for _, v := range p.flds {
		if v.Name == name {
			stk := cb.stk
			o := t.Underlying().(*types.Struct)
			if cb.fieldRef(stk.Get(-1).Val, o, v.FldName, src) {
				fld := stk.Get(-1)
				tfld := fld.Type.(*refType).typ.(*types.Basic)
				stk.Ret(1, &internal.Elem{
					Val: fld.Val, Src: src,
					Type: &bfRefType{typ: tfld, bits: v.Bits, off: v.Off},
				})
				return MemberField
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
	Pos  token.Pos
}

type UnionFields struct {
	flds []*UnionField
}

func NewUnionFields(flds []*UnionField) *UnionFields {
	return &UnionFields{flds: flds}
}

func (p *UnionFields) At(i int) *UnionField {
	return p.flds[i]
}

func (p *UnionFields) Len() int {
	return len(p.flds)
}

func (p *UnionFields) getField(
	cb *CodeBuilder, tfld *types.Named, name string, _ ast.Node, ref bool) MemberKind {
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
			cb.Call(1).Call(1) // => voidptr => *type
			if ref {
				cb.ElemRef()
			} else {
				cb.Elem()
			}
			return MemberField
		}
	}
	return MemberInvalid
}

func (p *UnionFields) FindField(
	cb *CodeBuilder, tfld *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	return p.getField(cb, tfld, name, src, false)
}

func (p *UnionFields) FieldRef(cb *CodeBuilder, tfld *types.Named, name string, src ast.Node) MemberKind {
	return p.getField(cb, tfld, name, src, true)
}

// ----------------------------------------------------------------------------

// NewCSignature creates prototype of a C function.
func NewCSignature(params, results *types.Tuple, variadic bool) *types.Signature {
	crecv := types.NewParam(token.NoPos, nil, "", types.Typ[types.UntypedNil])
	return types.NewSignatureType(crecv, nil, nil, params, results, variadic)
}

// IsCSignature checks a prototype is C function or not.
func IsCSignature(sig *types.Signature) bool {
	recv := sig.Recv()
	return recv != nil && isCSigRecv(recv)
}

func IsMethodRecv(recv *types.Var) bool {
	return recv != nil && !isCSigRecv(recv)
}

func isCSigRecv(recv *types.Var) bool {
	return recv.Type() == types.Typ[types.UntypedNil]
}
*/

type none = struct{}

// ----------------------------------------------------------------------------

func (p *CodeBuilder) getFieldName(t *types.Named, name string) string {
	return name
}

func (p *CodeBuilder) findVField(t *types.Named, name string, arg *Element, src ast.Node) MemberKind {
	return MemberInvalid
}

func (p *CodeBuilder) refVField(t *types.Named, name string, src ast.Node) MemberKind {
	return MemberInvalid
}

func IsMethodRecv(recv *types.Var) bool {
	return recv != nil
}

// ----------------------------------------------------------------------------
