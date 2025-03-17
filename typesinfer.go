//go:build !go1.23
// +build !go1.23

package gogen

import (
	"errors"
	"go/types"
	_ "unsafe"
)

const (
	invalid   operandMode = iota // operand is invalid
	novalue                      // operand represents no value (result of a function call w/o result)
	builtin                      // operand is a built-in function
	typexpr                      // operand is a type
	constant_                    // operand is a constant; the operand's typ is a Basic type
	variable                     // operand is an addressable variable
	mapindex                     // operand is a map index expression (acts like a variable on lhs, commaok on rhs of an assignment)
	value                        // operand is a computed value
	commaok                      // like value, but operand may be used in a comma,ok expression
	commaerr                     // like commaok, but second value is error, not boolean
	cgofunc                      // operand is a cgo function
)

//go:linkname checker_infer go/types.(*Checker).infer
func checker_infer(check *types.Checker, posn positioner, tparams []*types.TypeParam, targs []types.Type, params *types.Tuple, args []*operand) (result []types.Type)

func infer(pkg *Package, posn positioner, tparams []*types.TypeParam, targs []types.Type, params *types.Tuple, args []*operand) (result []types.Type, err error) {
	conf := &types.Config{
		Error: func(e error) {
			err = e
			if terr, ok := e.(types.Error); ok {
				err = errors.New(terr.Msg)
			}
		},
	}
	checker := types.NewChecker(conf, pkg.Fset, pkg.Types, nil)
	result = checker_infer(checker, posn, tparams, targs, params, args)
	return
}
