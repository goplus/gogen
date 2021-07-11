gox - Code generator for the Go language
========

[![Build Status](https://github.com/goplus/gox/actions/workflows/go.yml/badge.svg)](https://github.com/goplus/gox/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goplus/gox)](https://goreportcard.com/report/github.com/goplus/gox)
[![GitHub release](https://img.shields.io/github/v/tag/goplus/gox.svg?label=release)](https://github.com/goplus/gox/releases)
[![Coverage Status](https://codecov.io/gh/goplus/gox/branch/master/graph/badge.svg)](https://codecov.io/gh/goplus/gox)
[![GoDoc](https://img.shields.io/badge/godoc-reference-teal.svg)](https://pkg.go.dev/mod/github.com/goplus/gox)

## Tutorials

```go
import (
	"github.com/goplus/gox"
)

func ctxRef(pkg *gox.Package, name string) gox.Ref {
	return pkg.CB().Scope().Lookup(name)
}

pkg := gox.NewPackage("", "main", nil)

fmt := pkg.Import("fmt")

v := pkg.NewParam("v", types.Typ[types.String]) // v string

pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
	DefineVarStart("a", "b").Val("Hi").Val(3).EndInit(2).   // a, b := "Hi", 3
	NewVarStart(nil, "c").Val(ctxRef(pkg, "b")).EndInit(1). // var c = b
	NewVar(gox.TyEmptyInterface, "x", "y").                 // var x, y interface{}
	Val(fmt.Ref("Println")).
	/**/ Val(ctxRef(pkg, "a")).Val(ctxRef(pkg, "b")).Val(ctxRef(pkg, "c")). // fmt.Println(a, b, c)
	/**/ Call(3).EndStmt().
	NewClosure(gox.NewTuple(v), nil, false).BodyStart(pkg).
	/**/ Val(fmt.Ref("Println")).Val(v).Call(1).EndStmt(). // fmt.Println(v)
	/**/ End().
	Val("Hello").Call(1).EndStmt(). // func(v string) { ... } ("Hello")
	End()

gox.WriteFile("./foo.go", pkg)
```

This will generate a Go source file named `./foo.go`. The following is its content:

```go
package main

import fmt "fmt"

func main() {
	a, b := "Hi", 3
	var c = b
	var x, y interface {
	}
	fmt.Println(a, b, c)
	func(v string) {
		fmt.Println(v)
	}("Hello")
}
```
