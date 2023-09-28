gox - Code generator for the Go language
========

[![Build Status](https://github.com/goplus/gox/actions/workflows/go.yml/badge.svg)](https://github.com/goplus/gox/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goplus/gox)](https://goreportcard.com/report/github.com/goplus/gox)
[![GitHub release](https://img.shields.io/github/v/tag/goplus/gox.svg?label=release)](https://github.com/goplus/gox/releases)
[![Coverage Status](https://codecov.io/gh/goplus/gox/branch/main/graph/badge.svg)](https://codecov.io/gh/goplus/gox)
[![GoDoc](https://pkg.go.dev/badge/github.com/goplus/gox.svg)](https://pkg.go.dev/github.com/goplus/gox)

## Quick start

Create a file named `hellogen.go`, like below:

```go
package main

import (
	"go/token"
	"go/types"
	"os"

	"github.com/goplus/gox"
)

func main() {
	pkg := gox.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	v := pkg.NewParam(token.NoPos, "v", types.Typ[types.String]) // v string

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(token.NoPos, "a", "b").Val("Hi").Val(3).EndInit(2). // a, b := "Hi", 3
		NewVarStart(nil, "c").VarVal("b").EndInit(1).                      // var c = b
		NewVar(gox.TyEmptyInterface, "x", "y").                            // var x, y interface{}
		Val(fmt.Ref("Println")).
		/**/ VarVal("a").VarVal("b").VarVal("c"). // fmt.Println(a, b, c)
		/**/ Call(3).EndStmt().
		NewClosure(gox.NewTuple(v), nil, false).BodyStart(pkg).
		/**/ Val(fmt.Ref("Println")).Val(v).Call(1).EndStmt(). // fmt.Println(v)
		/**/ End().
		Val("Hello").Call(1).EndStmt(). // func(v string) { ... } ("Hello")
		End()

	pkg.WriteTo(os.Stdout)
}
```

Try it like:

```shell
go mod init hello
go mod tidy
go run hellogen.go
```

This will dump Go source code to `stdout`. The following is the output content:

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
