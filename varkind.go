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

package gogen

import "fmt"

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

var varKindNames = map[VarKind]string{
	0:                "VarKind(0)",
	PackageVar:       "PackageVar",
	LocalVar:         "LocalVar",
	RecvVar:          "RecvVar",
	ParamVar:         "ParamVar",
	ResultVar:        "ResultVar",
	FieldVar:         "FieldVar",
	ParamOptionalVar: "ParamOptional",
}

func (kind VarKind) String() string {
	if s, ok := varKindNames[kind]; ok {
		return s
	}
	return fmt.Sprintf("VarKind(%d)", kind)
}
