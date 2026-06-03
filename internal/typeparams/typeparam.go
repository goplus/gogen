package typeparams

import (
	"fmt"
	"go/types"
	"slices"
)

// IsParameterized reports whether typ contains any of the type parameters of tparams.
// If typ is a generic function, isParameterized ignores the type parameter declarations;
// it only considers the signature proper (incoming and result parameters).
func IsParameterized(tparams []*types.TypeParam, typ types.Type) bool {
	w := tpWalker{
		tparams: tparams,
		seen:    make(map[types.Type]bool),
	}
	return w.isParameterized(typ)
}

type tpWalker struct {
	tparams []*types.TypeParam
	seen    map[types.Type]bool
}

func (w *tpWalker) isParameterized(typ types.Type) (res bool) {
	// detect cycles
	if x, ok := w.seen[typ]; ok {
		return x
	}
	w.seen[typ] = false
	defer func() {
		w.seen[typ] = res
	}()

	switch t := typ.(type) {
	case *types.Basic:
		// nothing to do

	case *types.Alias:
		return w.isParameterized(types.Unalias(t))

	case *types.Array:
		return w.isParameterized(t.Elem())

	case *types.Slice:
		return w.isParameterized(t.Elem())

	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if w.isParameterized(t.Field(i).Type()) {
				return true
			}
		}

	case *types.Pointer:
		return w.isParameterized(t.Elem())

	case *types.Tuple:
		// This case does not occur from within isParameterized
		// because tuples only appear in signatures where they
		// are handled explicitly. But isParameterized is also
		// called by Checker.callExpr with a function result tuple
		// if instantiation failed (go.dev/issue/59890).
		return w.tuple(t)

	case *types.Signature:
		// t.tparams may not be nil if we are looking at a signature
		// of a generic function type (or an interface method) that is
		// part of the type we're testing. We don't care about these type
		// parameters.
		// Similarly, the receiver of a method may declare (rather than
		// use) type parameters, we don't care about those either.
		// Thus, we only need to look at the input and result parameters.
		return w.tuple(t.Params()) || w.tuple(t.Results())

	case *types.Interface:
		for i := 0; i < t.NumMethods(); i++ {
			if w.isParameterized(t.Method(i).Type()) {
				return true
			}
		}
	case *types.Map:
		return w.isParameterized(t.Key()) || w.isParameterized(t.Elem())

	case *types.Chan:
		return w.isParameterized(t.Elem())

	case *types.Named:
		targs := t.TypeArgs()
		for i := 0; i < targs.Len(); i++ {
			if w.isParameterized(targs.At(i)) {
				return true
			}
		}

	case *types.TypeParam:
		return slices.Index(w.tparams, t) >= 0

	default:
		panic(fmt.Sprintf("unexpected %T", typ))
	}

	return false
}

func (w *tpWalker) tuple(t *types.Tuple) bool {
	for i := 0; i < t.Len(); i++ {
		if w.isParameterized(t.At(i).Type()) {
			return true
		}
	}
	return false
}
