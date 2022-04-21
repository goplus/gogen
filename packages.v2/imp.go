package packages

import (
	"go/importer"
	"go/token"
	"go/types"
)

func NewImporter(fset *token.FileSet) types.Importer {
	if fset == nil {
		fset = token.NewFileSet()
	}
	return importer.ForCompiler(fset, "source", nil)
}
