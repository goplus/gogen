package gogen

import (
	"go/token"
	"go/types"

	"github.com/goplus/gogen/internal"
)

var (
	termUntypedBool    = types.NewTerm(false, types.Typ[types.UntypedBool])
	termUntypedRune    = types.NewTerm(false, types.Typ[types.UntypedRune])
	termUntypedInt     = types.NewTerm(false, types.Typ[types.UntypedInt])
	termUntypedFloat   = types.NewTerm(false, types.Typ[types.UntypedFloat])
	termUntypedComplex = types.NewTerm(false, types.Typ[types.UntypedComplex])
	termUntypedString  = types.NewTerm(false, types.Typ[types.UntypedString])
	tildeBool          = types.NewTerm(true, types.Typ[types.Bool])
	tildeInt           = types.NewTerm(true, types.Typ[types.Int])
	tildeInt8          = types.NewTerm(true, types.Typ[types.Int8])
	tildeInt16         = types.NewTerm(true, types.Typ[types.Int16])
	tildeInt32         = types.NewTerm(true, types.Typ[types.Int32])
	tildeInt64         = types.NewTerm(true, types.Typ[types.Int64])
	tildeUint          = types.NewTerm(true, types.Typ[types.Uint])
	tildeUint8         = types.NewTerm(true, types.Typ[types.Uint8])
	tildeUint16        = types.NewTerm(true, types.Typ[types.Uint16])
	tildeUint32        = types.NewTerm(true, types.Typ[types.Uint32])
	tildeUint64        = types.NewTerm(true, types.Typ[types.Uint64])
	tildeUintptr       = types.NewTerm(true, types.Typ[types.Uintptr])
	tildeFloat32       = types.NewTerm(true, types.Typ[types.Float32])
	tildeFloat64       = types.NewTerm(true, types.Typ[types.Float64])
	tildeComplex64     = types.NewTerm(true, types.Typ[types.Complex64])
	tildeComplex128    = types.NewTerm(true, types.Typ[types.Complex128])
	tildeString        = types.NewTerm(true, types.Typ[types.String])
)

func makeConstraint(conf *Config, name string) types.Type {
	switch name {
	case "any":
		return anyConstraint
	case "bool":
		return boolConstraint
	case "string":
		return stringConstraint
	case "ninteger":
		// kindsNumber = kindsInteger | kindsFloat | kindsComplex
		return nintegerConstraint
	case "integer":
		// kindsNumber = kindsInteger | kindsFloat | kindsComplex
		terms := []*types.Term{
			tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
			tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
			tildeUintptr,
			tildeFloat32, tildeFloat64,
			tildeComplex64, tildeComplex128,
			// termUntypedRune, termUntypedInt, termUntypedFloat, termUntypedComplex,
		}
		if conf.UntypedBigInt != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigInt))
		}
		return newConstraint(terms)
	case "number":
		// kindsNumber = kindsInteger | kindsFloat | kindsComplex
		terms := []*types.Term{
			tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
			tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
			tildeUintptr,
			tildeFloat32, tildeFloat64,
			tildeComplex64, tildeComplex128,
			termUntypedRune, termUntypedInt, termUntypedFloat, termUntypedComplex,
		}
		if conf.UntypedBigInt != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigInt))
		}
		if conf.UntypedBigFloat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigFloat))
		}
		if conf.UntypedBigRat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigRat))
		}
		return newConstraint(terms)
	case "norderable":
		// kindsOrderable = kindsInteger | kindsFloat | kindsString
		return nnorderableConstraint
	case "orderable":
		// kindsOrderable = kindsInteger | kindsFloat | kindsString
		terms := []*types.Term{
			tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
			tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
			tildeUintptr,
			tildeFloat32, tildeFloat64,
			tildeString,
			// termUntypedRune, termUntypedInt, termUntypedFloat, termUntypedString,
		}
		if conf.UntypedBigInt != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigInt))
		}
		if conf.UntypedBigFloat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigFloat))
		}
		if conf.UntypedBigRat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigRat))
		}
		return newConstraint(terms)
	case "addable":
		// kindsNumber = kindsInteger | kindsFloat | kindsComplex
		// kindsAddable = kindsNumber | kindsString
		terms := []*types.Term{
			tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
			tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
			tildeUintptr,
			tildeFloat32, tildeFloat64,
			tildeComplex64, tildeComplex128,
			tildeString,
			//termUntypedRune, termUntypedInt, termUntypedFloat, termUntypedComplex, termUntypedString,
		}
		if conf.UntypedBigInt != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigInt))
		}
		if conf.UntypedBigFloat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigFloat))
		}
		if conf.UntypedBigRat != nil {
			terms = append(terms, types.NewTerm(false, conf.UntypedBigRat))
		}
		return newConstraint(terms)
	case "comparable":
		return comparableConstraint
	case "clearable":
		return anyConstraint
	}
	panic("unsupported: " + name)
}

var (
	boolConstraint     = newConstraint([]*types.Term{tildeBool, termUntypedBool})
	stringConstraint   = newConstraint([]*types.Term{tildeString, termUntypedString})
	nintegerConstraint = newConstraint([]*types.Term{
		tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
		tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
		tildeUintptr,
		// termUntypedRune, termUntypedInt, termUntypedFloat,
	})
	nnorderableConstraint = newConstraint([]*types.Term{
		tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
		tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
		tildeUintptr,
		tildeFloat32, tildeFloat64,
		tildeString,
		// termUntypedRune, termUntypedInt, termUntypedFloat, termUntypedString,
	})
	comparableConstraint = types.Universe.Lookup("comparable").Type()
	anyConstraint        = types.Universe.Lookup("any").Type()
)

// func[T interface{Add[T interface{}](x T) T}](a T, b T) T
func makeXGoAddSignature(pkg *types.Package) *types.Signature {
	anyInterface := types.NewInterfaceType(nil, nil)

	adderTName := types.NewTypeName(token.NoPos, pkg, "T", nil)
	adderTParam := types.NewTypeParam(adderTName, anyInterface)

	addMethodSig := types.NewSignatureType(
		nil,
		nil,
		[]*types.TypeParam{adderTParam},
		types.NewTuple(
			types.NewParam(token.NoPos, pkg, "x", adderTParam),
		),
		types.NewTuple(
			types.NewParam(token.NoPos, pkg, "", adderTParam),
		),
		false,
	)

	addMethod := types.NewFunc(token.NoPos, pkg, "XGo_Add", addMethodSig)

	adderInterface := types.NewInterfaceType(
		[]*types.Func{addMethod},
		nil,
	)

	addFuncTName := types.NewTypeName(token.NoPos, pkg, "T", nil)
	addFuncTParam := types.NewTypeParam(addFuncTName, adderInterface)

	addFuncSig := types.NewSignatureType(
		nil,
		nil,
		[]*types.TypeParam{addFuncTParam},
		types.NewTuple(
			types.NewParam(token.NoPos, pkg, "a", addFuncTParam),
			types.NewParam(token.NoPos, pkg, "b", addFuncTParam),
		),
		types.NewTuple(
			types.NewParam(token.NoPos, pkg, "", addFuncTParam),
		),
		false,
	)
	return addFuncSig
}

func newConstraint(terms []*types.Term) *types.Interface {
	union := types.NewUnion(terms)
	iface := types.NewInterfaceType(nil, []types.Type{union})
	iface.Complete()
	return iface
}

func newConstraintEx(methods []*types.Func, terms []*types.Term) *types.Interface {
	union := types.NewUnion(terms)
	iface := types.NewInterfaceType(methods, []types.Type{union})
	iface.Complete()
	return iface
}

func newTypeParams(pkg *types.Package, conf *Config, params []typeTParam) []*types.TypeParam {
	n := len(params)
	tparams := make([]*types.TypeParam, n)
	for i, tparam := range params {
		tparams[i] = types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, tparam.name, nil),
			makeConstraint(conf, tparam.contract.String()))
	}
	return tparams
}

func newOpTypeParams(pkg *types.Package, conf *Config, t Contract, utinteger bool) []*types.TypeParam {
	tparams := make([]*types.TypeParam, 1, 2)
	tparams[0] = types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "T", nil),
		makeConstraint(conf, t.String()))
	if utinteger {
		tparams = append(tparams, types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "N", nil),
			makeConstraint(conf, "ninteger")))
	}
	return tparams
}

// NewTemplateSignatureEx creates type of a typeparams function.
func NewTemplateSignatureEx(
	tparams []*types.TypeParam, recv *types.Var, params, results *types.Tuple, variadic bool, tok ...token.Token) *TemplateSignature {
	var tokFlag token.Token
	if tok != nil {
		tokFlag = tok[0]
	}
	tsig := &TemplateSignature{
		sig:     types.NewSignatureType(recv, nil, tparams, params, results, variadic),
		tokFlag: tokFlag,
	}
	return tsig
}

func (p *TemplateSignature) instantiateEx(pkg *Package, fn *internal.Elem, args []*internal.Elem, flags InstrFlags) (*types.Signature, error) {
	nargs := make([]*internal.Elem, len(args))
	copy(nargs, args)
	for i := 0; i < len(nargs); i++ {
		if ref, ok := nargs[i].Type.(*refType); ok {
			nargs[i] = &internal.Elem{
				Val:  args[i].Val,
				Type: types.NewPointer(ref.typ),
				CVal: args[i].CVal,
				Src:  args[i].Src,
			}
		}
	}
	if p.isOp() {
		// fix binary bigint -> rat
		if args[0].Type == pkg.utBigRat && args[1].Type == pkg.utBigInt {
			nargs[1] = &internal.Elem{
				Val:  args[1].Val,
				Type: types.Typ[types.UntypedInt],
				CVal: args[1].CVal,
				Src:  args[1].Src,
			}
		} else if args[0].Type == pkg.utBigInt && args[1].Type == pkg.utBigRat {
			nargs[0] = &internal.Elem{
				Val:  args[0].Val,
				Type: types.Typ[types.UntypedInt],
				CVal: args[0].CVal,
				Src:  args[0].Src,
			}
		}
	}
	sig, err := InferFunc(pkg, fn, p.sig, nil, nargs, flags)
	if err != nil {
		return nil, err
	}
	return sig.(*types.Signature), nil
}
