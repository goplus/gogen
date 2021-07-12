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
	"go/token"
	"go/types"
)

var (
	defaultNamePrefix = &NamePrefix{
		BuiltinType: "Gopb_",
		TypeExtend:  "Gope_",
		Operator:    "Gopo_",
		TypeConv:    "Gopc_",
	}
)

func newBuiltinDefault(pkg PkgImporter, prefix *NamePrefix, contracts *BuiltinContracts) *types.Package {
	builtin := types.NewPackage("", "")
	InitBuiltinOps(builtin, prefix, contracts)
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
func InitBuiltinOps(builtin *types.Package, prefix *NamePrefix, contracts *BuiltinContracts) {
	ops := [...]struct {
		name    string
		tparams []typeTParam
		params  []typeParam
		result  int
	}{
		{"Add", []typeTParam{{"T", contracts.Addable}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Add[T addable](a, b T) T

		{"Sub", []typeTParam{{"T", contracts.Number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Sub[T number](a, b T) T

		{"Mul", []typeTParam{{"T", contracts.Number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Mul[T number](a, b T) T

		{"Quo", []typeTParam{{"T", contracts.Number}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Quo[T number](a, b T) T

		{"Rem", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Rem[T integer](a, b T) T

		{"Or", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Or[T integer](a, b T) T

		{"Xor", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_Xor[T integer](a, b T) T

		{"And", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_And[T integer](a, b T) T

		{"AndNot", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}, {"b", 0}}, 0},
		// func gopo_AndNot[T integer](a, b T) T

		{"Lsh", []typeTParam{{"T", contracts.Integer}, {"N", contracts.NInteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
		// func gopo_Lsh[T integer, N ninteger](a T, n N) T

		{"Rsh", []typeTParam{{"T", contracts.Integer}, {"N", contracts.NInteger}}, []typeParam{{"a", 0}, {"n", 1}}, 0},
		// func gopo_Rsh[T integer, N ninteger](a T, n N) T

		{"LT", []typeTParam{{"T", contracts.Orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_LT[T orderable](a, b T) bool

		{"LE", []typeTParam{{"T", contracts.Orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_LE[T orderable](a, b T) bool

		{"GT", []typeTParam{{"T", contracts.Orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_GT[T orderable](a, b T) bool

		{"GE", []typeTParam{{"T", contracts.Orderable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_GE[T orderable](a, b T) bool

		{"EQ", []typeTParam{{"T", contracts.Comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_EQ[T comparable](a, b T) bool

		{"NE", []typeTParam{{"T", contracts.Comparable}}, []typeParam{{"a", 0}, {"b", 0}}, -1},
		// func gopo_NE[T comparable](a, b T) bool

		{"Neg", []typeTParam{{"T", contracts.Number}}, []typeParam{{"a", 0}}, 0},
		// func gopo_Neg[T number](a T) T

		{"Not", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", 0}}, 0},
		// func gopo_Not[T integer](a T) T

		// {"Inc", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", xtPointer}}, -2},
		// func gopo_Inc[T integer](a *T)

		// {"Dec", []typeTParam{{"T", contracts.Integer}}, []typeParam{{"a", xtPointer}}, -2},
		// func gopo_Dec[T integer](a *T)
	}
	gbl := builtin.Scope()
	pre := prefix.Operator
	for _, op := range ops {
		tparams := newTParams(op.tparams)
		n := len(op.params)
		params := make([]*types.Var, n)
		for i, param := range op.params {
			var typ types.Type
			if param.tidx == xtPointer {
				typ = NewPointer(tparams[0])
			} else {
				typ = tparams[param.tidx]
			}
			params[i] = types.NewParam(token.NoPos, builtin, param.name, typ)
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
		var allowUntyped = (op.params[0].tidx != xtPointer)
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, false, allowUntyped)
		gbl.Insert(NewTemplateFunc(token.NoPos, builtin, pre+op.name, tsig))
	}
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
	xtPointer
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

		{"len", []typeTParam{{"Type", lenable}}, []typeXParam{{"v", 0}}, types.Typ[types.Int]},
		// func [Type lenable] len(v Type) int

		{"cap", []typeTParam{{"Type", capable}}, []typeXParam{{"v", 0}}, types.Typ[types.Int]},
		// func [Type capable] cap(v Type) int

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
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), results, ellipsis, false)
		gbl.Insert(NewTemplateFunc(token.NoPos, builtin, fn.name, tsig))
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

type basicContract struct {
	kinds uint64
	desc  string
}

func (p *basicContract) Match(typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		if (uint64(1)<<t.Kind())&p.kinds != 0 {
			return true
		}
	case *types.Named:
		typ = t.Underlying()
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

func (p comparableT) Match(typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		return t.Kind() != types.UntypedNil // excluding nil
	case *types.Named:
		typ = t.Underlying()
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

func (p anyT) Match(typ types.Type) bool {
	return true
}

func (p anyT) String() string {
	return "any"
}

// ----------------------------------------------------------------------------

type capableT struct {
	// type slice, chan, array, array_pointer
}

func (p capableT) Match(typ types.Type) bool {
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
		typ = t.Underlying()
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

func (p lenableT) Match(typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		k := t.Kind()
		return k == types.String || k == types.UntypedString
	case *types.Map:
		return true
	case *types.Named:
		typ = t.Underlying()
		goto retry
	}
	return capable.Match(typ)
}

func (p lenableT) String() string {
	return "lenable"
}

// ----------------------------------------------------------------------------

type makableT struct {
	// type slice, chan, map
}

func (p makableT) Match(typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Chan:
		return true
	case *types.Named:
		typ = t.Underlying()
		goto retry
	}
	return false
}

func (p makableT) String() string {
	return "makable"
}

// ----------------------------------------------------------------------------

var (
	any        = anyT{}
	capable    = capableT{}
	lenable    = lenableT{}
	makable    = makableT{}
	comparable = comparableT{}
	ninteger   = &basicContract{kindsInteger, "ninteger"}
)

var (
	defaultContracts = &BuiltinContracts{
		NInteger:   ninteger,
		Integer:    &basicContract{kindsInteger, "integer"},
		Float:      &basicContract{kindsFloat, "float"},
		Complex:    &basicContract{kindsComplex, "complex"},
		Number:     &basicContract{kindsNumber, "number"},
		Addable:    &basicContract{kindsAddable, "addable"},
		Orderable:  &basicContract{kindsOrderable, "orderable"},
		Comparable: comparable,
	}
)

// ----------------------------------------------------------------------------
