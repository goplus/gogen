package workref

import (
	"github.com/goplus/gogen/packages"
)

var NewImp = packages.NewImporter

func init() {
	_ = NewImp(nil)
}
