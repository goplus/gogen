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

package gogen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/goplus/gogen/internal"
)

// ----------------------------------------------------------------------------

// A Param represents a function parameters and results.
type Param = types.Var

// NewAutoParam returns a new variable representing a function result parameter with auto type.
func (p *Package) NewAutoParam(name string) *Param {
	return p.NewAutoParamEx(token.NoPos, name)
}

// NewAutoParamEx returns a new variable representing a function result parameter with auto type.
func (p *Package) NewAutoParamEx(pos token.Pos, name string) *Param {
	return types.NewParam(pos, p.Types, name, &unboundType{})
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
	decl   *ast.FuncDecl
	old    funcBodyCtx
	arity1 int // 0 for normal, (arity+1) for inlineClosure
}

// Obj returns this function object.
func (p *Func) Obj() types.Object {
	return p.Func
}

// Comments returns associated documentation.
func (p *Func) Comments() *ast.CommentGroup {
	return p.decl.Doc
}

// SetComments sets associated documentation.
func (p *Func) SetComments(pkg *Package, doc *ast.CommentGroup) *Func {
	p.decl.Doc = doc
	pkg.setDoc(p.Func, doc)
	return p
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
func (p *Func) BodyStart(pkg *Package, src ...ast.Node) *CodeBuilder {
	if debugInstr {
		var recv string
		tag := "NewFunc "
		name := p.Name()
		sig := p.Type().(*types.Signature)
		if v := sig.Recv(); IsMethodRecv(v) {
			recv = fmt.Sprintf(" (%v)", v.Type())
		}
		if name == "" {
			tag = "NewClosure"
		}
		log.Printf("%v%v%v %v\n", tag, name, recv, sig)
	}
	return pkg.cb.startFuncBody(p, src, &p.old)
}

// End is for internal use.
func (p *Func) End(cb *CodeBuilder, src ast.Node) {
	if p.isInline() {
		p.inlineClosureEnd(cb)
		return
	}
	pkg := cb.pkg
	body := &ast.BlockStmt{List: cb.endFuncBody(p.old)}
	t, _ := toNormalizeSignature(nil, p.Type().(*types.Signature))
	if fn := p.decl; fn == nil { // is closure
		expr := &ast.FuncLit{Type: toFuncType(pkg, t), Body: body}
		cb.stk.Push(&internal.Elem{Val: expr, Type: t, Src: src})
	} else {
		fn.Name, fn.Type, fn.Body = ident(p.Name()), toFuncType(pkg, t), body
		if recv := t.Recv(); IsMethodRecv(recv) {
			fn.Recv = toRecv(pkg, recv)
		}
	}
}

// NewFuncDecl creates a new function without function body (declaration only).
func (p *Package) NewFuncDecl(pos token.Pos, name string, sig *types.Signature) *Func {
	f, err := p.NewFuncWith(pos, name, sig, nil)
	if err != nil {
		panic(err)
	}
	fn := f.decl
	fn.Name, fn.Type = ident(name), toFuncType(p, sig)
	return f
}

// NewFunc creates a new function (should have a function body).
func (p *Package) NewFunc(recv *Param, name string, params, results *Tuple, variadic bool) *Func {
	sig := types.NewSignatureType(recv, nil, nil, params, results, variadic)
	f, err := p.NewFuncWith(token.NoPos, name, sig, nil)
	if err != nil {
		panic(err)
	}
	return f
}

func getRecv(recvTypePos func() token.Pos) token.Pos {
	if recvTypePos != nil {
		return recvTypePos()
	}
	return token.NoPos
}

func IsMethodRecv(recv *types.Var) bool {
	return recv != nil
}

// NewFuncWith creates a new function (should have a function body).
func (p *Package) NewFuncWith(
	pos token.Pos, name string, sig *types.Signature, recvTypePos func() token.Pos) (*Func, error) {
	if name == "" {
		panic("no func name")
	}
	cb := p.cb
	fn := &Func{Func: types.NewFunc(pos, p.Types, name, sig)}
	if recv := sig.Recv(); IsMethodRecv(recv) { // add method to this type
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
			return nil, cb.newCodeErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is not a defined type)", typ, typ)
		}
		switch getUnderlying(p, t.Obj().Type()).(type) {
		case *types.Interface:
			return nil, cb.newCodeErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is an interface type)", typ, typ)
		case *types.Pointer:
			return nil, cb.newCodeErrorf(
				getRecv(recvTypePos), "invalid receiver type %v (%v is a pointer type)", typ, typ)
		}
		if name != "_" { // skip underscore
			t.AddMethod(fn.Func)
		}
	} else if name == "init" { // init is not a normal func
		if sig.Params() != nil || sig.Results() != nil {
			return nil, cb.newCodeErrorf(
				pos, "func init must have no arguments and no return values")
		}
	} else if name != "_" { // skip underscore
		old := p.Types.Scope().Insert(fn.Obj())
		if old != nil {
			if !(p.allowRedecl && types.Identical(old.Type(), sig)) { // for c2go
				oldPos := cb.fset.Position(old.Pos())
				return nil, cb.newCodeErrorf(
					pos, "%s redeclared in this block\n\t%v: other declaration of %s", name, oldPos, name)
			}
		}
		p.useName(name)
	}

	if isGopFunc(name) {
		p.isGopPkg = true
	}
	if token.IsExported(name) {
		p.expObjTypes = append(p.expObjTypes, sig)
	}

	fn.decl = &ast.FuncDecl{}
	p.file.decls = append(p.file.decls, fn.decl)
	return fn, nil
}

func (p *Package) newClosure(sig *types.Signature) *Func {
	fn := types.NewFunc(token.NoPos, p.Types, "", sig)
	return &Func{Func: fn}
}

func (p *Package) newInlineClosure(sig *types.Signature, arity int) *Func {
	fn := types.NewFunc(token.NoPos, p.Types, "", sig)
	return &Func{Func: fn, arity1: arity + 1}
}

func (p *Func) isInline() bool {
	return p.arity1 != 0
}

func (p *Func) getInlineCallArity() int {
	return p.arity1 - 1
}

// ----------------------------------------------------------------------------

type Element = internal.Elem
type InstrFlags token.Pos

const (
	InstrFlagEllipsis InstrFlags = 1 << iota
	InstrFlagTwoValue

	instrFlagApproxType // restricts to all types whose underlying type is T
	instrFlagGopxFunc   // call Gopx_xxx function
	instrFlagGoptFunc   // call Gopt_xxx function
	instrFlagOpFunc     // from callOpFunc
	instrFlagBinaryOp   // from cb.BinaryOp
)

type Instruction interface {
	Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error)
}

// ----------------------------------------------------------------------------
