package packages

import (
	"go/token"
	"go/types"
	"os"
	"syscall"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/packages"
)

type Importer struct {
	loaded map[string]*types.Package
	fset   *token.FileSet
	dir    string
}

func NewImporter(fset *token.FileSet, workDir ...string) *Importer {
	dir := ""
	if len(workDir) > 0 {
		dir = workDir[0]
	}
	if fset == nil {
		fset = token.NewFileSet()
	}
	loaded := make(map[string]*types.Package)
	loaded["unsafe"] = types.Unsafe
	return &Importer{loaded: loaded, fset: fset, dir: dir}
}

func (p *Importer) Import(pkgPath string) (pkg *types.Package, err error) {
	if ret, ok := p.loaded[pkgPath]; ok && ret.Complete() {
		return ret, nil
	}
	expfile := findExport(p.dir, pkgPath)
	if expfile == "" {
		return nil, syscall.ENOENT
	}
	return p.loadByExport(expfile, pkgPath)
}

func (p *Importer) loadByExport(expfile string, pkgPath string) (pkg *types.Package, err error) {
	f, err := os.Open(expfile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := gcexportdata.NewReader(f)
	if err == nil {
		pkg, err = gcexportdata.Read(r, p.fset, p.loaded, pkgPath)
	}
	return
}

func findExport(dir, pkgPath string) (expfile string) {
	if expfile, _ = gcexportdata.Find(pkgPath, dir); expfile != "" {
		return
	}
	pkgs, err := packages.Load(&packages.Config{Dir: dir, Mode: packages.NeedExportsFile}, pkgPath)
	if err == nil {
		expfile = pkgs[0].ExportFile
	}
	return
}
