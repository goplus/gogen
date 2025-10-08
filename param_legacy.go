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

import "go/types"

// optionalVars manages optional parameter metadata for Go < 1.25.
type optionalVars struct {
	paramsMeta map[*types.Var]bool
}

// setParamOptional marks a parameter as optional using a map (for Go < 1.25).
func (o *optionalVars) setParamOptional(param *types.Var) {
	if o.paramsMeta == nil {
		o.paramsMeta = make(map[*types.Var]bool)
	}
	o.paramsMeta[param] = true
}

// isParamOptional checks if a parameter is marked as optional using the map (for Go < 1.25).
func (o *optionalVars) isParamOptional(param *types.Var) bool {
	if o.paramsMeta == nil {
		return false
	}
	return o.paramsMeta[param]
}
