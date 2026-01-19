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

package gogen

import (
	"go/types"
	"testing"

	"github.com/goplus/gogen/internal/typesalias"
)

func Test_embedName(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		typ  types.Type
		want string
	}{
		{
			name: "basic type",
			typ:  types.Typ[types.Int],
			want: "int",
		},
		{
			name: "named type",
			typ:  types.NewNamed(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int], nil),
			want: "MyInt",
		},
		{
			name: "pointer to named type",
			typ:  types.NewPointer(types.NewNamed(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int], nil)),
			want: "MyInt",
		},
		{
			name: "alias type",
			typ:  typesalias.NewAlias(types.NewTypeName(0, nil, "MyInt", nil), types.Typ[types.Int]),
			want: "MyInt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := embedName(tt.typ)
			if got != tt.want {
				t.Errorf("embedName() = %v, want %v", got, tt.want)
			}
		})
	}
}
