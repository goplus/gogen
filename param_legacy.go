// Copyright 2021 The XGo Authors (xgo.dev)
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !go1.25

package gogen

import (
	"fmt"
	"go/types"
)

// optionalVars manages optional parameter metadata for Go < 1.25.
type optionalVars struct {
	paramsMeta map[*types.Var]VarKind
}

// setParamOptional marks a parameter as optional using a map (for Go < 1.25).
func (o *optionalVars) setParamOptional(param *types.Var) {
	if o.paramsMeta == nil {
		o.paramsMeta = make(map[*types.Var]VarKind)
	}
	o.paramsMeta[param] = ParamOptionalVar
}

// isParamOptional checks if a parameter is marked as optional using the map (for Go < 1.25).
func (o *optionalVars) isParamOptional(param *types.Var) bool {
	return o.paramsMeta[param] == ParamOptionalVar
}

func (o *optionalVars) SetVarKind(v *types.Var, kind VarKind) {
	if o.paramsMeta == nil {
		o.paramsMeta = make(map[*types.Var]VarKind)
	}
	o.paramsMeta[v] = kind
}

func (o *optionalVars) VarKind(v *types.Var) VarKind {
	return o.paramsMeta[v]
}

const varKindLegacy = true

// A VarKind discriminates the various kinds of variables.
type VarKind uint8

const (
	_          VarKind = iota // (not meaningful)
	PackageVar                // a package-level variable
	LocalVar                  // a local variable
	RecvVar                   // a method receiver variable
	ParamVar                  // a function parameter variable
	ResultVar                 // a function result variable
	FieldVar                  // a struct field

	ParamOptionalVar VarKind = 0xff
)

var varKindNames = [...]string{
	0:          "VarKind(0)",
	PackageVar: "PackageVar",
	LocalVar:   "LocalVar",
	RecvVar:    "RecvVar",
	ParamVar:   "ParamVar",
	ResultVar:  "ResultVar",
	FieldVar:   "FieldVar",
}

func (kind VarKind) String() string {
	if 0 <= kind && int(kind) < len(varKindNames) {
		return varKindNames[kind]
	} else if kind == ParamOptionalVar {
		return "ParamOptional"
	}
	return fmt.Sprintf("VarKind(%d)", kind)
}
