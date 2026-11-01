package packages

import (
	"unsafe"
)

type Importer struct {
}

func NewImporter(_ unsafe.Pointer) *Importer {
	return &Importer{}
}
