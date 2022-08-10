/*
 Copyright 2021 The GoPlus Authors (goplus.org)
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

package types

import (
	"fmt"
	"go/token"
	"go/types"
)

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

func (p *TypeType) Underlying() types.Type {
	panic("type of type")
}

func (p *TypeType) String() string {
	return fmt.Sprintf("TypeType{typ: %v}", p.typ)
}

// ----------------------------------------------------------------------------

type SubstType struct {
	Real types.Object
}

func (p *SubstType) Underlying() types.Type {
	panic("substitute type")
}

func (p *SubstType) String() string {
	return fmt.Sprintf("substType{real: %v}", p.Real)
}

func NewSubst(pos token.Pos, pkg *types.Package, name string, real types.Object) *types.Var {
	return types.NewVar(pos, pkg, name, &SubstType{Real: real})
}

func LookupParent(scope *types.Scope, name string, pos token.Pos) (at *types.Scope, obj types.Object) {
	if at, obj = scope.LookupParent(name, pos); obj != nil {
		if t, ok := obj.Type().(*SubstType); ok {
			obj = t.Real
		}
	}
	return
}

func Lookup(scope *types.Scope, name string) (obj types.Object) {
	if obj = scope.Lookup(name); obj != nil {
		if t, ok := obj.Type().(*SubstType); ok {
			obj = t.Real
		}
	}
	return
}

// ----------------------------------------------------------------------------

// NewCSignature creates prototype of a C function.
func NewCSignature(params, results *types.Tuple, variadic bool) *types.Signature {
	crecv := types.NewParam(token.NoPos, nil, "", types.Typ[types.UntypedNil])
	return types.NewSignature(crecv, params, results, variadic)
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

// ----------------------------------------------------------------------------
