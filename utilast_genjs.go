//go:build !gengo
// +build !gengo

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

package gogen

import (
	"github.com/goplus/gogen/target/js"
)

// ----------------------------------------------------------------------------

type termChecker struct {
	panicCalls map[*js.CallExpr]none
}

// isTerminating reports whether s is a terminating statement.
func (c *termChecker) isTerminating(s js.Stmt, label string) bool {
	panic("todo")
}

// ----------------------------------------------------------------------------

type astVisitor struct {
	pkg  *Package
	file *File
}

func markUsed(this *Package, p *File) {
	panic("todo")
}

// ----------------------------------------------------------------------------
