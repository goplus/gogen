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
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/tools/go/types/typeutil"
)

func newBuiltinDefault(pkg *Package, conf *Config) *types.Package {
	builtin := types.NewPackage("", "")
	InitBuiltin(pkg, builtin, conf)
	return builtin
}

func InitBuiltin(pkg *Package, builtin *types.Package, conf *Config) {
	initBuiltinOps(builtin, conf)
	initBuiltinAssignOps(builtin)
	initBuiltinFuncs(builtin)
	initBuiltinTIs(pkg)
	initUnsafeFuncs(pkg)
}

// ----------------------------------------------------------------------------

type typeTParam struct {
	name     string
	contract Contract
}

type typeParam struct {
	name string
	tidx int
}

// initBuiltinOps initializes operators of the builtin package.
func initBuiltinOps(builtin *types.Package, conf *Config) {
	ops := [...]struct {
		name    string
		tparams []typeTParam
		params  []typeParam
		result  int
	}{
		{"Add", []typeTParam{{"T", addable}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Add[T addable](a, b T) T

		{"Sub", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Sub[T number](a, b T) T

		{"Mul", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Mul[T number](a, b T) T

		{"Quo", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Quo(a, b untyped_bigint) untyped_bigrat
		// func Gop_Quo[T number](a, b T) T

		{"Rem", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Rem[T integer](a, b T) T

		{"Or", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Or[T integer](a, b T) T

		{"Xor", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_Xor[T integer](a, b T) T

		{"And", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_And[T integer](a, b T) T

		{"AndNot", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_AndNot[T integer](a, b T) T

		{"Lsh", []typeTParam{{"T", integer}, {"N", ninteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
		// func Gop_Lsh[T integer, N ninteger](a T, n N) T

		{"Rsh", []typeTParam{{"T", integer}, {"N", ninteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
		// func Gop_Rsh[T integer, N ninteger](a T, n N) T

		{"LT", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_LT[T orderable](a, b T) untyped_bool

		{"LE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_LE[T orderable](a, b T) untyped_bool

		{"GT", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_GT[T orderable](a, b T) untyped_bool

		{"GE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_GE[T orderable](a, b T) untyped_bool

		{"EQ", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_EQ[T comparable](a, b T) untyped_bool

		{"NE", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_NE[T comparable](a, b T) untyped_bool

		{"LAnd", []typeTParam{{"T", cbool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_LAnd[T bool](a, b T) T

		{"LOr", []typeTParam{{"T", cbool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_LOr[T bool](a, b T) T

		{"Neg", []typeTParam{{"T", number}}, []typeParam{{"a", 0}}, 0},
		// func Gop_Neg[T number](a T) T

		{"Dup", []typeTParam{{"T", number}}, []typeParam{{"a", 0}}, 0},
		// func Gop_Dup[T number](a T) T

		{"Not", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}}, 0},
		// func Gop_Not[T integer](a T) T

		{"LNot", []typeTParam{{"T", cbool}}, []typeParam{{"a", 0}}, 0},
		// func Gop_LNot[T bool](a T) T
	}
	gbl := builtin.Scope()
	for _, op := range ops {
		tparams := newTParams(op.tparams)
		n := len(op.params)
		params := make([]*types.Var, n)
		for i, param := range op.params {
			params[i] = types.NewParam(token.NoPos, builtin, param.name, tparams[param.tidx])
		}
		var results *types.Tuple
		if op.result != -2 {
			var ret types.Type
			if op.result < 0 {
				ret = types.Typ[types.UntypedBool]
			} else {
				ret = tparams[op.result]
			}
			result := types.NewParam(token.NoPos, builtin, "", ret)
			results = types.NewTuple(result)
		}
		tokFlag := nameToOps[op.name].Tok
		if n == 1 {
			tokFlag |= tokUnaryFlag
		}
		name := goxPrefix + op.name
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, false, tokFlag)
		var tfn types.Object = NewTemplateFunc(token.NoPos, builtin, name, tsig)
		if op.name == "Quo" { // func Gop_Quo(a, b untyped_bigint) untyped_bigrat
			a := types.NewParam(token.NoPos, builtin, "a", conf.UntypedBigInt)
			b := types.NewParam(token.NoPos, builtin, "b", conf.UntypedBigInt)
			ret := types.NewParam(token.NoPos, builtin, "", conf.UntypedBigRat)
			sig := NewTemplateSignature(nil, nil, types.NewTuple(a, b), types.NewTuple(ret), false, tokFlag)
			quo := NewTemplateFunc(token.NoPos, builtin, name, sig)
			tfn = NewOverloadFunc(token.NoPos, builtin, name, tfn, quo)
		}
		gbl.Insert(tfn)
	}

	// Inc++, Dec--, Recv<-, Addr& are special cases
	gbl.Insert(NewInstruction(token.NoPos, builtin, goxPrefix+"Inc", incInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, goxPrefix+"Dec", decInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, goxPrefix+"Recv", recvInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, goxPrefix+"Addr", addrInstr{}))
}

func newTParams(params []typeTParam) []*TemplateParamType {
	n := len(params)
	tparams := make([]*TemplateParamType, n)
	for i, tparam := range params {
		tparams[i] = NewTemplateParamType(i, tparam.name, tparam.contract)
	}
	return tparams
}

// initBuiltinAssignOps initializes assign operators of the builtin package.
func initBuiltinAssignOps(builtin *types.Package) {
	ops := [...]struct {
		name     string
		t        Contract
		ninteger bool
	}{
		{"AddAssign", addable, false},
		// func Gop_AddAssign[T addable](a *T, b T)

		{"SubAssign", number, false},
		// func Gop_SubAssign[T number](a *T, b T)

		{"MulAssign", number, false},
		// func Gop_MulAssign[T number](a *T, b T)

		{"QuoAssign", number, false},
		// func Gop_QuoAssign[T number](a *T, b T)

		{"RemAssign", integer, false},
		// func Gop_RemAssign[T integer](a *T, b T)

		{"OrAssign", integer, false},
		// func Gop_OrAssign[T integer](a *T, b T)

		{"XorAssign", integer, false},
		// func Gop_XorAssign[T integer](a *T, b T)

		{"AndAssign", integer, false},
		// func Gop_Assign[T integer](a *T, b T)

		{"AndNotAssign", integer, false},
		// func Gop_AndNotAssign[T integer](a *T, b T)

		{"LshAssign", integer, true},
		// func Gop_LshAssign[T integer, N ninteger](a *T, n N)

		{"RshAssign", integer, true},
		// func Gop_RshAssign[T integer, N ninteger](a *T, n N)
	}
	gbl := builtin.Scope()
	for _, op := range ops {
		tparams := newOpTParams(op.t, op.ninteger)
		params := make([]*types.Var, 2)
		params[0] = types.NewParam(token.NoPos, builtin, "a", NewPointer(tparams[0]))
		if op.ninteger {
			params[1] = types.NewParam(token.NoPos, builtin, "n", tparams[1])
		} else {
			params[1] = types.NewParam(token.NoPos, builtin, "b", tparams[0])
		}
		name := goxPrefix + op.name
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), nil, false, 0)
		tfn := NewTemplateFunc(token.NoPos, builtin, name, tsig)
		gbl.Insert(tfn)
	}
}

func newOpTParams(t Contract, utinteger bool) []*TemplateParamType {
	tparams := make([]*TemplateParamType, 1, 2)
	tparams[0] = NewTemplateParamType(0, "T", t)
	if utinteger {
		tparams = append(tparams, NewTemplateParamType(1, "N", ninteger))
	}
	return tparams
}

// ----------------------------------------------------------------------------

type typeBParam struct {
	name string
	typ  types.BasicKind
}

type typeBFunc struct {
	params []typeBParam
	result types.BasicKind
}

type xType = interface{}
type typeXParam struct {
	name string
	typ  xType // tidx | types.Type
}

const (
	xtNone = iota << 16
	xtEllipsis
	xtSlice
	xtMap
	xtChanIn
)

// initBuiltinFuncs initializes builtin functions of the builtin package.
func initBuiltinFuncs(builtin *types.Package) {
	fns := [...]struct {
		name    string
		tparams []typeTParam
		params  []typeXParam
		result  xType
	}{
		{"copy", []typeTParam{{"Type", any}}, []typeXParam{{"dst", xtSlice}, {"src", xtSlice}}, types.Typ[types.Int]},
		// func [Type any] copy(dst, src []Type) int

		{"close", []typeTParam{{"Type", any}}, []typeXParam{{"c", xtChanIn}}, nil},
		// func [Type any] close(c chan<- Type)

		{"append", []typeTParam{{"Type", any}}, []typeXParam{{"slice", xtSlice}, {"elems", xtEllipsis}}, xtSlice},
		// func [Type any] append(slice []Type, elems ...Type) []Type

		{"delete", []typeTParam{{"Key", comparable}, {"Elem", any}}, []typeXParam{{"m", xtMap}, {"key", 0}}, nil},
		// func [Key comparable, Elem any] delete(m map[Key]Elem, key Key)
	}
	gbl := builtin.Scope()
	for _, fn := range fns {
		tparams := newTParams(fn.tparams)
		n := len(fn.params)
		params := make([]*types.Var, n)
		for i, param := range fn.params {
			typ := newXParamType(tparams, param.typ)
			params[i] = types.NewParam(token.NoPos, builtin, param.name, typ)
		}
		var ellipsis bool
		if tidx, ok := fn.params[n-1].typ.(int); ok && (tidx&xtEllipsis) != 0 {
			ellipsis = true
		}
		var results *types.Tuple
		if fn.result != nil {
			typ := newXParamType(tparams, fn.result)
			results = types.NewTuple(types.NewParam(token.NoPos, builtin, "", typ))
		}
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, ellipsis, tokFlagApproxType)
		var tfn types.Object = NewTemplateFunc(token.NoPos, builtin, fn.name, tsig)
		if fn.name == "append" { // append is a special case
			appendString := NewInstruction(token.NoPos, builtin, "append", appendStringInstr{})
			tfn = NewOverloadFunc(token.NoPos, builtin, "append", appendString, tfn)
		} else if fn.name == "copy" {
			// func [S string] copy(dst []byte, src S) int
			tparams := newTParams([]typeTParam{{"S", tstring}})
			dst := types.NewParam(token.NoPos, builtin, "dst", types.NewSlice(types.Typ[types.Byte]))
			src := types.NewParam(token.NoPos, builtin, "src", tparams[0])
			ret := types.NewParam(token.NoPos, builtin, "", types.Typ[types.Int])
			tsig := NewTemplateSignature(tparams, nil, types.NewTuple(dst, src), types.NewTuple(ret), false)
			copyString := NewTemplateFunc(token.NoPos, builtin, "copy", tsig)
			tfn = NewOverloadFunc(token.NoPos, builtin, "copy", copyString, tfn)
		}
		gbl.Insert(tfn)
	}
	overloads := [...]struct {
		name string
		fns  [3]typeBFunc
	}{
		{"complex", [...]typeBFunc{
			{[]typeBParam{{"r", types.UntypedFloat}, {"i", types.UntypedFloat}}, types.UntypedComplex},
			{[]typeBParam{{"r", types.Float32}, {"i", types.Float32}}, types.Complex64},
			{[]typeBParam{{"r", types.Float64}, {"i", types.Float64}}, types.Complex128},
		}},
		// func complex(r, i untyped_float) untyped_complex
		// func complex(r, i float32) complex64
		// func complex(r, i float64) complex128

		{"real", [...]typeBFunc{
			{[]typeBParam{{"c", types.UntypedComplex}}, types.UntypedFloat},
			{[]typeBParam{{"c", types.Complex64}}, types.Float32},
			{[]typeBParam{{"c", types.Complex128}}, types.Float64},
		}},
		// func real(c untyped_complex) untyped_float
		// func real(c complex64) float32
		// func real(c complex128) float64

		{"imag", [...]typeBFunc{
			{[]typeBParam{{"c", types.UntypedComplex}}, types.UntypedFloat},
			{[]typeBParam{{"c", types.Complex64}}, types.Float32},
			{[]typeBParam{{"c", types.Complex128}}, types.Float64},
		}},
		// func imag(c untyped_complex) untyped_float
		// func imag(c complex64) float32
		// func imag(c complex128) float64
	}
	for _, overload := range overloads {
		fns := []types.Object{
			newBFunc(builtin, overload.name, overload.fns[0]),
			newBFunc(builtin, overload.name, overload.fns[1]),
			newBFunc(builtin, overload.name, overload.fns[2]),
		}
		gbl.Insert(NewOverloadFunc(token.NoPos, builtin, overload.name, fns...))
	}
	// func panic(v interface{})
	// func recover() interface{}
	// func print(args ...interface{})
	// func println(args ...interface{})
	emptyIntfVar := types.NewVar(token.NoPos, builtin, "v", TyEmptyInterface)
	emptyIntfTuple := types.NewTuple(emptyIntfVar)
	emptyIntfSlice := types.NewSlice(TyEmptyInterface)
	emptyIntfSliceVar := types.NewVar(token.NoPos, builtin, "args", emptyIntfSlice)
	emptyIntfSliceTuple := types.NewTuple(emptyIntfSliceVar)
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "panic", types.NewSignatureType(nil, nil, nil, emptyIntfTuple, nil, false)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "recover", types.NewSignatureType(nil, nil, nil, nil, emptyIntfTuple, false)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "print", types.NewSignatureType(nil, nil, nil, emptyIntfSliceTuple, nil, true)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "println", types.NewSignatureType(nil, nil, nil, emptyIntfSliceTuple, nil, true)))

	// new & make are special cases, they require to pass a type.
	gbl.Insert(NewInstruction(token.NoPos, builtin, "new", newInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, "make", makeInstr{}))

	// len & cap are special cases, because they may return a constant value.
	gbl.Insert(NewInstruction(token.NoPos, builtin, "len", lenInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, "cap", capInstr{}))
}

func initUnsafeFuncs(pkg *Package) {
	unsafe := types.NewPackage("unsafe", "unsafe")
	gbl := unsafe.Scope()
	// unsafe
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Sizeof", unsafeSizeofInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Alignof", unsafeAlignofInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Offsetof", unsafeOffsetofInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Add", unsafeAddInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Slice", unsafeSliceInstr{}))
	pkg.unsafe_.Types = unsafe
}

func newBFunc(builtin *types.Package, name string, t typeBFunc) types.Object {
	n := len(t.params)
	vars := make([]*types.Var, n)
	for i, param := range t.params {
		vars[i] = types.NewParam(token.NoPos, builtin, param.name, types.Typ[param.typ])
	}
	result := types.NewParam(token.NoPos, builtin, "", types.Typ[t.result])
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(vars...), types.NewTuple(result), false)
	return types.NewFunc(token.NoPos, builtin, name, sig)
}

func newXParamType(tparams []*TemplateParamType, x xType) types.Type {
	if tidx, ok := x.(int); ok {
		idx := tidx & 0xffff
		switch tidx &^ 0xffff {
		case xtNone:
			return tparams[idx]
		case xtEllipsis, xtSlice:
			return NewSlice(tparams[idx])
		case xtMap:
			return NewMap(tparams[idx], tparams[idx+1])
		case xtChanIn:
			return NewChan(types.SendOnly, tparams[idx])
		default:
			panic("TODO: newXParamType - unexpected xType")
		}
	}
	return x.(types.Type)
}

// ----------------------------------------------------------------------------

type builtinFn struct {
	fn   interface{}
	narg int
}

var (
	builtinFns = map[string]builtinFn{
		"complex": {makeComplex, 2},
		"real":    {constant.Real, 1},
		"imag":    {constant.Imag, 1},
	}
)

func builtinCall(fn *Element, args []*Element) constant.Value {
	if fn, ok := fn.Val.(*ast.Ident); ok {
		if bfn, ok := builtinFns[fn.Name]; ok {
			a := args[0].CVal
			switch bfn.narg {
			case 1:
				return bfn.fn.(func(a constant.Value) constant.Value)(a)
			case 2:
				b := args[1].CVal
				return bfn.fn.(func(a, b constant.Value) constant.Value)(a, b)
			}
		}
		panic("builtinCall: expect constant")
	}
	return nil
}

func makeComplex(re, im constant.Value) constant.Value {
	return constant.BinaryOp(re, token.ADD, constant.MakeImag(im))
}

type appendStringInstr struct {
}

// func append(slice []byte, val ..string) []byte
func (p appendStringInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) == 2 && flags != 0 {
		if t, ok := args[0].Type.(*types.Slice); ok {
			if elem, ok := t.Elem().(*types.Basic); ok && elem.Kind() == types.Byte {
				if v, ok := args[1].Type.(*types.Basic); ok {
					if v.Kind() == types.String || v.Kind() == types.UntypedString {
						return &Element{
							Val: &ast.CallExpr{
								Fun:      identAppend,
								Args:     []ast.Expr{args[0].Val, args[1].Val},
								Ellipsis: 1,
							},
							Type: t,
						}, nil
					}
				}
			}
		}
	}
	return nil, syscall.EINVAL
}

type lenInstr struct {
}

type capInstr struct {
}

// func [Type lenable] len(v Type) int
func (p lenInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: len() should have one parameter")
	}
	var cval constant.Value
	switch t := args[0].Type.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.String, types.UntypedString:
			if v := args[0].CVal; v != nil {
				n := len(constant.StringVal(v))
				cval = constant.MakeInt64(int64(n))
			}
		default:
			panic("TODO: call len() to a basic type")
		}
	case *types.Array:
		cval = constant.MakeInt64(t.Len())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			cval = constant.MakeInt64(tt.Len())
		} else {
			panic("TODO: call len() to a pointer")
		}
	default:
		if !lenable.Match(pkg, t) {
			log.Panicln("TODO: can't call len() to", t)
		}
	}
	ret = &Element{
		Val:  &ast.CallExpr{Fun: identLen, Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Int],
		CVal: cval,
	}
	return
}

// func [Type capable] cap(v Type) int
func (p capInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: cap() should have one parameter")
	}
	var cval constant.Value
	switch t := args[0].Type.(type) {
	case *types.Array:
		cval = constant.MakeInt64(t.Len())
	case *types.Pointer:
		if tt, ok := t.Elem().(*types.Array); ok {
			cval = constant.MakeInt64(tt.Len())
		} else {
			panic("TODO: call cap() to a pointer")
		}
	default:
		if !capable.Match(pkg, t) {
			log.Panicln("TODO: can't call cap() to", t)
		}
	}
	ret = &Element{
		Val:  &ast.CallExpr{Fun: identCap, Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Int],
		CVal: cval,
	}
	return
}

type incInstr struct {
}

type decInstr struct {
}

// val++
func (p incInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	return callIncDec(pkg, args, token.INC)
}

// val--
func (p decInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	return callIncDec(pkg, args, token.DEC)
}

func callIncDec(pkg *Package, args []*Element, tok token.Token) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use val" + tok.String())
	}
	t, ok := args[0].Type.(*refType)
	if !ok {
		panic("TODO: not addressable")
	}
	cb := &pkg.cb
	if !isNumeric(cb, t.typ) {
		text, pos := cb.loadExpr(args[0].Src)
		cb.panicCodeErrorf(pos, "invalid operation: %s%v (non-numeric type %v)", text, tok, t.typ)
	}
	cb.emitStmt(&ast.IncDecStmt{X: args[0].Val, Tok: tok})
	return
}

func isNumeric(cb *CodeBuilder, typ types.Type) bool {
	const (
		numericFlags = types.IsInteger | types.IsFloat | types.IsComplex
	)
	if t, ok := typ.(*types.Named); ok {
		typ = cb.getUnderlying(t)
	}
	if t, ok := typ.(*types.Basic); ok {
		return (t.Info() & numericFlags) != 0
	}
	return false
}

type recvInstr struct {
}

// <-ch
func (p recvInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use <-ch")
	}
	t0 := args[0].Type
retry:
	switch t := t0.(type) {
	case *types.Chan:
		if t.Dir() != types.SendOnly {
			typ := t.Elem()
			if flags != 0 { // twoValue mode
				typ = types.NewTuple(
					pkg.NewParam(token.NoPos, "", typ),
					pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
			}
			ret = &Element{Val: &ast.UnaryExpr{Op: token.ARROW, X: args[0].Val}, Type: typ}
			return
		}
		panic("TODO: <-ch is a send only chan")
	case *types.Named:
		t0 = pkg.cb.getUnderlying(t)
		goto retry
	}
	panic("TODO: <-ch not a chan type")
}

type addrInstr struct {
}

// &variable
func (p addrInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use &variable to get its address")
	}
	// TODO: can't take addr(&) to a non-reference type
	t, _ := DerefType(args[0].Type)
	ret = &Element{Val: &ast.UnaryExpr{Op: token.AND, X: args[0].Val}, Type: types.NewPointer(t)}
	return
}

type newInstr struct {
}

// func [] new(T any) *T
func (p newInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: use new(T) please")
	}
	ttyp, ok := args[0].Type.(*TypeType)
	if !ok {
		panic("TODO: new arg isn't a type")
	}
	typ := ttyp.Type()
	ret = &Element{
		Val: &ast.CallExpr{
			Fun:  identNew,
			Args: []ast.Expr{args[0].Val},
		},
		Type: types.NewPointer(typ),
	}
	return
}

type makeInstr struct {
}

// func [N ninteger] make(Type makable, size ...N) Type
func (p makeInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	n := len(args)
	if n == 0 {
		panic("TODO: make without args")
	} else if n > 3 {
		n, args = 3, args[:3]
	}
	ttyp, ok := args[0].Type.(*TypeType)
	if !ok {
		panic("TODO: make: first arg isn't a type")
	}
	typ := ttyp.Type()
	if !makable.Match(pkg, typ) {
		log.Panicln("TODO: can't make this type -", typ)
	}
	argsExpr := make([]ast.Expr, n)
	for i, arg := range args {
		argsExpr[i] = arg.Val
	}
	ret = &Element{
		Val: &ast.CallExpr{
			Fun:  identMake,
			Args: argsExpr,
		},
		Type: typ,
	}
	return
}

func checkArgsCount(pkg *Package, fn string, n int, args int, src ast.Node) {
	if args == n {
		return
	}
	cb := &pkg.cb
	text, pos := cb.loadExpr(src)
	if pos != token.NoPos {
		pos += token.Pos(len(fn))
	}
	if args < n {
		cb.panicCodeErrorf(pos, "missing argument to function call: %v", text)
	}
	cb.panicCodeErrorf(pos, "too many arguments to function call: %v", text)
}

var (
	std types.Sizes
)

func init() {
	if runtime.Compiler == "gopherjs" {
		std = &types.StdSizes{WordSize: 4, MaxAlign: 4}
	} else {
		std = types.SizesFor(runtime.Compiler, runtime.GOARCH)
	}
}

func unsafeRef(name string) Ref {
	return PkgRef{types.Unsafe}.Ref(name)
}

type unsafeSizeofInstr struct{}

// func unsafe.Sizeof(x ArbitraryType) uintptr
func (p unsafeSizeofInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Sizeof", 1, len(args), src)

	typ := types.Default(realType(args[0].Type))
	fn := toObjectExpr(pkg, unsafeRef("Sizeof"))
	ret = &Element{
		Val:  &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(std.Sizeof(typ)),
		Src:  src,
	}
	return
}

type unsafeAlignofInstr struct{}

// func unsafe.Alignof(x ArbitraryType) uintptr
func (p unsafeAlignofInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Alignof", 1, len(args), src)

	typ := types.Default(realType(args[0].Type))
	fn := toObjectExpr(pkg, unsafeRef("Alignof"))
	ret = &Element{
		Val:  &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(std.Alignof(typ)),
		Src:  src,
	}
	return
}

type unsafeOffsetofInstr struct{}

// func unsafe.Offsetof(x ArbitraryType) uintptr
func (p unsafeOffsetofInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Offsetof", 1, len(args), src)

	var sel *ast.SelectorExpr
	var ok bool
	if sel, ok = args[0].Val.(*ast.SelectorExpr); !ok {
		s, pos := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, "invalid expression %v", s)
	}
	if _, ok = args[0].Type.(*types.Signature); ok {
		s, pos := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, "invalid expression %v: argument is a method value", s)
	}
	recv := denoteRecv(sel)
	typ := getStruct(pkg, recv.Type)
	_, index, _ := types.LookupFieldOrMethod(typ, false, pkg.Types, sel.Sel.Name)
	offset, err := offsetof(pkg, typ, index, recv.Src, sel.Sel.Name)
	if err != nil {
		_, pos := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, "%v", err)
	}
	//var offset int64
	fn := toObjectExpr(pkg, unsafeRef("Offsetof"))
	ret = &Element{
		Val:  &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(offset),
		Src:  src,
	}
	return
}

func getStruct(pkg *Package, typ types.Type) *types.Struct {
retry:
	switch t := typ.(type) {
	case *types.Struct:
		return t
	case *types.Pointer:
		typ = t.Elem()
		goto retry
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	}
	return nil
}

func offsetsof(T *types.Struct) []int64 {
	var fields []*types.Var
	for i := 0; i < T.NumFields(); i++ {
		fields = append(fields, T.Field(i))
	}
	return std.Offsetsof(fields)
}

// offsetof returns the offset of the field specified via
// the index sequence relative to typ. All embedded fields
// must be structs (rather than pointer to structs).
func offsetof(pkg *Package, typ types.Type, index []int, recv ast.Node, sel string) (int64, error) {
	var o int64
	var typList []string
	var indirectType int
	for n, i := range index {
		if n > 0 {
			if t, ok := typ.(*types.Pointer); ok {
				typ = t.Elem()
				indirectType = n
			}
			if t, ok := typ.(*types.Named); ok {
				typList = append(typList, t.Obj().Name())
				typ = t.Underlying()
			}
		}
		s := typ.(*types.Struct)
		o += offsetsof(s)[i]
		typ = s.Field(i).Type()
	}
	if indirectType > 0 {
		s, _ := pkg.cb.loadExpr(recv)
		return -1, fmt.Errorf("invalid expression unsafe.Offsetof(%v.%v.%v): selector implies indirection of embedded %v.%v",
			s, strings.Join(typList, "."), sel,
			s, strings.Join(typList[:indirectType], "."))
	}
	return o, nil
}

type unsafeAddInstr struct{}

// func unsafe.Add(ptr Pointer, len IntegerType) Pointer
func (p unsafeAddInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Add", 2, len(args), src)

	if ts := args[0].Type.String(); ts != "unsafe.Pointer" {
		s, _ := pkg.cb.loadExpr(args[0].Src)
		pos := getSrcPos(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Add"))
		}
		pkg.cb.panicCodeErrorf(pos, "cannot use %v (type %v) as type unsafe.Pointer in argument to unsafe.Add", s, ts)
	}
	if t := args[1].Type; !ninteger.Match(pkg, t) {
		s, _ := pkg.cb.loadExpr(args[1].Src)
		pos := getSrcPos(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Add"))
		}
		pkg.cb.panicCodeErrorf(pos, "cannot use %v (type %v) as type int", s, t)
	}
	fn := toObjectExpr(pkg, unsafeRef("Sizeof")).(*ast.SelectorExpr)
	fn.Sel.Name = "Add" // only in go v1.7+
	ret = &Element{
		Val:  &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val, args[1].Val}},
		Type: types.Typ[types.UnsafePointer],
	}
	return
}

type unsafeSliceInstr struct{}

// func unsafe.Slice(ptr *ArbitraryType, len IntegerType) []ArbitraryType
func (p unsafeSliceInstr) Call(pkg *Package, args []*Element, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Slice", 2, len(args), src)

	t0, ok := args[0].Type.(*types.Pointer)
	if !ok {
		pos := getSrcPos(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Slice"))
		}
		pkg.cb.panicCodeErrorf(pos, "first argument to unsafe.Slice must be pointer; have %v", args[0].Type)
	}
	if t := args[1].Type; !ninteger.Match(pkg, t) {
		pos := getSrcPos(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Slice"))
		}
		pkg.cb.panicCodeErrorf(pos, "non-integer len argument in unsafe.Slice - %v", t)
	}
	fn := toObjectExpr(pkg, unsafeRef("Sizeof")).(*ast.SelectorExpr)
	fn.Sel.Name = "Slice" // only in go v1.7+
	ret = &Element{
		Val:  &ast.CallExpr{Fun: fn, Args: []ast.Expr{args[0].Val, args[1].Val}},
		Type: types.NewSlice(t0.Elem()),
	}
	return
}

// ----------------------------------------------------------------------------

type basicContract struct {
	kinds uint64
	desc  string
}

func (p *basicContract) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		if (uint64(1)<<t.Kind())&p.kinds != 0 {
			return true
		}
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	}
	return false
}

func (p *basicContract) String() string {
	return p.desc
}

const (
	// int, int64, int32, int16, int8, uint, uintptr, uint64, uint32, uint16, uint8
	kindsInteger = (1 << types.Int) | (1 << types.Int64) | (1 << types.Int32) | (1 << types.Int16) | (1 << types.Int8) |
		(1 << types.Uint) | (1 << types.Uintptr) | (1 << types.Uint64) | (1 << types.Uint32) | (1 << types.Uint16) | (1 << types.Uint8) |
		(1 << types.UntypedInt) | (1 << types.UntypedRune)

	// float32, float64
	kindsFloat = (1 << types.Float32) | (1 << types.Float64) | (1 << types.UntypedFloat)

	// complex64, complex128
	kindsComplex = (1 << types.Complex64) | (1 << types.Complex128) | (1 << types.UntypedComplex)

	// string
	kindsString = (1 << types.String) | (1 << types.UntypedString)

	// bool
	kindsBool = (1 << types.Bool) | (1 << types.UntypedBool)

	// integer, float, complex
	kindsNumber = kindsInteger | kindsFloat | kindsComplex

	// number, string
	kindsAddable = kindsNumber | kindsString

	// integer, float, string
	kindsOrderable = kindsInteger | kindsFloat | kindsString
)

// ----------------------------------------------------------------------------

type comparableT struct {
	// addable
	// type bool, interface, pointer, array, chan, struct
	// NOTE: slice/map/func is very special, can only be compared to nil
}

func (p comparableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		return t.Kind() != types.UntypedNil // excluding nil
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Slice: // slice/map/func is very special
		return false
	case *types.Map:
		return false
	case *types.Signature:
		return false
	case *TemplateSignature:
		return false
	case *TemplateParamType:
		panic("TODO: unexpected - compare to template param type?")
	case *types.Tuple:
		panic("TODO: unexpected - compare to tuple type?")
	case *unboundType:
		panic("TODO: unexpected - compare to unboundType?")
	case *unboundFuncParam:
		panic("TODO: unexpected - compare to unboundFuncParam?")
	}
	return true
}

func (p comparableT) String() string {
	return "comparable"
}

// ----------------------------------------------------------------------------

type anyT struct {
}

func (p anyT) Match(pkg *Package, typ types.Type) bool {
	return true
}

func (p anyT) String() string {
	return "any"
}

// ----------------------------------------------------------------------------

type capableT struct {
	// type slice, chan, array, array_pointer
}

func (p capableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Chan:
		return true
	case *types.Array:
		return true
	case *types.Pointer:
		_, ok := t.Elem().(*types.Array) // array_pointer
		return ok
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	}
	return false
}

func (p capableT) String() string {
	return "capable"
}

// ----------------------------------------------------------------------------

type lenableT struct {
	// capable
	// type map, string
}

func (p lenableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		k := t.Kind()
		return k == types.String || k == types.UntypedString
	case *types.Map:
		return true
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	}
	return capable.Match(pkg, typ)
}

func (p lenableT) String() string {
	return "lenable"
}

// ----------------------------------------------------------------------------

type makableT struct {
	// type slice, chan, map
}

func (p makableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Chan:
		return true
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	}
	return false
}

func (p makableT) String() string {
	return "makable"
}

// ----------------------------------------------------------------------------

type addableT struct {
	// type basicContract{kindsAddable}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p addableT) Match(pkg *Package, typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		default:
			// TODO: refactor
			cb := pkg.cb
			cb.stk.Push(elemNone)
			kind := cb.findMember(typ, "Gop_Add", "", MemberFlagVal, &Element{}, nil, nil)
			if kind != 0 {
				cb.stk.PopN(1)
				if kind == MemberMethod {
					return true
				}
			}
		}
	}
	c := &basicContract{kinds: kindsAddable}
	return c.Match(pkg, typ)
}

func (p addableT) String() string {
	return "addable"
}

// ----------------------------------------------------------------------------

type numberT struct {
	// type basicContract{kindsNumber}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p numberT) Match(pkg *Package, typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		}
	}
	c := &basicContract{kinds: kindsNumber}
	return c.Match(pkg, typ)
}

func (p numberT) String() string {
	return "number"
}

// ----------------------------------------------------------------------------

type orderableT struct {
	// type basicContract{kindsOrderable}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p orderableT) Match(pkg *Package, typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		}
	}
	c := &basicContract{kinds: kindsOrderable}
	return c.Match(pkg, typ)
}

func (p orderableT) String() string {
	return "orderable"
}

// ----------------------------------------------------------------------------

type integerT struct {
	// type basicContract{kindsNumber}, untyped_bigint
}

func (p integerT) Match(pkg *Package, typ types.Type) bool {
	c := &basicContract{kinds: kindsNumber}
	if c.Match(pkg, typ) {
		return true
	}
	return typ == pkg.utBigInt
}

func (p integerT) String() string {
	return "integer"
}

// ----------------------------------------------------------------------------

var (
	any        = anyT{}
	capable    = capableT{}
	lenable    = lenableT{}
	makable    = makableT{}
	cbool      = &basicContract{kindsBool, "bool"}
	ninteger   = &basicContract{kindsInteger, "ninteger"}
	tstring    = &basicContract{kindsString, "tstring"}
	orderable  = orderableT{}
	integer    = integerT{}
	number     = numberT{}
	addable    = addableT{}
	comparable = comparableT{}
)

// ----------------------------------------------------------------------------

type bmExargs = []interface{}

type BuiltinMethod struct {
	Name   string
	Fn     types.Object
	Exargs []interface{}
}

func (p *BuiltinMethod) Results() *types.Tuple {
	return p.Fn.Type().(*types.Signature).Results()
}

func (p *BuiltinMethod) Params() *types.Tuple {
	params := p.Fn.Type().(*types.Signature).Params()
	n := params.Len() - len(p.Exargs) - 1
	if n <= 0 {
		return nil
	}
	ret := make([]*types.Var, n)
	for i := 0; i < n; i++ {
		ret[i] = params.At(i + 1)
	}
	return types.NewTuple(ret...)
}

type mthdSignature interface {
	Results() *types.Tuple
	Params() *types.Tuple
}

type BuiltinTI struct {
	typ     types.Type
	methods []*BuiltinMethod
}

func (p *BuiltinTI) AddMethods(mthds ...*BuiltinMethod) {
	p.methods = append(p.methods, mthds...)
}

func (p *BuiltinTI) numMethods() int {
	return len(p.methods)
}

func (p *BuiltinTI) method(i int) *BuiltinMethod {
	return p.methods[i]
}

func (p *BuiltinTI) lookupByName(name string) mthdSignature {
	for i, n := 0, p.numMethods(); i < n; i++ {
		method := p.method(i)
		if method.Name == name {
			return method
		}
	}
	return nil
}

var (
	tyMap   types.Type = types.NewMap(types.Typ[types.Invalid], types.Typ[types.Invalid])
	tyChan  types.Type = types.NewChan(0, types.Typ[types.Invalid])
	tySlice types.Type = types.NewSlice(types.Typ[types.Invalid])
)

func initBuiltinTIs(pkg *Package) {
	var (
		float64TI, intTI, int64TI, uint64TI *BuiltinTI
		ioxTI, stringTI, stringSliceTI      *BuiltinTI
	)
	btiMap := new(typeutil.Map)
	strconv := pkg.TryImport("strconv")
	strings := pkg.TryImport("strings")
	btoLen := types.Universe.Lookup("len")
	btoCap := types.Universe.Lookup("cap")
	{
		ioxPkg := pkg.conf.PkgPathIox
		if debugImportIox && ioxPkg == "" {
			ioxPkg = "github.com/goplus/gogen/internal/iox"
		}
		if ioxPkg != "" {
			if os := pkg.TryImport("os"); os.isValid() {
				if iox := pkg.TryImport(ioxPkg); iox.isValid() {
					ioxTI = &BuiltinTI{
						typ: os.Ref("File").Type(),
						methods: []*BuiltinMethod{
							{"Gop_Enum", iox.Ref("EnumLines"), nil},
						},
					}
				}
			}
		}
	}
	if strconv.isValid() {
		float64TI = &BuiltinTI{
			typ: types.Typ[types.Float64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatFloat"), bmExargs{'g', -1, 64}},
			},
		}
		intTI = &BuiltinTI{
			typ: types.Typ[types.Int],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("Itoa"), nil},
			},
		}
		int64TI = &BuiltinTI{
			typ: types.Typ[types.Int64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatInt"), bmExargs{10}},
			},
		}
		uint64TI = &BuiltinTI{
			typ: types.Typ[types.Uint64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatUint"), bmExargs{10}},
			},
		}
	}
	if strings.isValid() && strconv.isValid() {
		stringTI = &BuiltinTI{
			typ: types.Typ[types.String],
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Count", strings.Ref("Count"), nil},
				{"Int", strconv.Ref("Atoi"), nil},
				{"Int64", strconv.Ref("ParseInt"), bmExargs{10, 64}},
				{"Uint64", strconv.Ref("ParseUint"), bmExargs{10, 64}},
				{"Float", strconv.Ref("ParseFloat"), bmExargs{64}},
				{"Index", strings.Ref("Index"), nil},
				{"IndexAny", strings.Ref("IndexAny"), nil},
				{"IndexByte", strings.Ref("IndexByte"), nil},
				{"IndexRune", strings.Ref("IndexRune"), nil},
				{"LastIndex", strings.Ref("LastIndex"), nil},
				{"LastIndexAny", strings.Ref("LastIndexAny"), nil},
				{"LastIndexByte", strings.Ref("LastIndexByte"), nil},
				{"Contains", strings.Ref("Contains"), nil},
				{"ContainsAny", strings.Ref("ContainsAny"), nil},
				{"ContainsRune", strings.Ref("ContainsRune"), nil},
				{"Compare", strings.Ref("Compare"), nil},
				{"EqualFold", strings.Ref("EqualFold"), nil},
				{"HasPrefix", strings.Ref("HasPrefix"), nil},
				{"HasSuffix", strings.Ref("HasSuffix"), nil},
				{"Quote", strconv.Ref("Quote"), nil},
				{"Unquote", strconv.Ref("Unquote"), nil},
				{"ToTitle", strings.Ref("ToTitle"), nil},
				{"ToUpper", strings.Ref("ToUpper"), nil},
				{"ToLower", strings.Ref("ToLower"), nil},
				{"Fields", strings.Ref("Fields"), nil},
				{"Repeat", strings.Ref("Repeat"), nil},
				{"Split", strings.Ref("Split"), nil},
				{"SplitAfter", strings.Ref("SplitAfter"), nil},
				{"SplitN", strings.Ref("SplitN"), nil},
				{"SplitAfterN", strings.Ref("SplitAfterN"), nil},
				{"Replace", strings.Ref("Replace"), nil},
				{"ReplaceAll", strings.Ref("ReplaceAll"), nil},
				{"Trim", strings.Ref("Trim"), nil},
				{"TrimSpace", strings.Ref("TrimSpace"), nil},
				{"TrimLeft", strings.Ref("TrimLeft"), nil},
				{"TrimRight", strings.Ref("TrimRight"), nil},
				{"TrimPrefix", strings.Ref("TrimPrefix"), nil},
				{"TrimSuffix", strings.Ref("TrimSuffix"), nil},
			},
		}
	}
	if strings.isValid() {
		stringSliceTI = &BuiltinTI{
			typ: types.NewSlice(types.Typ[types.String]),
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Cap", btoCap, nil},
				{"Join", strings.Ref("Join"), nil},
			},
		}
	}
	tis := []*BuiltinTI{
		ioxTI,
		float64TI,
		intTI,
		int64TI,
		uint64TI,
		stringTI,
		stringSliceTI,
		{
			typ: tySlice,
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Cap", btoCap, nil},
			},
		},
		{
			typ: tyMap,
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
			},
		},
		{
			typ: tyChan,
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
			},
		},
	}
	for _, ti := range tis {
		if ti != nil {
			btiMap.Set(ti.typ, ti)
		}
	}
	pkg.cb.btiMap = btiMap
}

func (p *CodeBuilder) getBuiltinTI(typ types.Type) *BuiltinTI {
	switch t := typ.(type) {
	case *types.Basic:
		typ = types.Default(typ)
	case *types.Slice:
		if t.Elem() != types.Typ[types.String] {
			typ = tySlice
		}
	case *types.Map:
		typ = tyMap
	case *types.Chan:
		typ = tyChan
	}
	if bti := p.btiMap.At(typ); bti != nil {
		return bti.(*BuiltinTI)
	}
	return nil
}

// ----------------------------------------------------------------------------

func (p *Package) BuiltinTI(typ types.Type) *BuiltinTI {
	return p.cb.getBuiltinTI(typ)
}

// ----------------------------------------------------------------------------
