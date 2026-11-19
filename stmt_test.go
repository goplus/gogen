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
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestForRangeArrayPointerTypes(t *testing.T) {
	const src = `package fixture

type Item int
type Array [2]Item
type ArrayAlias = Array
type Pointer *ArrayAlias
type PointerAlias = Pointer
type Generic[T any] [2]T
type PlainAlias = [2]Item

type Custom [2]Item
type CustomPointer *Custom
type CustomPointerAlias = *Custom
func (*Custom) XGo_Enum() func(func(string) bool) { return nil }

type Legacy [2]Item
type Iterator struct{}
func (Iterator) Next() (string, Item, bool) { return "", 0, false }
func (*Legacy) Gop_Enum() *Iterator { return nil }

type Seq = func(func(string) bool)
type AliasedSeq [2]Item
func (*AliasedSeq) XGo_Enum() Seq { return nil }

type IteratorAlias = Iterator
type AliasedIterator [2]Item
func (*AliasedIterator) Gop_Enum() *IteratorAlias { return nil }

type Continue = bool
type AliasedYieldResult [2]Item
func (*AliasedYieldResult) XGo_Enum() func(func(string) Continue) { return nil }

type AliasedNextResult [2]Item
type BoolAliasIterator struct{}
func (BoolAliasIterator) Next() (string, Item, Continue) { return "", 0, false }
func (*AliasedNextResult) Gop_Enum() BoolAliasIterator { return BoolAliasIterator{} }

type DefinedBool bool

type InvalidParams [2]Item
func (*InvalidParams) XGo_Enum(int) func(func(string) bool) { return nil }

type InvalidYield [2]Item
func (*InvalidYield) XGo_Enum() func(func(string) DefinedBool) { return nil }

type InvalidNext [2]Item
type BadIterator struct{}
func (BadIterator) Next() (string, DefinedBool) { return "", false }
func (*InvalidNext) Gop_Enum() BadIterator { return BadIterator{} }

type RequiredNextArg [2]Item
type RequiredArgIterator struct{}
func (RequiredArgIterator) Next(int, ...int) (string, bool) { return "", false }
func (*RequiredNextArg) XGo_Enum() RequiredArgIterator { return RequiredArgIterator{} }

type OptionalNextArg [2]Item
type OptionalArgIterator struct{}
func (OptionalArgIterator) Next(...int) (string, bool) { return "", false }
func (*OptionalNextArg) XGo_Enum() OptionalArgIterator { return OptionalArgIterator{} }

type VariadicYield [2]Item
func (*VariadicYield) XGo_Enum() func(func(...Item) bool) { return nil }

type Slice []Item
type Recursive *Recursive

var Plain *[2]Item
var Defined *Array
var DefinedPointer Pointer
var Alias PointerAlias
var PointerToAlias *PlainAlias
var Instantiated *Generic[Item]
var WithIterator *Custom
var DefinedPointerWithIterator CustomPointer
var AliasedPointerWithIterator CustomPointerAlias
var WithLegacyIterator *Legacy
var WithAliasedSeq *AliasedSeq
var WithAliasedIterator *AliasedIterator
var WithAliasedYieldResult *AliasedYieldResult
var WithAliasedNextResult *AliasedNextResult
var WithInvalidParams *InvalidParams
var WithInvalidYield *InvalidYield
var WithInvalidNext *InvalidNext
var WithRequiredNextArg *RequiredNextArg
var WithOptionalNextArg *OptionalNextArg
var WithVariadicYield *VariadicYield
var PointerToSlice *Slice
var PointerToPointer *Pointer
var RecursivePointer Recursive
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	sdk, err := new(types.Config).Check("fixture", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	item := sdk.Scope().Lookup("Item").Type()
	index := types.Typ[types.Int]
	text := types.Typ[types.String]

	for _, tt := range []struct {
		operand    string
		key, value types.Type
		enum       string
		next       int
	}{
		{"Plain", index, item, "", 0},
		{"Defined", index, item, "", 0},
		{"DefinedPointer", index, item, "", 0},
		{"Alias", index, item, "", 0},
		{"PointerToAlias", index, item, "", 0},
		{"Instantiated", index, item, "", 0},
		{"WithIterator", text, nil, "XGo_Enum", 0},
		{"DefinedPointerWithIterator", index, item, "", 0},
		{"AliasedPointerWithIterator", text, nil, "XGo_Enum", 0},
		{"WithLegacyIterator", text, item, "Gop_Enum", 3},
		{"WithAliasedSeq", text, nil, "XGo_Enum", 0},
		{"WithAliasedIterator", text, item, "Gop_Enum", 3},
		{"WithAliasedYieldResult", text, nil, "XGo_Enum", 0},
		{"WithAliasedNextResult", text, item, "Gop_Enum", 3},
		{"WithInvalidParams", index, item, "", 0},
		{"WithInvalidYield", index, item, "", 0},
		{"WithInvalidNext", index, item, "", 0},
		{"WithRequiredNextArg", index, item, "", 0},
		{"WithOptionalNextArg", text, nil, "XGo_Enum", 2},
		{"WithVariadicYield", index, item, "", 0},
		{"PointerToSlice", nil, nil, "", 0},
		{"PointerToPointer", nil, nil, "", 0},
		{"RecursivePointer", nil, nil, "", 0},
	} {
		t.Run(tt.operand, func(t *testing.T) {
			var stmt forRangeStmt
			got := stmt.getKeyValTypes(&CodeBuilder{}, sdk.Scope().Lookup(tt.operand).Type())
			if tt.key == nil {
				if got != nil {
					t.Fatalf("iteration types = %v, want nil", got)
				}
			} else if len(got) != 2 || !types.Identical(got[0], tt.key) || !types.Identical(got[1], tt.value) {
				t.Fatalf("iteration types = %v, want [%v %v]", got, tt.key, tt.value)
			}
			if stmt.enumName != tt.enum || stmt.udt != tt.next {
				t.Fatalf("iterator = (%q, %d), want (%q, %d)", stmt.enumName, stmt.udt, tt.enum, tt.next)
			}
		})
	}
}

func TestForRangeEndAfterError(t *testing.T) {
	pkg := NewPackage("fixture", "fixture", nil)
	cb := pkg.CB()
	scope := cb.Scope()
	array := types.NewArray(types.Typ[types.Int], 2)
	typ := types.NewPointer(types.NewPointer(array))
	cb.ForRange("value").Val(types.NewVar(token.NoPos, pkg.Types, "values", typ))

	func() {
		defer func() {
			if _, ok := recover().(*CodeError); !ok {
				t.Fatal("want a CodeError for ranging over a pointer to an array pointer")
			}
		}()
		cb.RangeAssignThen(token.NoPos)
	}()

	cb.End()
	if got := cb.Scope(); got != scope {
		t.Fatal("ending a rejected range did not restore the enclosing scope")
	}
}
