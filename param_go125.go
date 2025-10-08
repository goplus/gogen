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

// setParamOptional marks a parameter as optional using types.Var.SetKind (Go 1.25+).
func (p *Package) setParamOptional(param *types.Var) {
	param.SetKind(-1)
}

// IsParamOptional checks if a parameter is marked as optional using types.Var.Kind (Go 1.25+).
func (p *Package) IsParamOptional(param *types.Var) bool {
	return param.Kind() == -1
}
