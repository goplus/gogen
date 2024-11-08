/*
 Copyright 2024 The GoPlus Authors (goplus.org)
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
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"testing"
	"time"
)

func TestTimeDurationUnits(t *testing.T) {
	timeDurationUnits := map[string]time.Duration{
		"ns": time.Nanosecond,
		"us": time.Microsecond,
		"µs": time.Microsecond,
		"ms": time.Millisecond,
		"s":  time.Second,
		"m":  time.Minute,
		"h":  time.Hour,
		"d":  24 * time.Hour,
	}
	pkg := NewPackage("", "foo", nil)
	ret, ok := pkg.buildTypeUnits(objectID{"time", "Duration"}, token.INT)
	if !ok || len(ret) != len(timeDurationUnits) {
		t.Fatal("TestTimeDurationUnits: failed")
	}
	for k, vex := range timeDurationUnits {
		if v := ret[k]; v != constant.MakeInt64(int64(vex)) {
			t.Fatal("TestTimeDurationUnits: failed:", k, v)
		}
	}
}

func TestUserDefinedTypeUnits(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	u := pkg.Import("github.com/goplus/gogen/internal/unit")
	ut := u.Ref("Distance").Type()
	ut2 := u.Ref("NoUnit").Type()
	cb := pkg.CB()
	cb.ValWithUnit(&ast.BasicLit{Value: "1", Kind: token.INT}, ut, "m")
	testValWithUnitPanic(t, "no unit for unit.NoUnit", cb, ut2, "m")
}

func TestValWithUnit(t *testing.T) {
	pkg := NewPackage("", "foo", nil)
	cb := pkg.CB()
	testValWithUnitPanic(t, "no unit for int", cb, types.Typ[types.Int], "m")
	testValWithUnitPanic(t, "y is not unit of time.Duration", cb, namedType("time", "Duration"), "y")
	testValWithUnitPanic(t, "user defined type: not impl", cb, namedType("foo", "Bar"), "m")
	cb.ValWithUnit(&ast.BasicLit{Value: "1", Kind: token.INT}, namedType("time", "Duration"), "m")
}

func namedType(pkgName, tName string) types.Type {
	pkg := types.NewPackage(pkgName, "")
	return types.NewNamed(types.NewTypeName(0, pkg, tName, nil), types.Typ[types.Int64], nil)
}

func testValWithUnitPanic(t *testing.T, name string, cb *CodeBuilder, typ types.Type, unit string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("TestErrValWithUnit: no panic?")
			}
		}()
		cb.ValWithUnit(&ast.BasicLit{Value: "1", Kind: token.INT}, typ, unit)
	})
}
