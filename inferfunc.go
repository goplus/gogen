//go:build !go1.21
// +build !go1.21

package gogen

import (
	"go/ast"
	"go/types"

	"github.com/goplus/gogen/internal"
)

func inferFunc(pkg *Package, fn *internal.Elem, sig *types.Signature, targs []types.Type, oargs []*Element, flags InstrFlags) ([]types.Type, types.Type, error) {
	args, err := checkInferArgs(pkg, fn, sig, oargs, flags)
	if err != nil {
		return nil, nil, err
	}
	xlist := make([]*operand, len(args))
	tp := sig.TypeParams()
	n := tp.Len()
	tparams := make([]*types.TypeParam, n)
	for i := 0; i < n; i++ {
		tparams[i] = tp.At(i)
	}
	for i, arg := range args {
		xlist[i] = &operand{
			mode: value,
			expr: arg.Val,
			typ:  arg.Type,
			val:  arg.CVal,
		}
		tt := arg.Type
	retry:
		switch t := tt.(type) {
		case *types.Slice:
			tt = t.Elem()
			goto retry
		case *inferFuncType:
			xlist[i].typ = types.Typ[types.UntypedNil]
			xlist[i].expr = ast.NewIdent("nil")
		case *types.Signature:
			if tp := t.TypeParams(); tp != nil {
				xlist[i].typ = types.Typ[types.UntypedNil]
				xlist[i].expr = ast.NewIdent("nil")
			}
		}
	}
	targs, err = infer(pkg, fn.Val, tparams, targs, sig.Params(), xlist)
	if err != nil {
		return nil, nil, err
	}
	typ, err := types.Instantiate(pkg.cb.ctxt, sig, targs[:n], true)
	return targs, typ, err
}
