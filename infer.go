package gox

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	_ "unsafe"

	"github.com/goplus/gox/internal"
)

type operandMode byte
type builtinId int

type operand struct {
	mode operandMode
	expr ast.Expr
	typ  types.Type
	val  constant.Value
	id   builtinId
}

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

type positioner interface {
	Pos() token.Pos
}

//go:linkname checker_infer go/types.(*Checker).infer
func checker_infer(check *types.Checker, posn positioner, tparams []*types.TypeParam, targs []types.Type, params *types.Tuple, args []*operand) (result []types.Type)

func inferFunc(pkg *Package, fn *internal.Elem, sig *types.Signature, args []*internal.Elem) (types.Type, error) {
	xlist := make([]*operand, len(args))
	for i, arg := range args {
		xlist[i] = &operand{}
		xlist[i].id = builtinId(i)
		xlist[i].expr = arg.Val
		xlist[i].val = arg.CVal
		xlist[i].typ = arg.Type
		xlist[i].mode = value
	}
	n := sig.TypeParams().Len()
	tparams := make([]*types.TypeParam, n)
	for i := 0; i < n; i++ {
		tparams[i] = sig.TypeParams().At(i)
	}
	checker := types.NewChecker(nil, pkg.Fset, pkg.Types, nil)
	targs := checker_infer(checker, fn.Val, tparams, nil, sig.Params(), xlist)
	return types.Instantiate(pkg.cb.ctxt, sig, targs, true)
}
