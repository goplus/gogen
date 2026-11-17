/*
Copyright 2026 The XGo Authors (xgo.dev)
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

package gogen_test

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/goplus/gogen"
	"github.com/goplus/gogen/packages"
)

// aliasType is a test helper mirroring the removed Package.AliasType: it
// declares a package-level type alias using the current API
// (NewType(...).AliasType(...)). It lives in this build-tag-agnostic file so
// both the default and genjs test builds can use it.
func aliasType(pkg *gogen.Package, name string, typ types.Type, src ...ast.Node) types.Type {
	return pkg.NewTypeDefs().NewType(name, nil, src...).AliasType(pkg, typ)
}

var (
	gblFset   *token.FileSet
	gblImp    types.Importer
	handleErr func(err error)
)

func init() {
	gogen.SetDebug(gogen.DbgFlagAll)
	gblFset = token.NewFileSet()
	gblImp = packages.NewImporter(gblFset)
}

func ctxRef(pkg *gogen.Package, name string) gogen.Ref {
	_, o := pkg.CB().Scope().LookupParent(name, token.NoPos)
	return o
}

type eventRecorder struct{}

func (p eventRecorder) Member(id ast.Node, obj types.Object) {}
func (p eventRecorder) Call(fn ast.Node, obj types.Object)   {}

type txtNode struct {
	Msg string
	pos token.Pos
}

func (p *txtNode) Pos() token.Pos {
	return p.pos
}

func (p *txtNode) End() token.Pos {
	return p.pos + token.Pos(len(p.Msg))
}

var (
	mutex         sync.Mutex
	pos2Positions = map[token.Pos]token.Position{}
)

// text, line, column
func source(text string, args ...interface{}) ast.Node {
	if len(args) < 2 {
		return &txtNode{Msg: text}
	}
	pos := position(args[0].(int), args[1].(int))
	return &txtNode{Msg: text, pos: pos}
}

const (
	posBits = 16
	posMask = (1 << posBits) - 1
)

func position(line, column int) token.Pos {
	mutex.Lock()
	defer mutex.Unlock()

	pos := token.Pos(len(pos2Positions)+1) << posBits
	pos2Positions[pos] = token.Position{Filename: "./foo.gop", Line: line, Column: column}
	return pos
}

type nodeInterp struct{}

func (p nodeInterp) Position(pos token.Pos) (ret token.Position) {
	mutex.Lock()
	defer mutex.Unlock()
	ret = pos2Positions[(pos &^ posMask)]
	ret.Column += int(pos & posMask)
	return
}

func (p nodeInterp) Caller(node ast.Node) string {
	t := node.(*txtNode)
	idx := strings.Index(t.Msg, "(")
	if idx > 0 {
		return t.Msg[:idx]
	}
	return t.Msg
}

func (p nodeInterp) LoadExpr(node ast.Node) string {
	t := node.(*txtNode)
	return t.Msg
}

func newParam(pkg *gogen.Package, pos token.Pos, name string, typ types.Type) *types.Var {
	return types.NewParam(pos, pkg.Types, name, typ)
}

func newMainPackage(
	implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	return newPackage("main", implicitCast...)
}

func newPackage(
	name string, implicitCast ...func(pkg *gogen.Package, V, T types.Type, pv *gogen.Element) bool) *gogen.Package {
	conf := &gogen.Config{
		Fset:            gblFset,
		Importer:        gblImp,
		Recorder:        eventRecorder{},
		NodeInterpreter: nodeInterp{},
		DbgPositioner:   nodeInterp{},
	}
	if len(implicitCast) > 0 {
		conf.CanImplicitCast = implicitCast[0]
	}
	if handleErr != nil {
		conf.HandleErr = handleErr
		handleErr = nil
	}
	return gogen.NewPackage("", name, conf)
}

func domTest(t *testing.T, pkg *gogen.Package, expected string) {
	domTestEx(t, pkg, expected, "")
}

func domTestEx(t *testing.T, pkg *gogen.Package, expected string, fname string) {
	doDomTest(t, pkg, expected, fname)
	log.Printf("====================== %s End =========================\n", t.Name())
}

func doDomTest(t *testing.T, pkg *gogen.Package, expected string, fname string) {
	var b bytes.Buffer
	t.Helper()
	err := gogen.WriteTo(&b, pkg, fname)
	if err != nil {
		t.Fatal("gogen.WriteTo failed:", err)
	}
	result := b.String()
	if result != expected {
		t.Fatalf("\ngogen.WriteTo Result:\n%s\nExpected:\n%s\n", result, expected)
	}
}

func TestCodeBuilderCallStringConversion(t *testing.T) {
	for _, tt := range []struct {
		name    string
		literal string
		want    string
	}{
		{"ASCII", "65", "A"},
		{"Null", "0", "\x00"},
		{"ASCIIBoundary", "0x7f", "\x7f"},
		{"TwoBytes", "0x80", "\u0080"},
		{"ThreeBytes", "0x266c", "\u266c"},
		{"FourBytes", "0x1f30e", "\U0001f30e"},
		{"BeforeSurrogates", "0xd7ff", "\ud7ff"},
		{"FirstSurrogate", "0xd800", "\ufffd"},
		{"LastSurrogate", "0xdfff", "\ufffd"},
		{"AfterSurrogates", "0xe000", "\ue000"},
		{"ReplacementCharacter", "0xfffd", "\ufffd"},
		{"MaximumRune", "0x10ffff", "\U0010ffff"},
		{"AboveMaximumRune", "0x110000", "\ufffd"},
		{"Negative", "-1", "\ufffd"},
		{"BeyondRuneRange", "0x100000041", "\ufffd"},
		{"BeyondInt64", "0x8000000000000000", "\ufffd"},
		{"MaximumUint64", "0xffffffffffffffff", "\ufffd"},
		{"BeyondUint64", "0x10000000000000041", "\ufffd"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := newMainPackage()
			value := constant.MakeFromLiteral(tt.literal, token.INT, 0)
			operand := types.NewConst(token.NoPos, pkg.Types, "codepoint", types.Typ[types.UntypedInt], value)
			cb := pkg.CB().Typ(types.Typ[types.String]).Val(operand)
			arg := cb.Get(-1)
			cb.Call(1)
			result := cb.Get(-1)
			if result.Type != types.Typ[types.String] {
				t.Fatalf("type = %v, want string", result.Type)
			}
			if result.CVal == nil || result.CVal.Kind() != constant.String {
				t.Fatalf("constant value = %v, want a string", result.CVal)
			}
			if got := constant.StringVal(result.CVal); got != tt.want {
				t.Fatalf("string(%s) = %q, want %q", tt.literal, got, tt.want)
			}
			if arg.Type != operand.Type() || arg.CVal != value {
				t.Fatal("conversion changed the operand")
			}
		})
	}
}

func TestCodeBuilderCallStringConversionValues(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  types.Type
		cval constant.Value
	}{
		{"UntypedRune", types.Typ[types.UntypedRune], constant.MakeInt64(65)},
		{"TypedInteger", types.Typ[types.Int32], constant.MakeInt64(65)},
		{"StringConstant", types.Typ[types.UntypedString], constant.MakeString("ab\x00c")},
		{"TypedStringConstant", types.Typ[types.String], constant.MakeString("ab\x00c")},
		{"IntegerVariable", types.Typ[types.Int], nil},
		{"StringVariable", types.Typ[types.String], nil},
		{"ByteSlice", types.NewSlice(types.Typ[types.Byte]), nil},
		{"RuneSlice", types.NewSlice(types.Typ[types.Rune]), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := newMainPackage()
			var operand types.Object
			if tt.cval == nil {
				operand = types.NewVar(token.NoPos, pkg.Types, "value", tt.typ)
			} else {
				operand = types.NewConst(token.NoPos, pkg.Types, "value", tt.typ, tt.cval)
			}
			cb := pkg.CB().Typ(types.Typ[types.String]).Val(operand).Call(1)
			result := cb.Get(-1)
			if result.Type != types.Typ[types.String] {
				t.Fatalf("type = %v, want string", result.Type)
			}
			if tt.cval == nil {
				if result.CVal != nil {
					t.Fatalf("constant value = %v, want nil", result.CVal)
				}
			} else {
				want := "A"
				if tt.cval.Kind() == constant.String {
					want = constant.StringVal(tt.cval)
				}
				if result.CVal == nil || result.CVal.Kind() != constant.String || constant.StringVal(result.CVal) != want {
					t.Fatalf("constant value = %v, want %q", result.CVal, want)
				}
			}
		})
	}
}

func TestCodeBuilderCallConstantConversion(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		typ   *types.Basic
	}{
		{"IntegralFloatToInteger", "65.0", types.Typ[types.Int]},
		{"ComplexToInteger", "65 + 0i", types.Typ[types.Int]},
		{"SignedInteger", "-128", types.Typ[types.Int8]},
		{"UnsignedInteger", "255", types.Typ[types.Uint8]},
		{"Uint64Maximum", "0xffffffffffffffff", types.Typ[types.Uint64]},
		{"IntegerToFloat32", "16777217", types.Typ[types.Float32]},
		{"IntegerToFloat64", "9007199254740993", types.Typ[types.Float64]},
		{"Float32Rounding", "0.49999999", types.Typ[types.Float32]},
		{"Float32Underflow", "-1e-1000", types.Typ[types.Float32]},
		{"Float64Rounding", "1.1", types.Typ[types.Float64]},
		{"Float64Underflow", "-1e-1000", types.Typ[types.Float64]},
		{"ComplexToFloat", "65 + 0i", types.Typ[types.Float32]},
		{"IntegerToComplex64", "16777217", types.Typ[types.Complex64]},
		{"IntegerToComplex128", "9007199254740993", types.Typ[types.Complex128]},
		{"Complex64Rounding", "0.49999999 + 16777217i", types.Typ[types.Complex64]},
		{"Complex128Rounding", "1.1 + 2.2i", types.Typ[types.Complex128]},
		{"TypedComplexToInteger", "complex128(65)", types.Typ[types.Int]},
		{"TypedComplexToFloat", "complex128(65)", types.Typ[types.Float64]},
		{"Boolean", "true", types.Typ[types.Bool]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			input, err := types.Eval(fset, nil, token.NoPos, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			want, err := types.Eval(fset, nil, token.NoPos, tt.typ.Name()+"("+tt.input+")")
			if err != nil {
				t.Fatal(err)
			}
			pkg := newMainPackage()
			operand := types.NewConst(token.NoPos, pkg.Types, "value", input.Type, input.Value)
			cb := pkg.CB().Typ(tt.typ).Val(operand).Call(1)
			got := cb.Get(-1)
			if got.Type != tt.typ {
				t.Fatalf("type = %v, want %v", got.Type, tt.typ)
			}
			if got.CVal == nil || got.CVal.Kind() != want.Value.Kind() || !constant.Compare(got.CVal, token.EQL, want.Value) {
				t.Fatalf("constant value = %v, want %v", got.CVal, want.Value)
			}
		})
	}
}

func TestCodeBuilderCallChainedStringConversion(t *testing.T) {
	pkg := newMainPackage()
	cb := pkg.CB().Typ(types.Typ[types.String]).Typ(types.Typ[types.Int]).Typ(types.Typ[types.Float32]).
		Val(&ast.BasicLit{Kind: token.FLOAT, Value: "65.0000001"}).Call(1).Call(1).Call(1)
	result := cb.Get(-1)
	if result.Type != types.Typ[types.String] {
		t.Fatalf("type = %v, want string", result.Type)
	}
	if result.CVal == nil || result.CVal.Kind() != constant.String || constant.StringVal(result.CVal) != "A" {
		t.Fatalf("constant value = %v, want A", result.CVal)
	}
}

func TestCodeBuilderCallConstantConversionErrors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		typ   *types.Basic
	}{
		{"IntegerOverflow", "256", types.Typ[types.Uint8]},
		{"SignedIntegerOverflow", "321", types.Typ[types.Int8]},
		{"IntegerUnderflow", "-129", types.Typ[types.Int8]},
		{"NegativeToUnsigned", "-1", types.Typ[types.Uint64]},
		{"FractionToInteger", "1.5", types.Typ[types.Int]},
		{"ComplexToInteger", "1 + 1i", types.Typ[types.Int]},
		{"ComplexToFloat", "1 + 1i", types.Typ[types.Float64]},
		{"FloatToString", "65.0", types.Typ[types.String]},
		{"IntegerToBool", "1", types.Typ[types.Bool]},
		{"Float32Overflow", "1e40", types.Typ[types.Float32]},
		{"Float64Overflow", "1e400", types.Typ[types.Float64]},
		{"Complex64Overflow", "1e40 + 1i", types.Typ[types.Complex64]},
		{"Complex128Overflow", "1 + 1e400i", types.Typ[types.Complex128]},
		{"StringToComplex", "\"abc\"", types.Typ[types.Complex64]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input, err := types.Eval(token.NewFileSet(), nil, token.NoPos, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			pkg := newMainPackage()
			operand := types.NewConst(token.NoPos, pkg.Types, "value", input.Type, input.Value)
			cb := pkg.CB().Typ(tt.typ).Val(operand, source(tt.input, 1, 5))
			err = cb.CallWithEx(1, 1, 0)
			codeErr, ok := err.(*gogen.CodeError)
			if !ok {
				t.Fatalf("error = %v, want a CodeError", err)
			}
			if got, want := codeErr.Error(), "cannot convert constant "; !strings.Contains(got, want) {
				t.Fatalf("error = %q, want %q", got, want)
			}
			if got := (nodeInterp{}).Position(codeErr.Pos); got.Line != 1 || got.Column != 5 {
				t.Fatalf("error position = %v, want line 1, column 5", got)
			}
			if got := (nodeInterp{}).Position(codeErr.End); got.Line != 1 || got.Column != 5+len(tt.input) {
				t.Fatalf("error end = %v, want line 1, column %d", got, 5+len(tt.input))
			}
		})
	}
}

func TestCodeBuilderCallTypedConstantConversion(t *testing.T) {
	for _, tt := range []struct {
		name string
		from *types.Basic
		to   *types.Basic
	}{
		{"Float32ToInteger", types.Typ[types.Float32], types.Typ[types.Int]},
		{"Float32ToFloat64", types.Typ[types.Float32], types.Typ[types.Float64]},
		{"Complex64ToInteger", types.Typ[types.Complex64], types.Typ[types.Int]},
		{"Complex64ToComplex128", types.Typ[types.Complex64], types.Typ[types.Complex128]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := newMainPackage()
			pkg.NewConstStart(pkg.Types.Scope(), token.NoPos, tt.from, "value").
				Val(&ast.BasicLit{Kind: token.FLOAT, Value: "65.0000001"}).EndInit(1)
			cb := pkg.CB().Typ(tt.to).Val(pkg.Ref("value")).Call(1)
			result := cb.Get(-1)
			if result.Type != tt.to {
				t.Fatalf("type = %v, want %v", result.Type, tt.to)
			}
			want, err := types.Eval(token.NewFileSet(), nil, token.NoPos, tt.to.Name()+"("+tt.from.Name()+"(65.0000001))")
			if err != nil {
				t.Fatal(err)
			}
			if result.CVal == nil || result.CVal.Kind() != want.Value.Kind() || !constant.Compare(result.CVal, token.EQL, want.Value) {
				t.Fatalf("constant value = %v, want %v", result.CVal, want.Value)
			}
		})
	}
}

func emitConstExpr(t *testing.T, pkg *gogen.Package, expr ast.Expr) {
	t.Helper()
	cb := pkg.CB()
	switch e := expr.(type) {
	case *ast.BasicLit:
		cb.Val(e)
	case *ast.Ident:
		obj := ctxRef(pkg, e.Name)
		if obj == nil {
			t.Fatalf("unknown identifier %s", e.Name)
		}
		cb.Val(obj)
	case *ast.CallExpr:
		emitConstExpr(t, pkg, e.Fun)
		for _, arg := range e.Args {
			emitConstExpr(t, pkg, arg)
		}
		cb.Call(len(e.Args))
	case *ast.UnaryExpr:
		emitConstExpr(t, pkg, e.X)
		cb.UnaryOp(e.Op)
	case *ast.BinaryExpr:
		emitConstExpr(t, pkg, e.X)
		emitConstExpr(t, pkg, e.Y)
		cb.BinaryOp(e.Op)
	case *ast.ParenExpr:
		emitConstExpr(t, pkg, e.X)
	default:
		t.Fatalf("unsupported constant expression %T", expr)
	}
}

func TestCodeBuilderConstantExpression(t *testing.T) {
	for _, tt := range []struct{ name, expression string }{
		{"TypedFloat32", "float32(1e8) + float32(0.5) - float32(1e8)"},
		{"TypedFloat64", "float64(1e16) + float64(0.5) - float64(1e16)"},
		{"WidenedInteger", "int8(int(float32(16777216) + float32(1) - float32(16777216) + float32(127)))"},
		{"TypedString", "string(int(float32(65.0000001)))"},
		{"TypedComplex", "complex64(1) + complex64(2)"},
		{"UntypedPrecision", "1e100 + 1 - 1e100"},
		{"UntypedShift", "1 << uint(3)"},
		{"TypedShift", "uint8(1) << 3"},
		{"TypedUnary", "-float32(0.1)"},
		{"TypedComparison", "float32(0.1) == 0.1"},
		{"TypedOrderedComparison", "float32(0.1) < 0.1"},
		{"TypedMixedAddition", "float32(1) + 0.0000000596046478"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			expr, err := parser.ParseExprFrom(fset, "main.go", tt.expression, 0)
			if err != nil {
				t.Fatal(err)
			}
			want, err := types.Eval(fset, nil, token.NoPos, tt.expression)
			if err != nil {
				t.Fatal(err)
			}
			pkg := newMainPackage()
			emitConstExpr(t, pkg, expr)
			got := pkg.CB().Get(-1)
			if !types.Identical(got.Type, want.Type) {
				t.Errorf("type = %v, want %v", got.Type, want.Type)
			}
			if got.CVal == nil || got.CVal.Kind() != want.Value.Kind() || !constant.Compare(got.CVal, token.EQL, want.Value) {
				t.Errorf("value = %v, want %v", got.CVal, want.Value)
			}
		})
	}
}

func TestCodeBuilderConstantExpressionErrors(t *testing.T) {
	for _, tt := range []struct{ name, expression string }{
		{"MixedIntegerOverflow", "int8(1) + 128"},
		{"LeftOperandOverflow", "128 + int8(1)"},
		{"TypedIntegerOverflow", "int8(127) + int8(1)"},
		{"UnaryOverflow", "-int8(-128)"},
		{"ComparisonOverflow", "float32(1) == 1e40"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expression)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				codeErr, ok := recover().(*gogen.CodeError)
				if !ok {
					t.Fatal("want a CodeError")
				}
				if !strings.Contains(codeErr.Msg, "cannot convert constant") {
					t.Fatalf("error = %q, want a constant conversion error", codeErr.Msg)
				}
			}()
			emitConstExpr(t, newMainPackage(), expr)
		})
	}
}

func TestCodeBuilderConstantDivisionOverflow(t *testing.T) {
	testConstantDivisionOverflow(t, func() *gogen.Package { return newMainPackage() })
}

func testConstantDivisionOverflow(t *testing.T, newPackage func() *gogen.Package) {
	t.Helper()
	for _, tt := range []struct {
		name, expression, diagnostic string
	}{
		{"Int8Result", "int8(-128) / -1", "cannot convert constant 128 to type int8"},
		{"Int32Result", "int32(-2147483648) / int32(-1)", "cannot convert constant 2147483648 to type int32"},
		{"Float32Result", "float32(1e38) / 0.1", "cannot convert constant"},
		{"RightOperand", "int8(1) / 128", "cannot convert constant 128 to type int8"},
		{"LeftOperand", "128 / int8(1)", "cannot convert constant 128 to type int8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expression)
			if err != nil {
				t.Fatal(err)
			}
			division, ok := expr.(*ast.BinaryExpr)
			if !ok {
				t.Fatalf("want a binary expression, got %T", expr)
			}
			pkg := newPackage()
			src := source(tt.expression, 2, 5)
			defer func() {
				got := recover()
				codeErr, ok := got.(*gogen.CodeError)
				if !ok {
					t.Fatalf("got %T %v, want a CodeError", got, got)
				}
				if !strings.Contains(codeErr.Msg, tt.diagnostic) {
					t.Errorf("error = %q, want %q", codeErr.Msg, tt.diagnostic)
				}
				if codeErr.Pos != src.Pos() || codeErr.End != src.End() {
					t.Errorf("error span = [%d, %d), want [%d, %d)", codeErr.Pos, codeErr.End, src.Pos(), src.End())
				}
			}()
			cb := pkg.NewConstStart(pkg.Types.Scope(), token.NoPos, nil, "value")
			emitConstExpr(t, pkg, division.X)
			emitConstExpr(t, pkg, division.Y)
			cb.BinaryOp(division.Op, src).EndInit(1)
		})
	}
}

func TestConstDefsTypedValues(t *testing.T) {
	pkg := newMainPackage()
	defs := pkg.NewConstDefs(pkg.Types.Scope()).New(func(cb *gogen.CodeBuilder) int {
		cb.Val(&ast.BasicLit{Kind: token.FLOAT, Value: "65.0000001"})
		return 1
	}, 0, token.NoPos, types.Typ[types.Float32], "first")
	defs.Next(1, token.NoPos, "second")
	for _, name := range []string{"first", "second"} {
		value, ok := pkg.Ref(name).(*types.Const)
		if !ok {
			t.Fatalf("%s is not a constant", name)
		}
		if value.Val().Kind() != constant.Float || !constant.Compare(value.Val(), token.EQL, constant.MakeFloat64(65)) {
			t.Errorf("%s = %v, want 65", name, value.Val())
		}
	}
}

func TestCodeBuilderCallNonConstantConversion(t *testing.T) {
	for _, tt := range []struct {
		name string
		from types.Type
		to   types.Type
		cval constant.Value
	}{
		{"Interface", types.Typ[types.UntypedInt], gogen.TyAny, constant.MakeInt64(1)},
		{"ByteSlice", types.Typ[types.UntypedString], types.NewSlice(types.Typ[types.Byte]), constant.MakeString("abc")},
		{"RuneSlice", types.Typ[types.UntypedString], types.NewSlice(types.Typ[types.Rune]), constant.MakeString("abc")},
		{"UnsafePointer", types.Typ[types.Uintptr], types.Typ[types.UnsafePointer], constant.MakeInt64(0)},
		{"TypeParameter", types.Typ[types.UntypedFloat], types.NewTypeParam(
			types.NewTypeName(token.NoPos, nil, "T", nil),
			types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{
				types.NewTerm(true, types.Typ[types.Float32]),
				types.NewTerm(true, types.Typ[types.Float64]),
			})}).Complete(),
		), constant.MakeFloat64(1.1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkg := newMainPackage()
			operand := types.NewConst(token.NoPos, pkg.Types, "value", tt.from, tt.cval)
			cb := pkg.CB().Typ(tt.to).Val(operand).Call(1)
			result := cb.Get(-1)
			if result.Type != tt.to {
				t.Fatalf("type = %v, want %v", result.Type, tt.to)
			}
			if result.CVal != nil {
				t.Fatalf("constant value = %v, want nil", result.CVal)
			}
		})
	}
}

func TestConstDefsNonConstant(t *testing.T) {
	for _, name := range []string{"First", "Next"} {
		t.Run(name, func(t *testing.T) {
			pkg := newMainPackage()
			var defs *gogen.ConstDefs
			if name == "Next" {
				defs = pkg.NewConstDefs(pkg.Types.Scope()).New(func(cb *gogen.CodeBuilder) int {
					cb.Val("abc")
					return 1
				}, 0, token.NoPos, nil, "first")
			}
			defer func() {
				codeErr, ok := recover().(*gogen.CodeError)
				if !ok {
					t.Fatal("want a CodeError")
				}
				if !strings.Contains(codeErr.Msg, "is not a constant") {
					t.Fatalf("error = %q, want a non-constant initializer error", codeErr.Msg)
				}
			}()
			if name == "First" {
				pkg.NewConstStart(pkg.Types.Scope(), token.NoPos, nil, "first").Typ(gogen.TyAny).Val("abc").Call(1).EndInit(1)
			} else {
				defs.NextAt(defs.NewPos(), func(cb *gogen.CodeBuilder) int {
					cb.Typ(types.NewSlice(types.Typ[types.Byte])).Val("abc").Call(1)
					return 1
				}, 1, token.NoPos, "second")
			}
		})
	}
}

func TestBlankConstInitializer(t *testing.T) {
	for _, tt := range []struct {
		name    string
		typ     types.Type
		literal string
		message string
	}{
		{"Overflow", types.Typ[types.Float32], "1e40", "cannot convert constant"},
		{"NonConstant", gogen.TyAny, "1.5", "is not a constant"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				codeErr, ok := recover().(*gogen.CodeError)
				if !ok {
					t.Fatal("want a CodeError")
				}
				if !strings.Contains(codeErr.Msg, tt.message) {
					t.Fatalf("error = %q, want %q", codeErr.Msg, tt.message)
				}
			}()
			pkg := newMainPackage()
			if tt.name == "NonConstant" {
				pkg.NewConstStart(pkg.Types.Scope(), token.NoPos, nil, "_").Typ(tt.typ).
					Val(&ast.BasicLit{Kind: token.FLOAT, Value: tt.literal}).Call(1).EndInit(1)
			} else {
				pkg.NewConstStart(pkg.Types.Scope(), token.NoPos, tt.typ, "_").
					Val(&ast.BasicLit{Kind: token.FLOAT, Value: tt.literal}).EndInit(1)
			}
		})
	}
}
