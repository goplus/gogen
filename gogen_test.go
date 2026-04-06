/*
Copyright 2026 The XGo Authors (xgo.dev)
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gogen_test

import (
	"bytes"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/goplus/gogen"
	"github.com/goplus/gogen/packages"
)

var (
	gblFset   *token.FileSet
	gblImp    types.Importer
	handleErr func(err error)
)

func init() {
	gogen.SetDebug(gogen.DbgFlagAll)
	gblFset = token.NewFileSet()
	gblImp = packages.NewImporter(gblFset)
}

func ctxRef(pkg *gogen.Package, name string) gogen.Ref {
	_, o := pkg.CB().Scope().LookupParent(name, token.NoPos)
	return o
}

type eventRecorder struct{}

func (p eventRecorder) Member(id ast.Node, obj types.Object) {}
func (p eventRecorder) Call(fn ast.Node, obj types.Object)   {}

type txtNode struct {
	Msg string
	pos token.Pos
}

func (p *txtNode) Pos() token.Pos {
	return p.pos
}

func (p *txtNode) End() token.Pos {
	return p.pos + token.Pos(len(p.Msg))
}

var (
	mutex         sync.Mutex
	pos2Positions = map[token.Pos]token.Position{}
)

// text, line, column
func source(text string, args ...interface{}) ast.Node {
	if len(args) < 2 {
		return &txtNode{Msg: text}
	}
	pos := position(args[0].(int), args[1].(int))
	return &txtNode{Msg: text, pos: pos}
}

const (
	posBits = 16
	posMask = (1 << posBits) - 1
)

func position(line, column int) token.Pos {
	mutex.Lock()
	defer mutex.Unlock()

	pos := token.Pos(len(pos2Positions)+1) << posBits
	pos2Positions[pos] = token.Position{Filename: "./foo.gop", Line: line, Column: column}
	return pos
}

type nodeInterp struct{}

func (p nodeInterp) Position(pos token.Pos) (ret token.Position) {
	mutex.Lock()
	defer mutex.Unlock()
	ret = pos2Positions[(pos &^ posMask)]
	ret.Column += int(pos & posMask)
	return
}

func (p nodeInterp) Caller(node ast.Node) string {
	t := node.(*txtNode)
	idx := strings.Index(t.Msg, "(")
	if idx > 0 {
		return t.Msg[:idx]
	}
	return t.Msg
}

func (p nodeInterp) LoadExpr(node ast.Node) string {
	t := node.(*txtNode)
	return t.Msg
}

func newMainPackage(
	implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	return newPackage("main", implicitCast...)
}

func newPackage(
	name string, implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	conf := &gogen.Config{
		Fset:            gblFset,
		Importer:        gblImp,
		Recorder:        eventRecorder{},
		NodeInterpreter: nodeInterp{},
		DbgPositioner:   nodeInterp{},
	}
	if len(implicitCast) > 0 {
		conf.CanImplicitCast = implicitCast[0]
	}
	if handleErr != nil {
		conf.HandleErr = handleErr
		handleErr = nil
	}
	return gogen.NewPackage("", name, conf)
}

func domTest(t *testing.T, pkg *gogen.Package, expected string) {
	domTestEx(t, pkg, expected, "")
}

func domTestEx(t *testing.T, pkg *gogen.Package, expected string, fname string) {
	var b bytes.Buffer
	t.Helper()
	err := gogen.WriteTo(&b, pkg, fname)
	if err != nil {
		t.Fatal("gogen.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\nResult:\n%s\nExpected:\n%s\n", result, expected)
	}
	log.Printf("====================== %s End =========================\n", t.Name())
}
