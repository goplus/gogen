gogen - Code generator for the Go language
========

[![Build Status](https://github.com/goplus/gogen/actions/workflows/go.yml/badge.svg)](https://github.com/goplus/gogen/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goplus/gogen)](https://goreportcard.com/report/github.com/goplus/gogen)
[![GitHub release](https://img.shields.io/github/v/tag/goplus/gogen.svg?label=release)](https://github.com/goplus/gogen/releases)
[![Coverage Status](https://codecov.io/gh/goplus/gogen/branch/main/graph/badge.svg)](https://codecov.io/gh/goplus/gogen)
[![GoDoc](https://pkg.go.dev/badge/github.com/goplus/gogen.svg)](https://pkg.go.dev/github.com/goplus/gogen)

`gogen` is a general-purpose Go code generation toolkit. Like the Go compiler, It can perform type checking of expressions. For example, if you generate an expression like `"Hello" + 1`, it will report the corresponding error.

## Quick start

Create a file named `hellogen.go`, like below:

```go
package main

import (
	"go/token"
	"go/types"
	"os"

	"github.com/goplus/gogen"
)

func main() {
	pkg := gogen.NewPackage("", "main", nil)
	fmt := pkg.Import("fmt")
	v := pkg.NewParam(token.NoPos, "v", types.Typ[types.String]) // v string

	pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
		DefineVarStart(token.NoPos, "a", "b").Val("Hi").Val(3).EndInit(2). // a, b := "Hi", 3
		NewVarStart(nil, "c").VarVal("b").EndInit(1).                      // var c = b
		NewVar(gogen.TyEmptyInterface, "x", "y").                          // var x, y interface{}
		Val(fmt.Ref("Println")).
		/**/ VarVal("a").VarVal("b").VarVal("c"). // fmt.Println(a, b, c)
		/**/ Call(3).EndStmt().
		NewClosure(gogen.NewTuple(v), nil, false).BodyStart(pkg).
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

import "fmt"

func main() {
	a, b := "Hi", 3
	var c = b
	var x, y interface{}
	fmt.Println(a, b, c)
	func(v string) {
		fmt.Println(v)
	}("Hello")
}
```
