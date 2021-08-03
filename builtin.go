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
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"syscall"
)

var (
	defaultNamePrefix = "Go_"
)

func newBuiltinDefault(pkg PkgImporter, prefix string, conf *Config) *types.Package {
	builtin := types.NewPackage("", "")
	InitBuiltinOps(builtin, prefix, conf)
	InitBuiltinFuncs(builtin)
	return builtin
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

// InitBuiltinOps initializes operators of the builtin package.
func InitBuiltinOps(builtin *types.Package, pre string, conf *Config) {
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
		// func Gop_LT[T orderable](a, b T) bool

		{"LE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_LE[T orderable](a, b T) bool

		{"GT", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_GT[T orderable](a, b T) bool

		{"GE", []typeTParam{{"T", orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_GE[T orderable](a, b T) bool

		{"EQ", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_EQ[T comparable](a, b T) bool

		{"NE", []typeTParam{{"T", comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func Gop_NE[T comparable](a, b T) bool

		{"LAnd", []typeTParam{{"T", cbool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_LAnd[T bool](a, b T) T

		{"LOr", []typeTParam{{"T", cbool}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func Gop_LOr[T bool](a, b T) T

		{"Neg", []typeTParam{{"T", number}}, []typeParam{{"a", 0}}, 0},
		// func Gop_Neg[T number](a T) T

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
				ret = types.Typ[types.Bool]
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
		name := pre + op.name
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, false, tokFlag)
		var tfn types.Object = NewTemplateFunc(token.NoPos, builtin, name, tsig)
		if op.name == "Quo" { // func Gop_Quo(a, b untyped_bigint) untyped_bigrat
			a := types.NewParam(token.NoPos, builtin, "a", conf.UntypedBigInt)
			b := types.NewParam(token.NoPos, builtin, "b", conf.UntypedBigInt)
			ret := types.NewParam(token.NoPos, builtin, "", conf.UntypedBigRat)
			sig := NewTemplateSignature(nil, nil, types.NewTuple(a, b), types.NewTuple(ret), false, tokFlag)
			quo := NewTemplateFunc(token.NoPos, builtin, name, sig)
			tfn = NewOverloadFunc(token.NoPos, builtin, name, quo, tfn)
		}
		gbl.Insert(tfn)
	}

	// Inc++, Dec--, Recv<-, Addr& are special cases
	gbl.Insert(NewInstruction(token.NoPos, builtin, pre+"Inc", incInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, pre+"Dec", decInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, pre+"Recv", recvInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, pre+"Addr", addrInstr{}))
}

func newTParams(params []typeTParam) []*TemplateParamType {
	n := len(params)
	tparams := make([]*TemplateParamType, n)
	for i, tparam := range params {
		tparams[i] = NewTemplateParamType(i, tparam.name, tparam.contract)
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

// InitBuiltinFuncs initializes builtin functions of the builtin package.
func InitBuiltinFuncs(builtin *types.Package) {
	fns := [...]struct {
		name    string
		tparams []typeTParam
		params  []typeXParam
		result  xType
	}{
		{"copy", []typeTParam{{"Type", any}}, []typeXParam{{"dst", xtSlice}, {"src", xtSlice}}, types.Typ[types.Int]},
		// func [Type any] copy(dst, src []Type) int

		{"close", []typeTParam{{"Type", any}}, []typeXParam{{"c", xtChanIn}}, nil},
		//func [Type any] close(c chan<- Type)

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
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, ellipsis)
		var tfn types.Object = NewTemplateFunc(token.NoPos, builtin, fn.name, tsig)
		if fn.name == "append" { // append is a special case
			appendString := NewInstruction(token.NoPos, builtin, "append", appendStringInstr{})
			tfn = NewOverloadFunc(token.NoPos, builtin, "append", appendString, tfn)
		}
		gbl.Insert(tfn)
	}
	overloads := [...]struct {
		name string
		fns  [2]typeBFunc
	}{
		{"complex", [...]typeBFunc{
			{[]typeBParam{{"r", types.Float32}, {"i", types.Float32}}, types.Complex64},
			{[]typeBParam{{"r", types.Float64}, {"i", types.Float64}}, types.Complex128},
		}},
		// func complex(r, i float32) complex64
		// func complex(r, i float64) complex128

		{"real", [...]typeBFunc{
			{[]typeBParam{{"c", types.Complex64}}, types.Float32},
			{[]typeBParam{{"c", types.Complex128}}, types.Float64},
		}},
		// func real(c complex64) float32
		// func real(c complex128) float64

		{"imag", [...]typeBFunc{
			{[]typeBParam{{"c", types.Complex64}}, types.Float32},
			{[]typeBParam{{"c", types.Complex128}}, types.Float64},
		}},
		// func imag(c complex64) float32
		// func imag(c complex128) float64
	}
	for _, overload := range overloads {
		fns := []types.Object{
			newBFunc(builtin, overload.name, overload.fns[0]),
			newBFunc(builtin, overload.name, overload.fns[1]),
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
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "panic", types.NewSignature(nil, emptyIntfTuple, nil, false)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "recover", types.NewSignature(nil, nil, emptyIntfTuple, false)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "print", types.NewSignature(nil, emptyIntfSliceTuple, nil, true)))
	gbl.Insert(types.NewFunc(token.NoPos, builtin, "println", types.NewSignature(nil, emptyIntfSliceTuple, nil, true)))

	// new & make are special cases, they require to pass a type.
	gbl.Insert(NewInstruction(token.NoPos, builtin, "new", newInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, "make", makeInstr{}))

	// len & cap are special cases, because they may return a constant value.
	gbl.Insert(NewInstruction(token.NoPos, builtin, "len", lenInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, "cap", capInstr{}))
}

func newBFunc(builtin *types.Package, name string, t typeBFunc) types.Object {
	n := len(t.params)
	vars := make([]*types.Var, n)
	for i, param := range t.params {
		vars[i] = types.NewParam(token.NoPos, builtin, param.name, types.Typ[param.typ])
	}
	result := types.NewParam(token.NoPos, builtin, "", types.Typ[t.result])
	sig := types.NewSignature(nil, types.NewTuple(vars...), types.NewTuple(result), false)
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
			return NewChan(types.RecvOnly, tparams[idx])
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
func (p appendStringInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
	if len(args) == 2 && flags != 0 {
		if t, ok := args[0].Type.(*types.Slice); ok {
			if elem, ok := t.Elem().(*types.Basic); ok && elem.Kind() == types.Byte {
				if v, ok := args[1].Type.(*types.Basic); ok {
					if v.Kind() == types.String || v.Kind() == types.UntypedString {
						return &Element{
							Val: &ast.CallExpr{
								Fun:      ident("append"),
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
func (p lenInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
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
		Val:  &ast.CallExpr{Fun: ident("len"), Args: []ast.Expr{args[0].Val}},
		Type: types.Typ[types.Int],
		CVal: cval,
	}
	return
}

// func [Type capable] cap(v Type) int
func (p capInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
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
		Val:  &ast.CallExpr{Fun: ident("cap"), Args: []ast.Expr{args[0].Val}},
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
func (p incInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
	return callIncDec(pkg, args, token.INC)
}

// val--
func (p decInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
	return callIncDec(pkg, args, token.DEC)
}

func callIncDec(pkg *Package, args []*Element, tok token.Token) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use val" + tok.String())
	}
	if _, ok := args[0].Type.(*refType); !ok {
		panic("TODO: not addressable")
	}
	// TODO: type check
	pkg.cb.emitStmt(&ast.IncDecStmt{X: args[0].Val, Tok: tok})
	return
}

type recvInstr struct {
}

// <-ch
func (p recvInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use <-ch")
	}
	if t, ok := args[0].Type.(*types.Chan); ok {
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
	}
	panic("TODO: <-ch not a chan type")
}

type addrInstr struct {
}

// &variable
func (p addrInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use &variable to get its address")
	}
	// TODO: type check
	t := args[0].Type
	ret = &Element{Val: &ast.UnaryExpr{Op: token.AND, X: args[0].Val}, Type: types.NewPointer(t)}
	return
}

type newInstr struct {
}

// func [] new(T any) *T
func (p newInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
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
			Fun:  ident("new"),
			Args: []ast.Expr{args[0].Val},
		},
		Type: types.NewPointer(typ),
	}
	return
}

type makeInstr struct {
}

// func [N ninteger] make(Type makable, size ...N) Type
func (p makeInstr) Call(pkg *Package, args []*Element, flags InstrFlags) (ret *Element, err error) {
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
			Fun:  ident("make"),
			Args: argsExpr,
		},
		Type: typ,
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
		typ = pkg.getUnderlying(t)
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
		(1 << types.Uint) | (1 << types.Uintptr) | (1 << types.Uint32) | (1 << types.Uint16) | (1 << types.Uint8) |
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
		typ = pkg.getUnderlying(t)
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
		typ = pkg.getUnderlying(t)
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
		typ = pkg.getUnderlying(t)
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
		typ = pkg.getUnderlying(t)
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
		case pkg.cb.utBigInt, pkg.cb.utBigRat, pkg.cb.utBigFlt:
			return true
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
		case pkg.cb.utBigInt, pkg.cb.utBigRat, pkg.cb.utBigFlt:
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

type integerT struct {
	// type basicContract{kindsNumber}, untyped_bigint
}

func (p integerT) Match(pkg *Package, typ types.Type) bool {
	c := &basicContract{kinds: kindsNumber}
	if c.Match(pkg, typ) {
		return true
	}
	return typ == pkg.cb.utBigInt
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
	orderable  = &basicContract{kindsOrderable, "orderable"}
	integer    = integerT{}
	number     = numberT{}
	addable    = addableT{}
	comparable = comparableT{}
)

// ----------------------------------------------------------------------------
