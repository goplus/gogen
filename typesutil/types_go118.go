//go:build go1.18
// +build go1.18

/*
 Copyright 2023 The GoPlus Authors (goplus.org)
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

package typesutil

import (
	"go/types"
)

// ----------------------------------------------------------------------------

// A TypeParam represents a type parameter type.
type TypeParam = types.TypeParam

// NewSignatureType creates a new function type for the given receiver,
// receiver type parameters, type parameters, parameters, and results. If
// variadic is set, params must hold at least one parameter and the last
// parameter's core type must be of unnamed slice or bytestring type.
// If recv is non-nil, typeParams must be empty. If recvTypeParams is
// non-empty, recv must be non-nil.
func NewSignatureType(recv *types.Var, recvTypeParams, typeParams []*TypeParam, params, results *types.Tuple, variadic bool) *types.Signature {
	return types.NewSignatureType(recv, recvTypeParams, typeParams, params, results, variadic)
}

// ----------------------------------------------------------------------------
