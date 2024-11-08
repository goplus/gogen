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
	"log"
	"time"
)

// ----------------------------------------------------------------------------

// ValWithUnit func
func (p *CodeBuilder) ValWithUnit(v *ast.BasicLit, t types.Type, unit string) *CodeBuilder {
	if debugInstr {
		log.Println("ValWithUnit", v.Value, t, unit)
	}
	named, ok := t.(*types.Named)
	if !ok {
		panic("TODO: ValWithUnit: t isn't a named type")
	}
	pkg := p.pkg
	e := toExpr(pkg, v, v)
	ot := named.Obj()
	if ot.Name() == "Duration" && ot.Pkg().Path() == "time" { // time.Duration
		u, ok := timeDurationUnits[unit]
		if !ok {
			panic("TODO: ValWithUnit: unknown unit of time.Duration - " + unit)
		}
		val := constant.BinaryOp(e.CVal, token.MUL, constant.MakeInt64(int64(u)))
		e.CVal = val
		e.Val = &ast.BasicLit{Kind: token.INT, Value: val.ExactString()}
		e.Type = t
		p.Val(e, v)
	} else {
		/* imp := pkg.Import(ot.Pkg().Path())
		ounits := imp.TryRef("Gopu_" + ot.Name())
		ounits.(*types.Const).Val()
		*/
		panic("TODO: notimpl")
	}
	return p
}

var timeDurationUnits = map[string]time.Duration{
	"ns": time.Nanosecond,
	"us": time.Microsecond,
	"µs": time.Microsecond,
	"ms": time.Millisecond,
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  24 * time.Hour,
}

// ----------------------------------------------------------------------------
