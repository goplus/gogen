/*
 Copyright 2021 The XGo Authors (xgo.dev)
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

package foo

import (
	"github.com/goplus/gogen/internal/bar"
)

const (
	GopPackage = true // to indicate this is a Go+ package
)

func CallBar() *bar.Game {
	return nil
}

// -----------------------------------------------------------------------------

type nodeSetIt struct {
}

func (p *nodeSetIt) Next() (key int, val string, ok bool) {
	return
}

// -----------------------------------------------------------------------------

type NodeSet struct {
}

func (p NodeSet) Gop_Enum() *nodeSetIt {
	return &nodeSetIt{}
}

func (p NodeSet) Len__0() int {
	return 0
}

func (p NodeSet) Attr__0(k string, exactlyOne ...bool) (text string, err error) {
	return
}

func (p NodeSet) Attr__1(k, v string) (ret NodeSet) {
	return
}

// -----------------------------------------------------------------------------

type barIt struct {
}

func (p barIt) Next() (val string, ok bool) {
	return
}

type Bar struct {
}

func (p *Bar) Gop_Enum() barIt {
	return barIt{}
}

// -----------------------------------------------------------------------------

type Foo struct {
}

func (p *Foo) Gop_Enum(c func(v string)) {
}

func (a Foo) Gop_Add(b Foo) Foo {
	return Foo{}
}

// -----------------------------------------------------------------------------

type Foo2 struct {
}

func (p Foo2) Gop_Enum(c func(k int, v string)) {
}

// -----------------------------------------------------------------------------

type Foo3 struct {
}

func (p Foo3) Gop_Enum(c func(k int, v string)) int {
	return 0
}

// -----------------------------------------------------------------------------

type Foo4 struct {
}

func (p Foo4) Gop_Enum(c func()) {
}

// -----------------------------------------------------------------------------

type NodeSeter interface {
	Len__0() int
	Attr__0(k string, exactlyOne ...bool) (text string, err error)
	Attr__1(k, v string) (ret NodeSeter)
}

type Data[T any] struct {
	data []T
}

func (p *Data[T]) Size() int {
	return len(p.data)
}

func (p *Data[T]) Add__0(v ...T) {
	p.data = append(p.data, v...)
}

func (p *Data[T]) Add__1(v Data[T]) {
	p.data = append(p.data, v.data...)
}

func (p *Data[T]) IndexOf__0(v T) int {
	return -1
}

func (p *Data[T]) IndexOf__1(pos int, v T) int {
	return -1
}

type DataInterface[T any] interface {
	Size() int
	Add__0(v ...T)
	Add__1(v DataInterface[T])
	IndexOf__0(v T) int
	IndexOf__1(pos int, v T) int
}

// -----------------------------------------------------------------------------
