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

func isBuiltinOp(v types.Object) bool {
	return v.Pos() == token.NoPos
}

type typeTParam struct {
	name     string
	contract Contract
}

type typeParam struct {
	name string
	tidx int
}

// InitBuiltin initializes the builtin package.
func InitBuiltin(builtin *types.Package, prefix *NamePrefix, contracts *BuiltinContracts) {
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
	}
	gbl := builtin.Scope()
	pre := prefix.Operator
	for _, op := range ops {
		n := len(op.tparams)
		tparams := make([]*TemplateParamType, n)
		for i, tparam := range op.tparams {
			tparams[i] = NewTemplateParamType(i, tparam.name, tparam.contract)
		}
		n = len(op.params)
		params := make([]*types.Var, n)
		for i, param := range op.params {
			params[i] = types.NewParam(token.NoPos, builtin, param.name, tparams[param.tidx])
		}
		var ret types.Type
		if op.result < 0 {
			ret = types.Typ[types.Bool]
		} else {
			ret = tparams[op.result]
		}
		result := types.NewParam(token.NoPos, builtin, "", ret)
		tsig := NewTemplateSignature(tparams, nil, types.NewTuple(params...), types.NewTuple(result), false)
		gbl.Insert(NewTemplateFunc(token.NoPos, builtin, pre+op.name, tsig))
	}
}

func newBuiltinDefault(prefix *NamePrefix, contracts *BuiltinContracts) *types.Package {
	builtin := types.NewPackage("", "")
	InitBuiltin(builtin, prefix, contracts)
	return builtin
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

type comparable struct {
}

func (p comparable) Match(typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		return t.Kind() != types.UntypedNil // excluding nil
	case *types.Named:
		typ = t.Underlying()
		goto retry
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

func (p comparable) String() string {
	return "comparable"
}

// ----------------------------------------------------------------------------

var (
	defaultContracts = &BuiltinContracts{
		NInteger:   &basicContract{kindsInteger, "ninteger"},
		Integer:    &basicContract{kindsInteger, "integer"},
		Float:      &basicContract{kindsFloat, "float"},
		Complex:    &basicContract{kindsComplex, "complex"},
		Number:     &basicContract{kindsNumber, "number"},
		Addable:    &basicContract{kindsAddable, "addable"},
		Orderable:  &basicContract{kindsOrderable, "orderable"},
		Comparable: comparable{},
	}
)

// ----------------------------------------------------------------------------
