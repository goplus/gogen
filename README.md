gox - Code generator for the Go language
========

[![Build Status](https://travis-ci.org/goplus/gox.png?branch=master)](https://travis-ci.org/goplus/gox)
[![Go Report Card](https://goreportcard.com/badge/github.com/goplus/gox)](https://goreportcard.com/report/github.com/goplus/gox)
[![GitHub release](https://img.shields.io/github/v/tag/goplus/gox.svg?label=release)](https://github.com/goplus/gox/releases)
[![Coverage Status](https://codecov.io/gh/goplus/gox/branch/master/graph/badge.svg)](https://codecov.io/gh/goplus/gox)
[![GoDoc](https://img.shields.io/badge/godoc-reference-teal.svg)](https://pkg.go.dev/mod/github.com/goplus/gox)

## Tutorials

```go
import (
    "github.com/goplus/gox"
)

var a, b, c *gox.Var

pkg := gox.NewPackage("", "main", nil)

fmt := pkg.Import("fmt")

paramV := pkg.NewParam("v", types.Typ[types.String]) // v string

pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
    NewVar("a", &a).NewVar("b", &b).NewVar("c", &c). // type of variables will be auto detected
    VarRef(a).VarRef(b).Const("Hi").Const(3).Assign(2).EndStmt(). // a, b = "Hi", 3
    VarRef(c).Val(b).Assign(1).EndStmt(). // c = b
    Val(fmt.Ref("Println")).Val(a).Val(b).Val(c).Call(3).EndStmt(). // fmt.Println(a, b, c)
    NewClosure(nil, gox.NewTuple(paramV), false).BodyStart(pkg).
        Val(fmt.Ref("Println")).Val("Hello").Call(1).EndStmt().
        End().
        Val(paramV).Call(0).EndStmt(). // func(v string) { fmt.Println(v) } ("Hello")
    End()

gox.WriteFile("./foo.go", pkg)
```

This will generate a Go source file named `./foo.go`. The following is its content:

```go
package main

import (
    "fmt"
)

func main() {
    var a string
    var b int
    var c int
    a, b = "Hi", 3
    c = b
    fmt.Println(a, b, c)
}
```
