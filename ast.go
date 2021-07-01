package gox

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
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
