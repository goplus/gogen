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

package gox

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

// A Variable represents a function parameters and results.
type Param = types.Var

// NewAutoParam returns a new variable representing a function result parameter with auto type.
func (p *Package) NewAutoParam(name string) *Param {
	return types.NewParam(token.NoPos, p.Types, name, &unboundType{})
}

// NewParam returns a new variable representing a function parameter.
func (p *Package) NewParam(pos token.Pos, name string, typ types.Type) *Param {
	return types.NewParam(pos, p.Types, name, typ)
}

// ----------------------------------------------------------------------------

// A Tuple represents an ordered list of variables; a nil *Tuple is a valid (empty) tuple.
// Tuples are used as components of signatures and to represent the type of multiple
// assignments; they are not first class types of Go.
type Tuple = types.Tuple

// NewTuple returns a new tuple for the given parameters.
func NewTuple(x ...*Param) *Tuple {
	return types.NewTuple(x...)
}

// ----------------------------------------------------------------------------

// Func type
type Func struct {
	*types.Func
	decl *ast.FuncDecl
	old  funcBodyCtx
}

// Ancestor returns ancestor of a closure function.
// It returns itself if the specified func is a normal function.
func (p *Func) Ancestor() *Func {
	for {
		if fn := p.old.fn; fn != nil {
			p = fn
			continue
		}
		return p
	}
}

// BodyStart func
func (p *Func) BodyStart(pkg *Package) *CodeBuilder {
	if debugInstr {
		var recv string
		tag := "NewFunc "
		name := p.Name()
		sig := p.Type().(*types.Signature)
		if v := sig.Recv(); v != nil {
			recv = fmt.Sprintf(" (%v)", v.Type())
		}
		if name == "" {
			tag = "NewClosure"
		}
		log.Printf("%v%v%v %v\n", tag, name, recv, sig)
	}
	return pkg.cb.startFuncBody(p, &p.old)
}

// End is for internal use.
func (p *Func) End(cb *CodeBuilder) {
	if p.isInline() {
		p.inlineClosureEnd(cb)
		return
	}
	pkg := cb.pkg
	body := &ast.BlockStmt{List: cb.endFuncBody(p.old)}
	t, _ := toNormalizeSignature(nil, p.Type().(*types.Signature))
	if fn := p.decl; fn == nil { // is closure
		expr := &ast.FuncLit{Type: toFuncType(pkg, t), Body: body}
		cb.stk.Push(&internal.Elem{Val: expr, Type: t})
	} else {
		fn.Name, fn.Type, fn.Body = ident(p.Name()), toFuncType(pkg, t), body
		if recv := t.Recv(); recv != nil {
			fn.Recv = toRecv(pkg, recv)
		}
	}
}

// NewFunc func
func (p *Package) NewFunc(recv *Param, name string, params, results *Tuple, variadic bool) *Func {
	sig := types.NewSignature(recv, params, results, variadic)
	fn, err := p.NewFuncWith(token.NoPos, name, sig, nil)
	if err != nil {
		panic(err)
	}
	return fn
}

func getRecv(recvTypePos func() token.Pos) token.Pos {
	if recvTypePos != nil {
		return recvTypePos()
	}
	return token.NoPos
}

// NewFuncWith func
func (p *Package) NewFuncWith(
	pos token.Pos, name string, sig *types.Signature, recvTypePos func() token.Pos) (*Func, error) {
	if name == "" {
		panic("no func name")
	}
	cb := p.cb
	fn := types.NewFunc(pos, p.Types, name, sig)
	if recv := sig.Recv(); recv != nil { // add method to this type
		var t *types.Named
		var ok bool
		var typ = recv.Type()
		switch tt := typ.(type) {
		case *types.Named:
			t, ok = tt, true
		case *types.Pointer:
			typ = tt.Elem()
			t, ok = typ.(*types.Named)
		}
		if !ok {
			return nil, cb.newCodePosErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is not a defined type)", typ, typ)
		}
		switch getUnderlying(p, t.Obj().Type()).(type) {
		case *types.Interface:
			return nil, cb.newCodePosErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is an interface type)", typ, typ)
		case *types.Pointer:
			return nil, cb.newCodePosErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is a pointer type)", typ, typ)
		}
		if fn.Name() != "_" { // skip underscore
			t.AddMethod(fn)
		}
	} else if name == "init" { // init is not a normal func
		if sig.Params() != nil || sig.Results() != nil {
			return nil, cb.newCodePosError(
				pos, "func init must have no arguments and no return values")
		}
	} else if fn.Name() != "_" { // skip underscore
		p.Types.Scope().Insert(fn)
	}

	decl := &ast.FuncDecl{}
	idx := p.testingFile
	p.files[idx].decls = append(p.files[idx].decls, decl)
	return &Func{Func: fn, decl: decl}, nil
}

type closureType = token.Pos

const (
	closureNormal     closureType = 0
	closureFlagInline closureType = (1 << 30)
)

func (p *Package) newClosure(sig *types.Signature, ct closureType) *Func {
	fn := types.NewFunc(ct, p.Types, "", sig)
	return &Func{Func: fn}
}

func (p *Func) isInline() bool {
	return (p.Pos() & closureFlagInline) != 0
}

// ----------------------------------------------------------------------------

func NewOverloadFunc(pos token.Pos, pkg *types.Package, name string, funcs ...types.Object) *types.TypeName {
	return types.NewTypeName(pos, pkg, name, &overloadFuncType{funcs})
}

func NewOverloadMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, funcs ...types.Object) *types.Func {
	oft := &overloadFuncType{funcs}
	recv := types.NewParam(token.NoPos, pkg, "", oft)
	sig := types.NewSignature(recv, nil, nil, false)
	ofn := types.NewFunc(pos, pkg, name, sig)
	typ.AddMethod(ofn)
	return ofn
}

func CheckOverloadMethod(sig *types.Signature) (funcs []types.Object, ok bool) {
	if recv := sig.Recv(); recv != nil {
		if oft, ok := recv.Type().(*overloadFuncType); ok {
			return oft.funcs, true
		}
	}
	return nil, false
}

// NewTemplateRecvMethod - https://github.com/goplus/gop/issues/811
func NewTemplateRecvMethod(typ *types.Named, pos token.Pos, pkg *types.Package, name string, fn types.Object) *types.Func {
	trmt := &templateRecvMethodType{fn}
	recv := types.NewParam(token.NoPos, pkg, "", trmt)
	sig := types.NewSignature(recv, nil, nil, false)
	ofn := types.NewFunc(pos, pkg, name, sig)
	typ.AddMethod(ofn)
	return ofn
}

func CheckSignature(typ types.Type, idx, nin int) *types.Signature {
	switch t := typ.(type) {
	case *types.Signature:
		if funcs, ok := CheckOverloadMethod(t); ok {
			for _, v := range funcs {
				if sig, ok := v.Type().(*types.Signature); ok {
					params := sig.Params()
					if idx < params.Len() && checkSigParam(params.At(idx), nin) {
						return sig
					}
				}
			}
		} else {
			return t
		}
	case *templateRecvMethodType:
		if sig, ok := t.fn.Type().(*types.Signature); ok {
			params := sig.Params()
			n := params.Len()
			mparams := make([]*types.Var, n-1)
			for i := range mparams {
				mparams[i] = params.At(i + 1)
			}
			return types.NewSignature(nil, types.NewTuple(mparams...), sig.Results(), sig.Variadic())
		}
	}
	log.Println("[ERROR] CheckSignature: not found -", typ)
	return nil
}

func checkSigParam(v *types.Var, nin int) bool {
	typ := v.Type()
	if nin < 0 { // input is CompositeLit
		if t, ok := typ.(*types.Pointer); ok {
			typ = t.Elem()
		}
		switch typ.(type) {
		case *types.Struct, *types.Named:
			return true
		}
	} else if t, ok := typ.(*types.Signature); ok {
		return t.Params().Len() == nin
	}
	return false
}

// ----------------------------------------------------------------------------

type Element = internal.Elem
type InstrFlags = token.Pos

const (
	InstrFlagEllipsis InstrFlags = 1 << iota
	InstrFlagTwoValue
	instrFlagApproxType // restricts to all types whose underlying type is T
)

type Instruction interface {
	Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error)
}

func NewInstruction(pos token.Pos, pkg *types.Package, name string, instr Instruction) *types.TypeName {
	return types.NewTypeName(pos, pkg, name, &instructionType{instr})
}

// ----------------------------------------------------------------------------
