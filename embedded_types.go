/*
 Copyright 2026 The XGo Authors (xgo.dev)
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gogen

import (
	"go/ast"
	"go/token"
	"go/types"
)

// embeddedStructs retains Go spellings for embedded types across scopes.
type embeddedStructs struct {
	// typeParamNames applies only while writing a generated alias's constraints.
	typeParamNames map[*types.TypeParam]*ast.Ident
	localTypeUsed  *bool
}

// localEmbeddedStruct records where an anonymous struct using local types was emitted.
type localEmbeddedStruct struct {
	scope *types.Scope
	expr  *ast.StructType
	alias *ast.Ident
}

// structSpelling preserves embedded field types when their names are shadowed.
func (p *Package) structSpelling(t *types.Struct) ast.Expr {
	scope := p.cb.Scope()
	pkgScope := p.Types.Scope()
	if scope == pkgScope {
		return inlineStructType(p, t)
	}

	var usesLocalName, shadowed bool
	for i := 0; i < t.NumFields(); i++ {
		field := t.Field(i)
		if !field.Embedded() {
			continue
		}
		name := embeddedTypeName(field)
		if name == nil || (name.Pkg() != nil && name.Pkg() != p.Types) {
			continue
		}
		if name.Pkg() == p.Types && name.Parent() != pkgScope {
			usesLocalName = true
		}
		_, binding := scope.LookupParent(name.Name(), token.NoPos)
		if !sameTypeBinding(name, binding) {
			shadowed = true
		}
	}
	if shadowed {
		if !usesLocalName {
			return p.packageStructAlias(t)
		}
		if alias := p.localStructAlias(t, scope); alias != nil {
			return alias
		}
		return inlineStructType(p, t)
	}

	expr := inlineStructType(p, t)
	if usesLocalName {
		p.rememberLocalStruct(t, expr, scope)
	}
	return expr
}

// embeddedTypeName returns the object required by the spelling of an embedded field.
func embeddedTypeName(field *types.Var) *types.TypeName {
	if isEmbeddedAny(field) {
		return types.Universe.Lookup("any").(*types.TypeName)
	}
	typ := field.Type()
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	switch t := typ.(type) {
	case *types.Named:
		return t.Obj()
	case *types.Alias:
		return t.Obj()
	case *types.Basic:
		name, _ := types.Universe.Lookup(t.Name()).(*types.TypeName)
		return name
	}
	return nil
}

// isEmbeddedAny reports whether a field embeds the predeclared any type.
func isEmbeddedAny(field *types.Var) bool {
	return field.Embedded() && field.Name() == "any" && types.Identical(field.Type(), TyEmptyInterface)
}

// packageStructAlias creates a package-scope alias, with type parameters for
// field types and type arguments that are supplied at the point of use.
func (p *Package) packageStructAlias(t *types.Struct) ast.Expr {
	if alias := p.file.structAliases[t]; alias != nil {
		return alias
	}
	params := &ast.FieldList{}
	var args []ast.Expr
	parameterize := func(arg ast.Expr) *ast.Field {
		field := &ast.Field{Names: []*ast.Ident{p.newGeneratedIdentifier()}}
		params.List = append(params.List, field)
		args = append(args, arg)
		return field
	}

	list := structFields(t, func(field *types.Var) ast.Expr {
		if expr, ok := p.fieldTypeAtPackageScope(field); ok {
			return expr
		}
		args := structFieldType(p, field)
		if field.Embedded() {
			return p.parameterizeEmbeddedArgs(field.Type(), args, parameterize)
		}
		param := parameterize(args)
		param.Type = &ast.InterfaceType{Methods: &ast.FieldList{}}
		return param.Names[0]
	})
	name := p.newGeneratedIdentifier()
	spec := &ast.TypeSpec{
		Name:   name,
		Assign: 1,
		Type:   &ast.StructType{Fields: &ast.FieldList{List: list}},
	}
	var alias ast.Expr = name
	if len(args) != 0 {
		spec.TypeParams = params
		alias = &ast.IndexListExpr{X: name, Indices: args}
	} else {
		if p.file.structAliases == nil {
			p.file.structAliases = make(map[*types.Struct]*ast.Ident)
		}
		p.file.structAliases[t] = name
	}
	p.file.fileDecls.goDecls = append(p.file.fileDecls.goDecls, typeAliasDecl(spec))
	return alias
}

// fieldTypeAtPackageScope spells a field where its alias declaration would appear.
// A local type name or type parameter requires an argument at the use site.
func (p *Package) fieldTypeAtPackageScope(field *types.Var) (ast.Expr, bool) {
	var local bool
	old := p.localTypeUsed
	p.localTypeUsed = &local
	defer func() { p.localTypeUsed = old }()
	var expr ast.Expr
	p.cb.AtPackageScope(func() {
		expr = structFieldType(p, field)
	})
	return expr, !local
}

// parameterizeEmbeddedArgs keeps the embedded type name and moves its type
// arguments to the use of the generated alias. The generated parameters
// retain the constraints of the embedded type's parameters.
func (p *Package) parameterizeEmbeddedArgs(typ types.Type, expr ast.Expr, parameterize func(ast.Expr) *ast.Field) ast.Expr {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return &ast.StarExpr{X: p.parameterizeEmbeddedArgs(typ.(*types.Pointer).Elem(), t.X, parameterize)}
	case *ast.IndexExpr:
		args := p.aliasTypeParams(typeParameters(typ), p.Types.Scope(), func(int) *ast.Field {
			return parameterize(t.Index)
		})
		return &ast.IndexExpr{X: t.X, Index: args[0]}
	case *ast.IndexListExpr:
		args := p.aliasTypeParams(typeParameters(typ), p.Types.Scope(), func(i int) *ast.Field {
			return parameterize(t.Indices[i])
		})
		return &ast.IndexListExpr{X: t.X, Indices: args}
	}
	return expr
}

// typeParameters returns the parameters declared by a named or alias type.
func typeParameters(typ types.Type) *types.TypeParamList {
	switch t := typ.(type) {
	case *types.Named:
		return t.Origin().TypeParams()
	case *types.Alias:
		return t.Origin().TypeParams()
	}
	return nil
}

// aliasTypeParams copies the original constraints and binds their type
// references to the generated alias's parameters.
func (p *Package) aliasTypeParams(original *types.TypeParamList, scope *types.Scope, parameterAt func(int) *ast.Field) []ast.Expr {
	n := original.Len()
	names := make(map[*types.TypeParam]*ast.Ident, n)
	fields := make([]*ast.Field, n)
	refs := make([]ast.Expr, n)
	for i := range fields {
		fields[i] = parameterAt(i)
		name := fields[i].Names[0]
		names[original.At(i)] = name
		refs[i] = name
	}
	old := p.typeParamNames
	p.typeParamNames = names
	defer func() { p.typeParamNames = old }()
	p.cb.atScope(scope, func() {
		for i, field := range fields {
			field.Type = toType(p, original.At(i).Constraint())
		}
	})
	return refs
}

// rememberLocalStruct retains the spelling of a struct that can only be
// declared in the current or an enclosing function scope.
func (p *Package) rememberLocalStruct(t *types.Struct, expr *ast.StructType, scope *types.Scope) {
	if p.file.localStructs == nil {
		p.file.localStructs = make(map[*types.Struct][]*localEmbeddedStruct)
	}
	p.file.localStructs[t] = append(p.file.localStructs[t], &localEmbeddedStruct{scope: scope, expr: expr})
}

// localStructAlias requests an alias at an earlier spelling of the same type.
func (p *Package) localStructAlias(t *types.Struct, scope *types.Scope) ast.Expr {
	for _, original := range p.file.localStructs[t] {
		if !enclosesScope(original.scope, scope) {
			continue
		}
		if original.alias == nil {
			original.alias = p.newGeneratedIdentifier()
			p.deferTypeAlias(original.expr, typeAliasDecl(&ast.TypeSpec{
				Name: original.alias, Assign: 1, Type: original.expr,
			}))
		}
		return original.alias
	}
	return nil
}

// scopedTypeName preserves the binding of a type name shadowed at its use site.
// Embedded field names are handled by structSpelling because replacing them
// would alter the resulting field name.
func (p *Package) scopedTypeName(name *types.TypeName, expr *ast.Ident) ast.Expr {
	scope, pkgScope := p.cb.Scope(), p.Types.Scope()
	if scope == pkgScope {
		return expr
	}
	_, binding := scope.LookupParent(name.Name(), token.NoPos)
	if binding == nil || sameTypeBinding(name, binding) {
		return expr
	}
	declScope := pkgScope
	var anchor *ast.TypeSpec
	if name.Pkg() == p.Types && name.Parent() != pkgScope {
		declScope = name.Parent()
		anchor = p.file.localTypeSpecs[name]
		if anchor == nil || !enclosesScope(declScope, scope) {
			return expr
		}
	} else {
		_, binding := pkgScope.LookupParent(name.Name(), token.NoPos)
		if !sameTypeBinding(name, binding) {
			return expr
		}
	}

	if alias := p.file.typeNameAliases[name]; alias != nil {
		return alias
	}

	alias := p.newGeneratedIdentifier()
	spec := &ast.TypeSpec{Name: alias, Assign: 1, Type: expr}
	if params := typeParameters(name.Type()); params.Len() != 0 {
		spec.TypeParams = &ast.FieldList{}
		refs := p.aliasTypeParams(params, declScope, func(int) *ast.Field {
			param := &ast.Field{Names: []*ast.Ident{p.newGeneratedIdentifier()}}
			spec.TypeParams.List = append(spec.TypeParams.List, param)
			return param
		})
		spec.Type = &ast.IndexListExpr{X: expr, Indices: refs}
	}
	decl := typeAliasDecl(spec)
	if anchor == nil {
		p.useName(name.Name())
		p.file.fileDecls.goDecls = append(p.file.fileDecls.goDecls, decl)
	} else {
		p.deferTypeAlias(anchor, decl)
	}
	if p.file.typeNameAliases == nil {
		p.file.typeNameAliases = make(map[*types.TypeName]*ast.Ident)
	}
	p.file.typeNameAliases[name] = alias
	return alias
}

// sameTypeBinding reports whether binding can retain name's spelling.
func sameTypeBinding(name *types.TypeName, binding types.Object) bool {
	other, ok := binding.(*types.TypeName)
	return ok && (name == other || types.Identical(other.Type(), name.Type()))
}

// enclosesScope reports whether outer is an ancestor of inner or inner itself.
func enclosesScope(outer, inner *types.Scope) bool {
	for at := inner; at != nil; at = at.Parent() {
		if at == outer {
			return true
		}
	}
	return false
}

// deferTypeAlias records where to insert an alias in an enclosing function.
func (p *Package) deferTypeAlias(anchor ast.Node, decl *ast.GenDecl) {
	if p.file.localTypeAliases == nil {
		p.file.localTypeAliases = make(localAliasDecls)
	}
	p.file.localTypeAliases[anchor] = decl
}

// typeAliasDecl wraps a type alias in its own declaration.
func typeAliasDecl(spec *ast.TypeSpec) *ast.GenDecl {
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

// insertLocalTypeAliases inserts aliases and returns the first referenced alias
// whose declaration site has been removed.
func (f *File) insertLocalTypeAliases() *ast.Ident {
	if len(f.localTypeAliases) == 0 {
		return nil
	}
	for _, decl := range f.fileDecls.goDecls {
		f.localTypeAliases.find(decl)
	}
	if len(f.localTypeAliases) == 0 {
		return nil
	}
	pending := make(map[*ast.Ident]bool, len(f.localTypeAliases))
	for _, decl := range f.localTypeAliases {
		pending[decl.Specs[0].(*ast.TypeSpec).Name] = true
	}
	var missing *ast.Ident
	for _, decl := range f.goDecls() {
		ast.Inspect(decl, func(node ast.Node) bool {
			switch v := node.(type) {
			case *ast.TypeSpec:
				return v.Name != nil && missing == nil
			case *ast.Ident:
				if pending[v] {
					missing = v
				}
			}
			return missing == nil
		})
		if missing != nil {
			return missing
		}
	}
	return nil
}

// localAliasDecls records the syntax nodes that anchor local type aliases.
type localAliasDecls map[ast.Node]*ast.GenDecl

// find collects aliases at node, inserting aliases with narrower scopes as it walks.
func (e localAliasDecls) find(node ast.Node) []ast.Stmt {
	var aliases []ast.Stmt
	ast.Inspect(node, func(n ast.Node) bool {
		if decl := e[n]; decl != nil {
			aliases = append(aliases, &ast.DeclStmt{Decl: decl})
			delete(e, n)
		}
		switch v := n.(type) {
		case *ast.BlockStmt:
			if len(v.List) != 0 {
				switch v.List[0].(type) {
				case *ast.CaseClause, *ast.CommClause:
					// A switch or select body has no declaration statements of its own.
					for _, clause := range v.List {
						aliases = append(aliases, e.find(clause)...)
					}
					return false
				}
			}
			v.List = e.insert(v.List)
			return false
		case *ast.CaseClause:
			for _, expr := range v.List {
				aliases = append(aliases, e.find(expr)...)
			}
			v.Body = e.insert(v.Body)
			return false
		case *ast.CommClause:
			if v.Comm != nil {
				aliases = append(aliases, e.find(v.Comm)...)
			}
			v.Body = e.insert(v.Body)
			return false
		}
		return len(e) != 0
	})
	return aliases
}

// insert inserts aliases just before the statements that supplied their
// spelling, or after a type declaration that may define one of their types.
func (e localAliasDecls) insert(stmts []ast.Stmt) []ast.Stmt {
	if len(e) == 0 {
		return stmts
	}
	var ret []ast.Stmt
	for i, stmt := range stmts {
		aliases := e.find(stmt)
		if len(aliases) != 0 && ret == nil {
			ret = append(make([]ast.Stmt, 0, len(stmts)+len(aliases)), stmts[:i]...)
		}
		if ret == nil {
			continue
		}
		site := stmt
		for {
			label, ok := site.(*ast.LabeledStmt)
			if !ok {
				break
			}
			site = label.Stmt
		}
		if decl, ok := site.(*ast.DeclStmt); ok && decl.Decl.(*ast.GenDecl).Tok == token.TYPE {
			ret = append(ret, stmt)
			ret = append(ret, aliases...)
		} else {
			ret = append(ret, aliases...)
			ret = append(ret, stmt)
		}
	}
	if ret != nil {
		return ret
	}
	return stmts
}
