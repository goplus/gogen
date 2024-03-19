package packages

import "testing"

func TestCacheDelCache(t *testing.T) {
	p := NewImporter(nil)
	if !p.GetPkgCache().DelPkgCache("fmt") {
		t.Fatal("del cache should pass!")
	}
	if p.GetPkgCache().DelPkgCache("fmt") {
		t.Fatal("del cache should fail")
	}
}

func TestRepeatLoadDir(t *testing.T) {
	p := NewImporter(nil)
	p.GetPkgCache().goListExportCache("", "")
}
