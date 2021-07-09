package gox

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

var (
	underscore ast.Expr = &ast.Ident{Name: "_"}
)

func ident(name string) *ast.Ident {
	return &ast.Ident{Name: name}
}

func boolean(v bool) *ast.Ident {
	if v {
		return &ast.Ident{Name: "true"}
	}
	return &ast.Ident{Name: "false"}
}

func newField(pkg *Package, name string, typ types.Type) *ast.Field {
	return &ast.Field{
		Names: []*ast.Ident{ident(name)},
		Type:  toType(pkg, typ),
	}
}

func toRecv(pkg *Package, recv *types.Var) *ast.FieldList {
	return &ast.FieldList{List: []*ast.Field{newField(pkg, recv.Name(), recv.Type())}}
}

// -----------------------------------------------------------------------------
// function type

func toFieldList(pkg *Package, t *types.Tuple) []*ast.Field {
	if t == nil {
		return nil
	}
	n := t.Len()
	flds := make([]*ast.Field, n)
	for i := 0; i < n; i++ {
		item := t.At(i)
		names := []*ast.Ident{ident(item.Name())}
		typ := toType(pkg, item.Type())
		flds[i] = &ast.Field{Names: names, Type: typ}
	}
	return flds
}

func toVariadic(fld *ast.Field) {
	t, ok := fld.Type.(*ast.ArrayType)
	if !ok {
		panic("TODO: not a slice type")
	}
	fld.Type = &ast.Ellipsis{Elt: t.Elt}
}

func toFuncType(pkg *Package, sig *types.Signature) *ast.FuncType {
	params := toFieldList(pkg, sig.Params())
	results := toFieldList(pkg, sig.Results())
	if sig.Variadic() {
		n := len(params)
		if n == 0 {
			panic("TODO: toFuncType error")
		}
		toVariadic(params[n-1])
	}
	return &ast.FuncType{
		Params:  &ast.FieldList{List: params},
		Results: &ast.FieldList{List: results},
	}
}

// -----------------------------------------------------------------------------

func toType(pkg *Package, typ types.Type) ast.Expr {
	switch t := typ.(type) {
	case *types.Basic: // bool, int, etc
		return toBasicType(pkg, t)
	case *types.Pointer:
		return &ast.StarExpr{X: toType(pkg, t.Elem())}
	case *types.Named:
		return toNamedType(pkg, t)
	case *types.Interface:
		return toInterface(pkg, t)
	case *types.Slice:
		return &ast.ArrayType{Elt: toType(pkg, t.Elem())}
	case *types.Array:
		return toArrayType(pkg, t)
	case *types.Chan:
		return toChanType(pkg, t)
	case *types.Signature:
		return toFuncType(pkg, t)
	}
	log.Panicln("TODO: toType -", reflect.TypeOf(typ))
	return nil
}

func toBasicType(pkg *Package, t *types.Basic) ast.Expr {
	if t.Kind() == types.UnsafePointer {
		return toObjectExpr(pkg, pkg.Import("unsafe").Ref("Pointer"))
	}
	return &ast.Ident{Name: t.Name()}
}

func toNamedType(pkg *Package, t *types.Named) ast.Expr {
	o := t.Obj()
	if at := o.Pkg(); at == nil || at == pkg.Types {
		return &ast.Ident{Name: o.Name()}
	}
	panic("TODO: toNamedType")
}

func toChanType(pkg *Package, t *types.Chan) ast.Expr {
	return &ast.ChanType{Value: toType(pkg, t.Elem()), Dir: chanDirs[t.Dir()]}
}

var (
	chanDirs = [...]ast.ChanDir{
		types.SendRecv: ast.SEND | ast.RECV,
		types.SendOnly: ast.SEND,
		types.RecvOnly: ast.RECV,
	}
)

func toArrayType(pkg *Package, t *types.Array) ast.Expr {
	len := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(t.Len(), 10)}
	return &ast.ArrayType{Len: len, Elt: toType(pkg, t.Elem())}
}

func toInterface(pkg *Package, t *types.Interface) ast.Expr {
	var flds []*ast.Field
	for i, n := 0, t.NumExplicitMethods(); i < n; i++ {
		fn := t.ExplicitMethod(i)
		name := ident(fn.Name())
		typ := toFuncType(pkg, fn.Type().(*types.Signature))
		fld := &ast.Field{Names: []*ast.Ident{name}, Type: typ}
		flds = append(flds, fld)
	}
	for i, n := 0, t.NumEmbeddeds(); i < n; i++ {
		panic("TODO: interface embedded")
	}
	return &ast.InterfaceType{Methods: &ast.FieldList{List: flds}}
}

// -----------------------------------------------------------------------------
// expression

func toExpr(pkg *Package, val interface{}) internal.Elem {
	if val == nil {
		return internal.Elem{
			Val:  ident("nil"),
			Type: types.Typ[types.UntypedNil],
		}
	}
	switch v := val.(type) {
	case *Var:
		if isUnbound(v.typ) {
			panic("TODO: variable type is unbound")
		}
		return internal.Elem{
			Val:  ident(v.name),
			Type: v.typ,
		}
	case *ast.BasicLit:
		return internal.Elem{
			Val:  v,
			Type: types.Typ[toBasicKind(v.Kind)],
		}
	case types.Object:
		return toObject(pkg, v)
	case int:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(v)},
			Type: types.Typ[types.UntypedInt],
		}
	case string:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(v)},
			Type: types.Typ[types.UntypedString],
		}
	case bool:
		return internal.Elem{
			Val:  boolean(v),
			Type: types.Typ[types.UntypedBool],
		}
	case rune:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.CHAR, Value: strconv.QuoteRune(v)},
			Type: types.Typ[types.UntypedRune],
		}
	case float64:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.FLOAT, Value: strconv.FormatFloat(v, 'g', -1, 64)},
			Type: types.Typ[types.UntypedFloat],
		}
	}
	panic("TODO: toExpr")
}

func toBasicKind(tok token.Token) types.BasicKind {
	return tok2BasicKinds[tok]
}

var (
	tok2BasicKinds = [...]types.BasicKind{
		token.INT:    types.UntypedInt,
		token.STRING: types.UntypedString,
		token.CHAR:   types.UntypedRune,
		token.FLOAT:  types.UntypedFloat,
		token.IMAG:   types.UntypedComplex,
	}
)

func toObject(pkg *Package, v types.Object) internal.Elem {
	return internal.Elem{
		Val:  toObjectExpr(pkg, v),
		Type: v.Type(),
	}
}

func toObjectExpr(pkg *Package, v types.Object) ast.Expr {
	atPkg, name := v.Pkg(), v.Name()
	if atPkg == nil || atPkg == pkg.Types { // at universe or at this package
		return ident(name)
	}
	if atPkg == pkg.builtin { // at builtin package
		if strings.HasPrefix(name, pkg.prefix.Operator) {
			return toOperatorExpr(name)
		}
		return ident(name)
	}
	importPkg, ok := pkg.importPkgs[atPkg.Path()]
	if !ok {
		log.Panicln("TODO: package not found -", atPkg.Name(), atPkg.Path())
	}
	x := ident(atPkg.Name())
	importPkg.nameRefs = append(importPkg.nameRefs, x)
	return &ast.SelectorExpr{
		X:   x,
		Sel: ident(v.Name()),
	}
}

func toOperatorExpr(fullName string) ast.Expr {
	if pos := strings.LastIndex(fullName, "_"); pos > 0 {
		name := fullName[pos+1:]
		if op, ok := nameToOps[name]; ok {
			if op.Arity == 2 {
				return &ast.BinaryExpr{Op: op.Tok}
			}
			return &ast.UnaryExpr{Op: op.Tok}
		}
	}
	log.Panicln("TODO: not a valid operator -", fullName)
	return nil
}

type operator struct {
	Tok   token.Token
	Arity int
}

var (
	nameToOps = map[string]operator{
		"Add":    {token.ADD, 2},
		"Sub":    {token.SUB, 2},
		"Mul":    {token.MUL, 2},
		"Quo":    {token.QUO, 2},
		"Rem":    {token.REM, 2},
		"Or":     {token.OR, 2},
		"Xor":    {token.XOR, 2},
		"And":    {token.AND, 2},
		"AndNot": {token.AND_NOT, 2},

		"Lsh": {token.SHL, 2},
		"Rsh": {token.SHR, 2},

		"LT": {token.LSS, 2},
		"LE": {token.LEQ, 2},
		"GT": {token.GTR, 2},
		"GE": {token.GEQ, 2},
		"EQ": {token.EQL, 2},
		"NE": {token.NEQ, 2},

		"Neg": {token.SUB, 1},
		"Not": {token.XOR, 1},
	}
)

func toFuncCall(pkg *Package, fn internal.Elem, args []internal.Elem, ellipsis token.Pos) internal.Elem {
	ret, err := checkFuncCall(pkg, fn, args, ellipsis)
	if err != nil {
		panic(err)
	}
	return ret
}

func checkFuncCall(pkg *Package, fn internal.Elem, args []internal.Elem, ellipsis token.Pos) (ret internal.Elem, err error) {
	var it *instantiated
	var sig *types.Signature
	switch t := fn.Type.(type) {
	case *types.Signature:
		sig = t
	case *TemplateSignature: // template function
		sig, it = t.instantiate()
	case *overloadFuncType:
		for _, o := range t.funcs {
			if ret, err = checkFuncCall(pkg, toObject(pkg, o), args, ellipsis); err == nil {
				return
			}
		}
		return
	default:
		panic("TODO: call to non function")
	}
	n := len(args)
	tyArgs := make([]types.Type, n)
	valArgs := make([]ast.Expr, n)
	for i, v := range args {
		valArgs[i] = v.Val
		tyArgs[i] = v.Type
	}
	params := sig.Params()
	if sig.Variadic() {
		n1 := params.Len() - 1
		if n < n1 {
			return internal.Elem{}, errors.New("TODO: not enough function parameters")
		}
		tyVariadic, ok := params.At(n1).Type().(*types.Slice)
		if !ok {
			return internal.Elem{}, errors.New("TODO: tyVariadic not a slice")
		}
		if err = checkMatchFuncArgs(pkg, tyArgs[:n1], params); err != nil {
			return
		}
		if err = checkMatchElemType(pkg, tyArgs[n1:], tyVariadic.Elem()); err != nil {
			return
		}
	} else {
		if params.Len() != n {
			return internal.Elem{}, errors.New("TODO: unmatched function parameters count")
		}
		if err = checkMatchFuncArgs(pkg, tyArgs, params); err != nil {
			return
		}
	}
	tyRet := toRetType(sig.Results(), it)
	switch t := fn.Val.(type) {
	case *ast.BinaryExpr:
		t.X, t.Y = valArgs[0], valArgs[1]
		return internal.Elem{Val: t, Type: tyRet}, nil
	case *ast.UnaryExpr:
		t.X = valArgs[0]
		return internal.Elem{Val: t, Type: tyRet}, nil
	default:
		return internal.Elem{
			Val:  &ast.CallExpr{Fun: fn.Val, Args: valArgs, Ellipsis: ellipsis},
			Type: tyRet,
		}, nil
	}
}

func toRetType(t *types.Tuple, it *instantiated) types.Type {
	if t == nil {
		return nil
	} else if t.Len() == 1 {
		return it.normalize(t.At(0).Type())
	}
	return it.normalizeTuple(t)
}

func checkMatchFuncArgs(pkg *Package, args []types.Type, params *types.Tuple) error {
	for i, arg := range args {
		if err := checkMatchType(pkg, arg, params.At(i).Type()); err != nil {
			return err
		}
	}
	return nil
}

func checkMatchFuncResults(pkg *Package, rets []internal.Elem, results *types.Tuple) error {
	n := len(rets)
	need := results.Len()
	switch n {
	case 0:
		if need > 0 && isUnnamedParams(results) {
			return errors.New("TODO: return without value")
		}
		return nil
	case 1:
		if need > 1 {
			if t, ok := rets[0].Type.(*types.Tuple); ok {
				if n1 := t.Len(); n1 != need {
					return fmt.Errorf("TODO: require %d results, but got %d", need, n1)
				}
				for i := 0; i < need; i++ {
					if err := checkMatchType(pkg, t.At(i).Type(), results.At(i).Type()); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	if n == need {
		for i := 0; i < need; i++ {
			if err := checkMatchType(pkg, rets[i].Type, results.At(i).Type()); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("TODO: require %d results, but got %d", need, n)
}

func isUnnamedParams(t *types.Tuple) bool {
	if t == nil {
		return true
	}
	for i, n := 0, t.Len(); i < n; i++ {
		if t.At(i).Name() == "" {
			return true
		}
	}
	return false
}

func checkMatchElemType(pkg *Package, vals []types.Type, elt types.Type) error {
	for _, val := range vals {
		if err := checkMatchType(pkg, val, elt); err != nil {
			return err
		}
	}
	return nil
}

func assignMatchType(pkg *Package, r internal.Elem, valTy types.Type) {
	if rt, ok := r.Type.(*refType); ok {
		if err := checkMatchType(pkg, rt.typ, valTy); err != nil {
			panic(err)
		}
	} else if r.Val == underscore {
		// do nothing
	} else {
		panic("TODO: unassignable")
	}
}

func checkMatchType(pkg *Package, arg, param types.Type) error {
	if doMatchType(pkg, arg, param) {
		return nil
	}
	return fmt.Errorf("TODO: can't assign %v to %v", arg, param)
}

func doMatchType(pkg *Package, arg, param types.Type) bool {
	if isUnboundParam(param) {
		if _, ok := arg.(*unboundType); ok {
			panic("TODO: don't pass unbound variables as template function params.")
		}
		return boundType(pkg, arg, param)
	}
	if t, ok := arg.(*unboundType); ok { // variable to bound type
		if t.bound == nil {
			param = types.Default(param)
			t.bound = param
			t.v.typ = param
			*t.v.ptype = toType(pkg, param)
		}
		arg = t.bound
	}
	return types.AssignableTo(arg, param)
}

// -----------------------------------------------------------------------------
