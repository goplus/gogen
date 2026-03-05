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

//go:build go1.25

package gogen

import "go/types"

// optionalVars manages optional parameter metadata for Go 1.25+.
type optionalVars struct{}

// setParamOptional marks a parameter as optional using types.Var.SetKind (Go 1.25+).
func (o *optionalVars) setParamOptional(param *types.Var) {
	param.SetKind(types.VarKind(ParamOptionalVar))
}

// isParamOptional checks if a parameter is marked as optional using types.Var.Kind (Go 1.25+).
func (o *optionalVars) isParamOptional(param *types.Var) bool {
	return param.Kind() == types.VarKind(ParamOptionalVar)
}

func (o *optionalVars) SetVarKind(v *types.Var, kind VarKind) {
	v.SetKind(types.VarKind(kind))
}

func (o *optionalVars) VarKind(v *types.Var) VarKind {
	return VarKind(v.Kind())
}

const varKindLegacy = false

type VarKind types.VarKind

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

func (kind VarKind) String() string {
	if kind == ParamOptionalVar {
		return "ParamOptional"
	}
	return types.VarKind(kind).String()
}
