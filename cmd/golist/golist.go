package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"strings"
)

var (
	verbose = flag.Bool("v", false, "print verbose information")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: golist [source.go ...]\n")
}

func isDir(name string) bool {
	if fi, err := os.Lstat(name); err == nil {
		return fi.IsDir()
	}
	return false
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
		return
	}
	initGoEnv()

	var files []*ast.File

	// Parse the input string, []byte, or io.Reader,
	// recording position information in fset.
	// ParseFile returns an *ast.File, a syntax tree.
	fset := token.NewFileSet()
	if infile := flag.Arg(0); isDir(infile) {
		pkgs, first := parser.ParseDir(fset, infile, func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		check(first)
		for name, pkg := range pkgs {
			if !strings.HasSuffix(name, "_test") {
				for _, f := range pkg.Files {
					files = append(files, f)
				}
				break
			}
		}
	} else {
		for i, n := 0, flag.NArg(); i < n; i++ {
			f, err := parser.ParseFile(fset, flag.Arg(i), nil, 0)
			check(err)
			files = append(files, f)
		}
	}

	// A Config controls various options of the type checker.
	// The defaults work fine except for one setting:
	// we must specify how to deal with imports.
	imp := importer.ForCompiler(fset, "source", nil)
	conf := types.Config{
		Importer:                 imp,
		IgnoreFuncBodies:         true,
		DisableUnusedImportCheck: true,
	}

	// Type-check the package containing only file f.
	// Check returns a *types.Package.
	pkg, err := conf.Check("", fset, files, nil)
	check(err)

	scope := pkg.Scope()
	names := scope.Names()
	for _, name := range names {
		fmt.Println(scope.Lookup(name))
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
