package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/goplus/gox/internal"
)

// ----------------------------------------------------------------------------

func ident(name string) *ast.Ident {
	return &ast.Ident{Name: name}
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
	panic("TODO: toType")
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
	switch v := val.(type) {
	case int:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(v)},
			Type: types.Typ[types.UntypedInt],
		}
	case string:
		return internal.Elem{
			Val:  &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(v)},
			Type: types.Typ[types.String],
		}
	case types.Object:
		return internal.Elem{
			Val:  toObject(pkg, v),
			Type: v.Type(),
		}
	}
	panic("TODO: toExpr")
}

func toObject(pkg *Package, v types.Object) ast.Expr {
	atPkg := v.Pkg()
	importPkg, ok := pkg.importPkgs[atPkg.Path()]
	if !ok {
		panic("TODO: package not found?")
	}
	x := ident(atPkg.Name())
	importPkg.nameRefs = append(importPkg.nameRefs, x)
	return &ast.SelectorExpr{
		X:   x,
		Sel: ident(v.Name()),
	}
}

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
	return internal.Elem{
		Val:  &ast.CallExpr{Fun: fn.Val, Args: valArgs},
		Type: sig.Results(),
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
		panic("TODO: assign")
	}
}

// -----------------------------------------------------------------------------
