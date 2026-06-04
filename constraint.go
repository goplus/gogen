package gogen

import (
	"go/types"
)

var (
	tildeBool       = types.NewTerm(true, types.Typ[types.Bool])
	tildeInt        = types.NewTerm(true, types.Typ[types.Int])
	tildeInt8       = types.NewTerm(true, types.Typ[types.Int8])
	tildeInt16      = types.NewTerm(true, types.Typ[types.Int16])
	tildeInt32      = types.NewTerm(true, types.Typ[types.Int32])
	tildeInt64      = types.NewTerm(true, types.Typ[types.Int64])
	tildeUint       = types.NewTerm(true, types.Typ[types.Uint])
	tildeUint8      = types.NewTerm(true, types.Typ[types.Uint8])
	tildeUint16     = types.NewTerm(true, types.Typ[types.Uint16])
	tildeUint32     = types.NewTerm(true, types.Typ[types.Uint32])
	tildeUint64     = types.NewTerm(true, types.Typ[types.Uint64])
	tildeUintptr    = types.NewTerm(true, types.Typ[types.Uintptr])
	tildeFloat32    = types.NewTerm(true, types.Typ[types.Float32])
	tildeFloat64    = types.NewTerm(true, types.Typ[types.Float64])
	tildeComplex64  = types.NewTerm(true, types.Typ[types.Complex64])
	tildeComplex128 = types.NewTerm(true, types.Typ[types.Complex128])
	tildeString     = types.NewTerm(true, types.Typ[types.String])
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
	}
	panic("unsupported: " + name)
}

var (
	boolConstraint     = newConstraint([]*types.Term{tildeBool})
	stringConstraint   = newConstraint([]*types.Term{tildeString})
	nintegerConstraint = newConstraint([]*types.Term{
		tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
		tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
		tildeUintptr,
	})
	nnorderableConstraint = newConstraint([]*types.Term{
		tildeInt, tildeInt8, tildeInt16, tildeInt32, tildeInt64,
		tildeUint, tildeUint8, tildeUint16, tildeUint32, tildeUint64,
		tildeUintptr,
		tildeFloat32, tildeFloat64,
		tildeString,
	})
	comparableConstraint = types.Universe.Lookup("comparable").Type()
	anyConstraint        = types.Universe.Lookup("any").Type()
)

func newConstraint(terms []*types.Term) *types.Interface {
	union := types.NewUnion(terms)
	iface := types.NewInterfaceType(nil, []types.Type{union})
	iface.Complete()
	return iface
}
