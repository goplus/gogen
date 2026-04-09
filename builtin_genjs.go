//go:build genjs

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
	"go/token"
	"go/types"
)

const (
	// untyped int, untyped rune
	kindsInteger = (1 << types.UntypedInt) | (1 << types.UntypedRune)

	// float32, float64
	kindsFloat = (1 << types.Float32) | (1 << types.Float64) | (1 << types.UntypedFloat)

	// complex64, complex128
	kindsComplex = (1 << types.UntypedComplex)

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

	// Inc++, Dec-- are special cases
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Inc", incInstr{}))
	gbl.Insert(NewInstruction(token.NoPos, builtin, xgoPrefix+"Dec", decInstr{}))
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
