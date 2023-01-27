//go:build go1.18
// +build go1.18

package printer

import (
	"go/ast"
	"go/token"
)

// IndexListExpr is an alias for ast.IndexListExpr.
type IndexListExpr = ast.IndexListExpr

func (p *printer) signature(sig *ast.FuncType) {
	if sig.TypeParams != nil {
		p.parameters(sig.TypeParams, funcTParam)
	}
	if sig.Params != nil {
		p.parameters(sig.Params, funcParam)
	} else {
		p.print(token.LPAREN, token.RPAREN)
	}
	res := sig.Results
	n := res.NumFields()
	if n > 0 {
		// res != nil
		p.print(blank)
		if n == 1 && res.List[0].Names == nil {
			// single anonymous res; no ()'s
			p.expr(stripParensAlways(res.List[0].Type))
			return
		}
		p.parameters(res, funcParam)
	}
}

const TILDE = token.TILDE

func specTypeParams(spec *ast.TypeSpec) *ast.FieldList {
	return spec.TypeParams
}
