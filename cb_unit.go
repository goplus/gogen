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
	"strings"
)

// ----------------------------------------------------------------------------

type objectID struct {
	pkg  string
	name string
}

type typeUnits = map[string]constant.Value

// const Gopu_XXX = "mm=1,cm=10,dm=100,m=1000"
func (p *Package) buildTypeUnits(id objectID, tok token.Token) (ret typeUnits, ok bool) {
	v, ok := p.lookupTypeUnitsVal(id)
	if ok {
		units := strings.Split(v, ",")
		ret = make(typeUnits, len(units))
		for _, unit := range units {
			if pos := strings.Index(unit, "="); pos > 0 {
				ret[unit[:pos]] = constant.MakeFromLiteral(unit[pos+1:], tok, 0)
			}
		}
	}
	return
}

/*
"ns": time.Nanosecond,
"us": time.Microsecond,
"µs": time.Microsecond,
"ms": time.Millisecond,
"s":  time.Second,
"m":  time.Minute,
"h":  time.Hour,
"d":  24 * time.Hour,
*/
func (p *Package) lookupTypeUnitsVal(id objectID) (string, bool) {
	if id.name == "Duration" && id.pkg == "time" { // time.Duration
		const tv = "ns=1,us=1000,µs=1000,ms=1000000,s=1000000000,m=60000000000,h=3600000000000,d=86400000000000"
		return tv, true
	}
	imp := p.Import(id.pkg)
	if ounits := imp.TryRef("Gopu_" + id.name); ounits != nil {
		if v := ounits.(*types.Const).Val(); v.Kind() == constant.String {
			return constant.StringVal(v), true
		}
	}
	return "", false
}

// ----------------------------------------------------------------------------

type unitMgr struct {
	units map[objectID]typeUnits
}

func (p *unitMgr) init() {
	units := make(map[objectID]typeUnits)
	p.units = units
}

func (p *Package) getUnits(id objectID, tok token.Token) (units typeUnits, ok bool) {
	units, ok = p.units[id]
	if !ok {
		if units, ok = p.buildTypeUnits(id, tok); ok {
			p.units[id] = units
		}
	}
	return
}

// ----------------------------------------------------------------------------

// ValWithUnit func
func (p *CodeBuilder) ValWithUnit(v *ast.BasicLit, t types.Type, unit string) *CodeBuilder {
	if debugInstr {
		log.Println("ValWithUnit", v.Value, t, unit)
	}
	named, ok := t.(*types.Named)
	if !ok {
		log.Panicf("TODO: ValWithUnit: `%v` isn't a named type", t)
	}
	pkg := p.pkg
	e := toExpr(pkg, v, v)
	ot := named.Obj()
	id := objectID{ot.Pkg().Path(), ot.Name()}
	units, ok := pkg.getUnits(id, token.INT) // TODO(xsw): INT or FLOAT
	if !ok {
		log.Panicf("TODO: ValWithUnit: no units of `%s.%s` found", id.pkg, id.name)
	}
	u, ok := units[unit]
	if !ok {
		log.Panicf("TODO: ValWithUnit: unknown unit `%s` for `%s.%s`", unit, id.pkg, id.name)
	}
	val := constant.BinaryOp(e.CVal, token.MUL, u)
	e.CVal = val
	e.Val = &ast.BasicLit{Kind: token.INT, Value: val.ExactString()}
	e.Type = t
	p.Val(e, v)
	return p
}

// ----------------------------------------------------------------------------
