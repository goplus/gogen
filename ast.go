package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strconv"
	"strings"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

func ident(name string) *ast.Ident {
	return &ast.Ident{Name: name}
}

func boolean(v bool) *ast.Ident {
	if v {
		return &ast.Ident{Name: "true"}
	}
	return &ast.Ident{Name: "false"}
}

func newField(name string, typ types.Type) *ast.Field {
	return &ast.Field{
		Names: []*ast.Ident{ident(name)},
		Type:  toType(typ),
	}
}

func toRecv(recv *types.Var) *ast.FieldList {
	return &ast.FieldList{List: []*ast.Field{newField(recv.Name(), recv.Type())}}
}

// -----------------------------------------------------------------------------
// function type

func toFieldList(t *types.Tuple) []*ast.Field {
	if t == nil {
		return nil
	}
	n := t.Len()
	flds := make([]*ast.Field, n)
	for i := 0; i < n; i++ {
		item := t.At(i)
		names := []*ast.Ident{ident(item.Name())}
		typ := toType(item.Type())
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

func toFuncType(sig *types.Signature) *ast.FuncType {
	params := toFieldList(sig.Params())
	results := toFieldList(sig.Results())
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

func toType(typ types.Type) ast.Expr {
	switch t := typ.(type) {
	case *types.Basic: // bool, int, etc
		return toBasicType(t)
	case *types.Slice:
		return toSliceType(t)
	case *types.Array:
		return toArrayType(t)
	}
	log.Panicln("TODO: toType -", typ)
	return nil
}

func toBasicType(t *types.Basic) ast.Expr {
	if t.Kind() == types.UnsafePointer {
		return &ast.SelectorExpr{X: &ast.Ident{Name: "unsafe"}, Sel: &ast.Ident{Name: "Pointer"}}
	}
	return &ast.Ident{Name: t.Name()}
}

func toSliceType(t *types.Slice) ast.Expr {
	return &ast.ArrayType{Elt: toType(t.Elem())}
}

func toArrayType(t *types.Array) ast.Expr {
	len := &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(t.Len(), 10)}
	return &ast.ArrayType{Len: len, Elt: toType(t.Elem())}
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
	case types.Object:
		return toObject(pkg, v)
	}
	panic("TODO: toExpr")
}

func toBasicKind(tok token.Token) types.BasicKind {
	switch tok {
	case token.INT:
		return types.UntypedInt
	case token.STRING:
		return types.UntypedString
	case token.CHAR:
		return types.UntypedRune
	case token.FLOAT:
		return types.UntypedFloat
	case token.IMAG:
		return types.UntypedComplex
	}
	panic("TODO: unknown Token")
}

func toObject(pkg *Package, v types.Object) internal.Elem {
	return internal.Elem{
		Val:  toObjectExpr(pkg, v),
		Type: v.Type(),
	}
}

func toObjectExpr(pkg *Package, v types.Object) ast.Expr {
	atPkg := v.Pkg()
	if atPkg == pkg.Types { // at this package
		return ident(v.Name())
	}
	if atPkg == pkg.builtin { // at builtin package
		if isBuiltinOp(v) {
			return toOperatorExpr(v.Name())
		}
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
		name := fullName[pos:]
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
		"_Add":    {token.ADD, 2},
		"_Sub":    {token.SUB, 2},
		"_Mul":    {token.MUL, 2},
		"_Quo":    {token.QUO, 2},
		"_Rem":    {token.REM, 2},
		"_Or":     {token.OR, 2},
		"_Xor":    {token.XOR, 2},
		"_And":    {token.AND, 2},
		"_AndNot": {token.AND_NOT, 2},

		"_Lsh": {token.SHL, 2},
		"_Rsh": {token.SHR, 2},

		"_LT": {token.LSS, 2},
		"_LE": {token.LEQ, 2},
		"_GT": {token.GTR, 2},
		"_GE": {token.GEQ, 2},
		"_EQ": {token.EQL, 2},
		"_NE": {token.NEQ, 2},

		"_Neg": {token.SUB, 1},
		"_Not": {token.XOR, 1},
	}
)

func toFuncCall(fn internal.Elem, args []internal.Elem) internal.Elem {
	sig, ok := fn.Type.(*types.Signature)
	if !ok {
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
			panic("TODO: not enough function parameters")
		}
		tyVariadic, ok := params.At(n1).Type().(*types.Slice)
		if !ok {
			panic("TODO: tyVariadic not a slice")
		}
		matchFuncArgs(tyArgs[:n1], params)
		matchElemType(tyArgs[n1:], tyVariadic.Elem())
	} else {
		if params.Len() != n {
			panic("TODO: unmatched function parameters count")
		}
		matchFuncArgs(tyArgs, params)
	}
	tyRet := toRetType(sig)
	switch t := fn.Val.(type) {
	case *ast.BinaryExpr:
		t.X, t.Y = valArgs[0], valArgs[1]
		return internal.Elem{Val: t, Type: tyRet}
	case *ast.UnaryExpr:
		t.X = valArgs[0]
		return internal.Elem{Val: t, Type: tyRet}
	default:
		return internal.Elem{
			Val:  &ast.CallExpr{Fun: fn.Val, Args: valArgs},
			Type: tyRet,
		}
	}
}

func toRetType(sig *types.Signature) types.Type {
	switch t := sig.Results(); t.Len() {
	case 1:
		return t.At(0).Type()
	case 0:
		return nil
	default:
		return t
	}
}

func matchFuncArgs(args []types.Type, params *types.Tuple) {
	for i, arg := range args {
		matchType(arg, params.At(i).Type())
	}
}

func matchElemType(vals []types.Type, elt types.Type) {
	for _, val := range vals {
		matchType(val, elt)
	}
}

// -----------------------------------------------------------------------------

func assignMatchType(stmt *ast.AssignStmt, r internal.Elem, val internal.Elem) {
	if rt, ok := r.Type.(*refType); ok {
		matchType(rt.typ, val.Type)
	} else {
		panic("TODO: unassignable")
	}
}

// -----------------------------------------------------------------------------
