package packages

import (
	"bytes"
	"encoding/json"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/tools/go/gcexportdata"
)

// ----------------------------------------------------------------------------

type Importer struct {
	loaded map[string]*types.Package
	fset   *token.FileSet
	dir    string
}

// NewImporter creates an Importer object that meets types.Importer interface.
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
	expfile := FindExport(p.dir, pkgPath)
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

// ----------------------------------------------------------------------------

type listExport struct {
	Export string `json:"Export"`
}

// FindExport lookups export file (.a) of a package by its pkgPath.
// It returns empty if pkgPath not found.
func FindExport(dir, pkgPath string) (expfile string) {
	var ret listExport
	if data, err := golistExport(dir, pkgPath); err == nil {
		json.Unmarshal(data, &ret)
	}
	return ret.Export
}

func golistExport(dir, pkgPath string) (ret []byte, err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "list", "-json", "-export", pkgPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err = cmd.Run()
	if err == nil {
		ret = stdout.Bytes()
	}
	return
}

// ----------------------------------------------------------------------------
