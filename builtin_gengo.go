//go:build !genjs

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
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"syscall"
)

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

var _builtinOps = [...]struct {
	name    string
	tparams []typeTParam
	params  []typeParam
	result  int
}{
	{"Add", []typeTParam{{"T", addable}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Add[T addable](a, b T) T

	{"Sub", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Sub[T number](a, b T) T

	{"Mul", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Mul[T number](a, b T) T

	{"Quo", []typeTParam{{"T", number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Quo(a, b untyped_bigint) untyped_bigrat
	// func XGo_Quo[T number](a, b T) T

	{"Rem", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Rem[T integer](a, b T) T

	{"Or", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Or[T integer](a, b T) T

	{"Xor", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_Xor[T integer](a, b T) T

	{"And", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_And[T integer](a, b T) T

	{"AndNot", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_AndNot[T integer](a, b T) T

	{"Lsh", []typeTParam{{"T", integer}, {"N", ninteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
	// func XGo_Lsh[T integer, N ninteger](a T, n N) T

	{"Rsh", []typeTParam{{"T", integer}, {"N", ninteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
	// func XGo_Rsh[T integer, N ninteger](a T, n N) T

	{"LT", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_LT[T orderable](a, b T) untyped_bool

	{"LE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_LE[T orderable](a, b T) untyped_bool

	{"GT", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_GT[T orderable](a, b T) untyped_bool

	{"GE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_GE[T orderable](a, b T) untyped_bool

	{"EQ", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_EQ[T comparable](a, b T) untyped_bool

	{"NE", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
	// func XGo_NE[T comparable](a, b T) untyped_bool

	{"LAnd", []typeTParam{{"T", _bool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_LAnd[T bool](a, b T) T

	{"LOr", []typeTParam{{"T", _bool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
	// func XGo_LOr[T bool](a, b T) T

	{"Neg", []typeTParam{{"T", number}}, []typeParam{{"a", 0}}, 0},
	// func XGo_Neg[T number](a T) T

	{"Dup", []typeTParam{{"T", number}}, []typeParam{{"a", 0}}, 0},
	// func XGo_Dup[T number](a T) T

	{"Not", []typeTParam{{"T", integer}}, []typeParam{{"a", 0}}, 0},
	// func XGo_Not[T integer](a T) T

	{"LNot", []typeTParam{{"T", _bool}}, []typeParam{{"a", 0}}, 0},
	// func XGo_LNot[T bool](a T) T
}

// initBuiltinOps initializes operators of the builtin package.
func initBuiltinOps(builtin *types.Package, conf *Config) {
	gbl := builtin.Scope()
	for _, op := range _builtinOps {
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
		name := xgoPrefix + op.name
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, false, tokFlag)
		var tfn types.Object = NewTemplateFunc(token.NoPos, builtin, name, tsig)
		if op.name == "Quo" { // func XGo_Quo(a, b untyped_bigint) untyped_bigrat
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
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Inc", incInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Dec", decInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Recv", recvInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Addr", addrInstr{}))
}

func newTParams(params []typeTParam) []*TemplateParamType {
	n := len(params)
	tparams := make([]*TemplateParamType, n)
	for i, tparam := range params {
		tparams[i] = NewTemplateParamType(i, tparam.name, tparam.contract)
	}
	return tparams
}

var _assignOps = [...]struct {
	name     string
	t        Contract
	ninteger bool
}{
	{"AddAssign", addable, false},
	// func XGo_AddAssign[T addable](a *T, b T)

	{"SubAssign", number, false},
	// func XGo_SubAssign[T number](a *T, b T)

	{"MulAssign", number, false},
	// func XGo_MulAssign[T number](a *T, b T)

	{"QuoAssign", number, false},
	// func XGo_QuoAssign[T number](a *T, b T)

	{"RemAssign", integer, false},
	// func XGo_RemAssign[T integer](a *T, b T)

	{"OrAssign", integer, false},
	// func XGo_OrAssign[T integer](a *T, b T)

	{"XorAssign", integer, false},
	// func XGo_XorAssign[T integer](a *T, b T)

	{"AndAssign", integer, false},
	// func XGo_AndAssign[T integer](a *T, b T)

	{"AndNotAssign", integer, false},
	// func XGo_AndNotAssign[T integer](a *T, b T)

	{"LshAssign", integer, true},
	// func XGo_LshAssign[T integer, N ninteger](a *T, n N)

	{"RshAssign", integer, true},
	// func XGo_RshAssign[T integer, N ninteger](a *T, n N)
}

// initBuiltinAssignOps initializes assign operators of the builtin package.
func initBuiltinAssignOps(builtin *types.Package) {
	gbl := builtin.Scope()
	for _, op := range _assignOps {
		tparams := newOpTParams(op.t, op.ninteger)
		params := make([]*types.Var, 2)
		params[0] = types.NewParam(token.NoPos, builtin, "a", NewPointer(tparams[0]))
		if op.ninteger {
			params[1] = types.NewParam(token.NoPos, builtin, "n", tparams[1])
		} else {
			params[1] = types.NewParam(token.NoPos, builtin, "b", tparams[0])
		}
		name := xgoPrefix + op.name
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

type xType = any
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

var _builtinFns = [...]struct {
	name    string
	tparams []typeTParam
	params  []typeXParam
	result  xType
}{
	{"copy", []typeTParam{{"Type", _any}}, []typeXParam{{"dst", xtSlice}, {"src", xtSlice}}, types.Typ[types.Int]},
	// func [Type any] copy(dst, src []Type) int

	{"close", []typeTParam{{"Type", _any}}, []typeXParam{{"c", xtChanIn}}, nil},
	// func [Type any] close(c chan<- Type)

	{"append", []typeTParam{{"Type", _any}}, []typeXParam{{"slice", xtSlice}, {"elems", xtEllipsis}}, xtSlice},
	// func [Type any] append(slice []Type, elems ...Type) []Type

	{"delete", []typeTParam{{"Key", comparable}, {"Elem", _any}}, []typeXParam{{"m", xtMap}, {"key", 0}}, nil},
	// func [Key comparable, Elem any] delete(m map[Key]Elem, key Key)

	{"clear", []typeTParam{{"Type", clearable}}, []typeXParam{{"t", 0}}, nil},
	// func clear[T clearable](t T)

	{"max", []typeTParam{{"Type", borderable}}, []typeXParam{{"x", 0}, {"elems", xtEllipsis}}, 0},
	//func max[T borderable](x T, y ...T) T

	{"min", []typeTParam{{"Type", borderable}}, []typeXParam{{"x", 0}, {"elems", xtEllipsis}}, 0},
	//func min[T borderable](x T, y ...T) T
}

var _builtinOverloads = [...]struct {
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

// initBuiltinFuncs initializes builtin functions of the builtin package.
func initBuiltinFuncs(builtin *types.Package) {
	gbl := builtin.Scope()
	for _, fn := range _builtinFns {
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
		switch fn.name {
		case "append": // append is a special case
			appendString := NewInstruction(token.NoPos, builtin, "append", appendStringInstr{})
			tfn = NewOverloadFunc(token.NoPos, builtin, "append", appendString, tfn)
		case "copy": // func [S string] copy(dst []byte, src S) int
			tparams := newTParams([]typeTParam{{"S", _string}})
			dst := types.NewParam(token.NoPos, builtin, "dst", types.NewSlice(types.Typ[types.Byte]))
			src := types.NewParam(token.NoPos, builtin, "src", tparams[0])
			ret := types.NewParam(token.NoPos, builtin, "", types.Typ[types.Int])
			tsig := NewTemplateSignature(tparams, nil, types.NewTuple(dst, src), types.NewTuple(ret), false)
			copyString := NewTemplateFunc(token.NoPos, builtin, "copy", tsig)
			tfn = NewOverloadFunc(token.NoPos, builtin, "copy", copyString, tfn)
		}
		gbl.Insert(tfn)
	}
	for _, overload := range _builtinOverloads {
		fns := []types.Object{
			newBFunc(builtin, overload.name, overload.fns[0]),
			newBFunc(builtin, overload.name, overload.fns[1]),
			newBFunc(builtin, overload.name, overload.fns[2]),
		}
		gbl.Insert(NewOverloadFunc(token.NoPos, builtin, overload.name, fns...))
	}
	// func panic(v any)
	// func recover() any
	// func print(args ...any)
	// func println(args ...any)
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
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "Slice", unsafeDataInstr{"Slice", 2}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "SliceData", unsafeDataInstr{"SliceData", 1}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "String", unsafeDataInstr{"String", 2}))
	gbl.Insert(NewInstruction(token.NoPos, types.Unsafe, "StringData", unsafeDataInstr{"StringData", 1}))
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

type appendStringInstr struct {
}

// func append(slice []byte, val ..string) []byte
func (p appendStringInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) == 2 && flags != 0 {
		if t, ok := args[0].Type.(*types.Slice); ok {
			if elem, ok := t.Elem().(*types.Basic); ok && elem.Kind() == types.Byte {
				if v, ok := args[1].Type.(*types.Basic); ok {
					if v.Kind() == types.String || v.Kind() == types.UntypedString {
						return &Element{
							Val:  newAppendStringExpr(args),
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
func (p lenInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
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
		Val:  newLenExpr(args),
		Type: types.Typ[types.Int],
		CVal: cval,
	}
	return
}

// func [Type capable] cap(v Type) int
func (p capInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
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
		Val:  newCapExpr(args),
		Type: types.Typ[types.Int],
		CVal: cval,
	}
	return
}

type recvInstr struct {
}

// <-ch
func (p recvInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use <-ch")
	}
	t0 := args[0].Type
retry:
	switch t := t0.(type) {
	case *types.Chan:
		if t.Dir() != types.SendOnly {
			typ := t.Elem()
			if lhs == 2 { // twoValue mode
				typ = types.NewTuple(
					pkg.NewParam(token.NoPos, "", typ),
					pkg.NewParam(token.NoPos, "", types.Typ[types.Bool]))
			}
			ret = &Element{Val: newRecvExpr(args), Type: typ}
			return
		}
		panic("TODO: <-ch is a send only chan")
	case *types.Named:
		t0 = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		t0 = types.Unalias(t)
		goto retry
	}
	panic("TODO: <-ch not a chan type")
}

type addrInstr struct {
}

// &variable
func (p addrInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use &variable to get its address")
	}
	// TODO: can't take addr(&) to a non-reference type
	t, _ := DerefType(args[0].Type)
	ret = &Element{Val: newAddrExpr(args), Type: types.NewPointer(t)}
	return
}

type newInstr struct {
}

// func [] new(T any) *T
func (p newInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: use new(T) please")
	}
	ttyp, ok := args[0].Type.(*TypeType)
	if !ok {
		panic("TODO: new arg isn't a type")
	}
	typ := ttyp.Type()
	ret = &Element{
		Val:  newNewExpr(args),
		Type: types.NewPointer(typ),
	}
	return
}

type makeInstr struct {
}

// func [N ninteger] make(Type makable, size ...N) Type
func (p makeInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	if n := len(args); n == 0 {
		panic("TODO: make without args")
	} else if n > 3 {
		args = args[:3]
	}
	ttyp, ok := args[0].Type.(*TypeType)
	if !ok {
		panic("TODO: make: first arg isn't a type")
	}
	typ := ttyp.Type()
	if !makable.Match(pkg, typ) {
		log.Panicln("TODO: can't make this type -", typ)
	}
	ret = &Element{
		Val:  newMakeExpr(args),
		Type: typ,
	}
	return
}

// ----------------------------------------------------------------------------
