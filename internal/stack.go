/*
 Copyright 2021 The GoPlus Authors (goplus.org)
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

package internal

import (
	"go/ast"
	"go/constant"
	"go/types"
)

// -----------------------------------------------------------------------------

const defaultStkSize = 64

type Elem struct {
	Val  ast.Expr
	Type types.Type
	CVal constant.Value
	Src  ast.Node
}

// A Stack represents a FILO container.
type Stack struct {
	data []*Elem
}

// NewStack creates a Stack instance.
func NewStack() (p *Stack) {
	return &Stack{data: make([]*Elem, 0, defaultStkSize)}
}

// Init initializes this Stack object.
func (p *Stack) Init() {
	p.data = make([]*Elem, 0, defaultStkSize)
}

// Get returns the value at specified index.
func (p *Stack) Get(idx int) *Elem {
	return p.data[len(p.data)+idx]
}

// Set returns the value at specified index.
func (p *Stack) Set(idx int, v *Elem) {
	p.data[len(p.data)+idx] = v
}

// GetArgs returns all arguments of a function.
func (p *Stack) GetArgs(arity int) []*Elem {
	return p.data[len(p.data)-arity:]
}

// Ret pops n values from this stack, and then pushes results.
func (p *Stack) Ret(arity int, results ...*Elem) {
	p.data = append(p.data[:len(p.data)-arity], results...)
}

// Push pushes a value into this stack.
func (p *Stack) Push(v *Elem) {
	p.data = append(p.data, v)
}

// PopN pops n elements.
func (p *Stack) PopN(n int) {
	p.data = p.data[:len(p.data)-n]
}

// Pop pops a value from this stack.
func (p *Stack) Pop() *Elem {
	n := len(p.data)
	v := p.data[n-1]
	p.data = p.data[:n-1]
	return v
}

// Len returns count of stack elements.
func (p *Stack) Len() int {
	return len(p.data)
}

// SetLen sets count of stack elements.
func (p *Stack) SetLen(base int) {
	p.data = p.data[:base]
}

// -----------------------------------------------------------------------------
