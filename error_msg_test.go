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

package gox_test

import (
	"bytes"
	"go/types"
	"testing"

	"github.com/goplus/gox"
)

func sourceErrorTest(t *testing.T, msg string, source func(pkg *gox.Package)) {
	defer func() {
		if e := recover(); e != nil {
			if err, ok := e.(*gox.SourceError); ok {
				if err.Error() != msg {
					t.Fatalf("\nError: `%s`\nExpected: `%s`\n", err.Msg, msg)
				}
			} else {
				t.Fatal("Unexpected error:", e)
			}
		} else {
			t.Fatal("no error?")
		}
	}()
	pkg := newMainPackage()
	source(pkg)
	var b bytes.Buffer
	err := gox.WriteTo(&b, pkg)
	t.Fatal("gox.WriteTo no error? -", err)
}

func TestFileLine(t *testing.T) {
	sourceErrorTest(t, "./foo.gop:1 func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam("v", gox.TyByte)
		pkg.CB().SetFileLine(&gox.FileLine{File: "./foo.gop", Line: 1}, false)
		pkg.NewFunc(nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}

func TestErrInit(t *testing.T) {
	sourceErrorTest(t, "func init must have no arguments and no return values", func(pkg *gox.Package) {
		v := pkg.NewParam("v", gox.TyByte)
		pkg.NewFunc(nil, "init", types.NewTuple(v), nil, false).BodyStart(pkg).End()
	})
}
