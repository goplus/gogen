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
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"runtime"
	"strings"

	"github.com/goplus/gogen/target"
	"github.com/goplus/gogen/typeutil"
)

// ----------------------------------------------------------------------------

var (
	std types.Sizes
)

func init() {
	if runtime.Compiler == "gopherjs" {
		std = &types.StdSizes{WordSize: 4, MaxAlign: 4}
	} else {
		std = types.SizesFor(runtime.Compiler, runtime.GOARCH)
	}
}

func checkArgsCount(pkg *Package, fn string, n int, args int, src ast.Node) {
	if args == n {
		return
	}
	cb := &pkg.cb
	text, pos, end := cb.loadExpr(src)
	if pos != token.NoPos {
		pos += token.Pos(len(fn))
		end += token.Pos(len(fn))
	}
	if args < n {
		cb.panicCodeErrorf(pos, end, "missing argument to function call: %v", text)
	}
	cb.panicCodeErrorf(pos, end, "too many arguments to function call: %v", text)
}

func unsafeRef(name string) Ref {
	return PkgRef{types.Unsafe}.Ref(name)
}

type unsafeSizeofInstr struct{}

// func unsafe.Sizeof(x ArbitraryType) uintptr
func (p unsafeSizeofInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Sizeof", 1, len(args), src)

	typ := types.Default(realType(args[0].Type))
	ret = &Element{
		Val:  newSizeofExpr(pkg, args),
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(std.Sizeof(typ)),
		Src:  src,
	}
	return
}

type unsafeAlignofInstr struct{}

// func unsafe.Alignof(x ArbitraryType) uintptr
func (p unsafeAlignofInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Alignof", 1, len(args), src)

	typ := types.Default(realType(args[0].Type))
	ret = &Element{
		Val:  newAlignofExpr(pkg, args),
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(std.Alignof(typ)),
		Src:  src,
	}
	return
}

type unsafeOffsetofInstr struct{}

// func unsafe.Offsetof(x ArbitraryType) uintptr
func (p unsafeOffsetofInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	checkArgsCount(pkg, "unsafe.Offsetof", 1, len(args), src)

	var sel *target.SelectorExpr
	var ok bool
	if sel, ok = args[0].Val.(*target.SelectorExpr); !ok {
		s, pos, end := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
			end += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, end, "invalid expression %v", s)
	}
	if _, ok = args[0].Type.(*types.Signature); ok {
		s, pos, end := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
			end += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, end, "invalid expression %v: argument is a method value", s)
	}
	recv := denoteRecv(sel)
	typ := getStruct(pkg, recv.Type)
	_, index, _ := types.LookupFieldOrMethod(typ, false, pkg.Types, sel.Sel.Name)
	offset, err := offsetof(pkg, typ, index, recv.Src, sel.Sel.Name)
	if err != nil {
		_, pos, end := pkg.cb.loadExpr(src)
		if pos != token.NoPos {
			pos += token.Pos(len("unsafe.Offsetof"))
			end += token.Pos(len("unsafe.Offsetof"))
		}
		pkg.cb.panicCodeErrorf(pos, end, "%v", err)
	}
	//var offset int64
	ret = &Element{
		Val:  newOffsetofExpr(pkg, args),
		Type: types.Typ[types.Uintptr],
		CVal: constant.MakeInt64(offset),
		Src:  src,
	}
	return
}

func getStruct(pkg *Package, typ types.Type) *types.Struct {
retry:
	switch t := typ.(type) {
	case *types.Struct:
		return t
	case *types.Pointer:
		typ = t.Elem()
		goto retry
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return nil
}

func offsetsof(T *types.Struct) []int64 {
	var fields []*types.Var
	for i := 0; i < T.NumFields(); i++ {
		fields = append(fields, T.Field(i))
	}
	return std.Offsetsof(fields)
}

// offsetof returns the offset of the field specified via
// the index sequence relative to typ. All embedded fields
// must be structs (rather than pointer to structs).
func offsetof(pkg *Package, typ types.Type, index []int, recv ast.Node, sel string) (int64, error) {
	var o int64
	var typList []string
	var indirectType int
	for n, i := range index {
		if n > 0 {
			if t, ok := types.Unalias(typ).(*types.Pointer); ok {
				typ = t.Elem()
				indirectType = n
			}
			if t, ok := types.Unalias(typ).(*types.Named); ok {
				typList = append(typList, t.Obj().Name())
				typ = t.Underlying()
			}
		}
		s := typ.(*types.Struct)
		o += offsetsof(s)[i]
		typ = s.Field(i).Type()
	}
	if indirectType > 0 {
		s, _, _ := pkg.cb.loadExpr(recv)
		return -1, fmt.Errorf("invalid expression unsafe.Offsetof(%v.%v.%v): selector implies indirection of embedded %v.%v",
			s, strings.Join(typList, "."), sel,
			s, strings.Join(typList[:indirectType], "."))
	}
	return o, nil
}

type unsafeAddInstr struct{}

// func unsafe.Add(ptr Pointer, len IntegerType) Pointer
func (p unsafeAddInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	const (
		tokenLen = token.Pos(len("unsafe.Add"))
	)
	checkArgsCount(pkg, "unsafe.Add", 2, len(args), src)

	if ts := args[0].Type.String(); ts != "unsafe.Pointer" {
		s, _, _ := pkg.cb.loadExpr(args[0].Src)
		pos := getSrcPos(src)
		end := getSrcEnd(src)
		if pos != token.NoPos {
			pos += tokenLen
			end += tokenLen
		}
		pkg.cb.panicCodeErrorf(pos, end, "cannot use %v (type %v) as type unsafe.Pointer in argument to unsafe.Add", s, ts)
	}
	if t := args[1].Type; !ninteger.Match(pkg, t) {
		s, _, _ := pkg.cb.loadExpr(args[1].Src)
		pos := getSrcPos(src)
		end := getSrcEnd(src)
		if pos != token.NoPos {
			pos += tokenLen
			end += tokenLen
		}
		pkg.cb.panicCodeErrorf(pos, end, "cannot use %v (type %v) as type int", s, t)
	}
	ret = &Element{
		Val:  newUnsafeAddExpr(pkg, args),
		Type: types.Typ[types.UnsafePointer],
	}
	return
}

type unsafeDataInstr struct {
	name string
	args int
}

// Go1.17+
// func unsafe.Slice(ptr *ArbitraryType, len IntegerType) []ArbitraryType
// Go1.20+
// func unsafe.String(ptr *byte, len IntegerType) string
// func unsafe.SliceData(slice []ArbitraryType) *ArbitraryType
// func unsafe.StringData(str string) *byte
func (unsafeDataInstr) checkFirstType(pkg *Package, fname string, arg *Element, src ast.Node) (ret types.Type, err error) {
	var info string
	switch fname {
	case "unsafe.Slice":
		if t, ok := arg.Type.(*types.Pointer); ok {
			return types.NewSlice(t.Elem()), nil
		}
		info = "pointer"
	case "unsafe.String":
		if t, ok := arg.Type.(*types.Pointer); ok && isBasicType(t.Elem(), types.Byte) {
			return types.Typ[types.String], nil
		}
		info = "*byte"
	case "unsafe.SliceData":
		if t, ok := arg.Type.Underlying().(*types.Slice); ok {
			return types.NewPointer(t.Elem()), nil
		}
		info = "slice"
	case "unsafe.StringData":
		if isBasicType(arg.Type, types.String) {
			return types.NewPointer(types.Typ[types.Byte]), nil
		}
		info = "string"
	}
	pos := getSrcPos(src)
	end := getSrcEnd(src)
	if pos != token.NoPos {
		pos += token.Pos(len(fname))
		end += token.Pos(len(fname))
	}
	return nil, pkg.cb.newCodeErrorf(pos, end, "first argument to %v must be %v; have %v", fname, info, arg.Type)
}

func (p unsafeDataInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	fname := "unsafe." + p.name
	checkArgsCount(pkg, fname, p.args, len(args), src)

	typ, err := p.checkFirstType(pkg, fname, args[0], src)
	if err != nil {
		return nil, err
	}
	if p.args == 2 {
		if t := args[1].Type; !ninteger.Match(pkg, t) {
			pos := getSrcPos(src)
			end := getSrcEnd(src)
			if pos != token.NoPos {
				pos += token.Pos(len(fname))
				end += token.Pos(len(fname))
			}
			return nil, pkg.cb.newCodeErrorf(pos, end, "non-integer len argument in %v - %v", fname, t)
		}
	}
	ret = &Element{
		Val:  newUnsafeDataExpr(p, pkg, args),
		Type: typ,
	}
	return
}

func isBasicType(typ types.Type, kind types.BasicKind) bool {
	if t, ok := typ.(*types.Basic); ok && t.Kind() == kind {
		return true
	}
	return false
}

// ----------------------------------------------------------------------------

type builtinFn struct {
	fn   any
	narg int
}

var (
	builtinFns = map[string]builtinFn{
		"complex": {makeComplex, 2},
		"real":    {constant.Real, 1},
		"imag":    {constant.Imag, 1},
		"min":     {minConst, -1},
		"max":     {maxConst, -1},
	}
)

// tryBuiltinCall attempts constant folding for builtin functions.
// If canfail is false and fn is an identifier but not a known builtin, it panics.
// If canfail is true, it returns nil for unknown or non-identifier functions.
func tryBuiltinCall(fn *Element, args []*Element, canfail bool) constant.Value {
	ident, ok := fn.Val.(*target.Ident)
	if !ok {
		return nil
	}
	if bfn, ok := builtinFns[ident.Name]; ok {
		return doBuiltinCall(bfn, args)
	}
	if !canfail {
		panic("builtinCall: unknown function")
	}
	return nil
}

func doBuiltinCall(bfn builtinFn, args []*Element) constant.Value {
	switch bfn.narg {
	case 1:
		a := args[0].CVal
		return bfn.fn.(func(a constant.Value) constant.Value)(a)
	case 2:
		a := args[0].CVal
		b := args[1].CVal
		return bfn.fn.(func(a, b constant.Value) constant.Value)(a, b)
	case -1: // variadic (min/max)
		if len(args) == 0 {
			return nil // let matchFuncType report proper error
		}
		cvals := make([]constant.Value, len(args))
		for i, arg := range args {
			if arg.CVal == nil {
				return nil
			}
			cvals[i] = arg.CVal
		}
		return bfn.fn.(func([]constant.Value) constant.Value)(cvals)
	}
	panic("builtinCall: unexpected narg")
}

func makeComplex(re, im constant.Value) constant.Value {
	return constant.BinaryOp(re, token.ADD, constant.MakeImag(im))
}

// isOrderable reports whether a constant value is orderable (can be compared with < >).
// Complex and boolean values are not orderable.
func isOrderable(v constant.Value) bool {
	switch v.Kind() {
	case constant.Int, constant.Float, constant.String:
		return true
	default:
		return false
	}
}

// minMaxConst performs constant folding for min/max builtins. It returns nil if
// any argument is not orderable, letting the type checker report the proper error.
func minMaxConst(args []constant.Value, op token.Token) constant.Value {
	result := args[0]
	if !isOrderable(result) {
		return nil // let type checker report proper error
	}
	for _, v := range args[1:] {
		if !isOrderable(v) {
			return nil
		}
		if constant.Compare(v, op, result) {
			result = v
		}
	}
	return result
}

func minConst(args []constant.Value) constant.Value {
	return minMaxConst(args, token.LSS)
}

func maxConst(args []constant.Value) constant.Value {
	return minMaxConst(args, token.GTR)
}

// ----------------------------------------------------------------------------

type basicContract struct {
	kinds uint64
	desc  string
}

func (p *basicContract) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		if (uint64(1)<<t.Kind())&p.kinds != 0 {
			return true
		}
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return false
}

func (p *basicContract) String() string {
	return p.desc
}

// ----------------------------------------------------------------------------

type comparableT struct {
	// addable
	// type bool, interface, pointer, array, chan, struct
	// NOTE: slice/map/func is very special, can only be compared to nil
}

func (p comparableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		return t.Kind() != types.UntypedNil // excluding nil
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	case *types.Slice: // slice/map/func is very special
		return false
	case *types.Map:
		return false
	case *types.Signature:
		return false
	case *TemplateSignature:
		return false
	case *TemplateParamType:
		panic("TODO: unexpected - compare to template param type?")
	case *types.Tuple:
		panic("TODO: unexpected - compare to tuple type?")
	case *unboundType:
		panic("TODO: unexpected - compare to unboundType?")
	case *unboundFuncParam:
		panic("TODO: unexpected - compare to unboundFuncParam?")
	}
	return true
}

func (p comparableT) String() string {
	return "comparable"
}

// ----------------------------------------------------------------------------

type anyT struct {
}

func (p anyT) Match(pkg *Package, typ types.Type) bool {
	return true
}

func (p anyT) String() string {
	return "any"
}

// ----------------------------------------------------------------------------

type capableT struct {
	// type slice, chan, array, array_pointer
}

func (p capableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Chan:
		return true
	case *types.Array:
		return true
	case *types.Pointer:
		_, ok := t.Elem().(*types.Array) // array_pointer
		return ok
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return false
}

func (p capableT) String() string {
	return "capable"
}

// ----------------------------------------------------------------------------

type lenableT struct {
	// capable
	// type map, string
}

func (p lenableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Basic:
		k := t.Kind()
		return k == types.String || k == types.UntypedString
	case *types.Map:
		return true
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return capable.Match(pkg, typ)
}

func (p lenableT) String() string {
	return "lenable"
}

// ----------------------------------------------------------------------------

type makableT struct {
	// type slice, chan, map
}

func (p makableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Chan:
		return true
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return false
}

func (p makableT) String() string {
	return "makable"
}

// ----------------------------------------------------------------------------

type addableT struct {
	// type basicContract{kindsAddable}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p addableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		default:
			// TODO(xsw): refactor
			cb := pkg.cb
			cb.stk.Push(elemNone)
			kind := cb.findMember(typ, "XGo_Add", "", 0, MemberFlagVal, &Element{}, nil, nil)
			if kind != 0 {
				cb.stk.PopN(1)
				if kind == MemberMethod {
					return true
				}
			}
		}
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	c := &basicContract{kinds: kindsAddable}
	return c.Match(pkg, typ)
}

func (p addableT) String() string {
	return "addable"
}

// ----------------------------------------------------------------------------

type numberT struct {
	// type basicContract{kindsNumber}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p numberT) Match(pkg *Package, typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		}
	}
	c := &basicContract{kinds: kindsNumber}
	return c.Match(pkg, typ)
}

func (p numberT) String() string {
	return "number"
}

// ----------------------------------------------------------------------------

type orderableT struct {
	// type basicContract{kindsOrderable}, untyped_bigint, untyped_bigrat, untyped_bigfloat
}

func (p orderableT) Match(pkg *Package, typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		switch t {
		case pkg.utBigInt, pkg.utBigRat, pkg.utBigFlt:
			return true
		}
	}
	c := &basicContract{kinds: kindsOrderable}
	return c.Match(pkg, typ)
}

func (p orderableT) String() string {
	return "orderable"
}

// ----------------------------------------------------------------------------

type integerT struct {
	// type basicContract{kindsNumber}, untyped_bigint
}

func (p integerT) Match(pkg *Package, typ types.Type) bool {
	c := &basicContract{kinds: kindsNumber}
	if c.Match(pkg, typ) {
		return true
	}
	return typ == pkg.utBigInt
}

func (p integerT) String() string {
	return "integer"
}

type clearableT struct {
	// type slice, map
}

func (p clearableT) Match(pkg *Package, typ types.Type) bool {
retry:
	switch t := typ.(type) {
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Named:
		typ = pkg.cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return false
}

func (p clearableT) String() string {
	return "clearable"
}

// ----------------------------------------------------------------------------

var (
	_any       = anyT{}
	capable    = capableT{}
	lenable    = lenableT{}
	makable    = makableT{}
	_bool      = &basicContract{kindsBool, "bool"}
	_string    = &basicContract{kindsString, "string"}
	ninteger   = &basicContract{kindsInteger, "ninteger"}
	integer    = integerT{}
	number     = numberT{}
	orderable  = orderableT{}
	addable    = addableT{}
	comparable = comparableT{}
	clearable  = clearableT{}
	borderable = &basicContract{kindsOrderable, "orderable"}
)

// ----------------------------------------------------------------------------

type incInstr struct {
}

type decInstr struct {
}

// val++
func (p incInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	return callIncDec(pkg, args, token.INC)
}

// val--
func (p decInstr) Call(pkg *Package, args []*Element, lhs int, flags InstrFlags, src ast.Node) (ret *Element, err error) {
	return callIncDec(pkg, args, token.DEC)
}

func callIncDec(pkg *Package, args []*Element, tok token.Token) (ret *Element, err error) {
	if len(args) != 1 {
		panic("TODO: please use val" + tok.String())
	}
	t, ok := args[0].Type.(*refType)
	if !ok {
		panic("TODO: not addressable")
	}
	cb := &pkg.cb
	if !isNumeric(cb, t.typ) {
		text, pos, end := cb.loadExpr(args[0].Src)
		cb.panicCodeErrorf(pos, end, "invalid operation: %s%v (non-numeric type %v)", text, tok, t.typ)
	}
	cb.emitStmt(newIncDecStmt(args[0].Val, tok))
	return
}

func isNumeric(cb *CodeBuilder, typ types.Type) bool {
	const (
		numericFlags = types.IsInteger | types.IsFloat | types.IsComplex
	)
retry:
	switch t := typ.(type) {
	case *types.Basic:
		return (t.Info() & numericFlags) != 0
	case *types.Named:
		typ = cb.getUnderlying(t)
		goto retry
	case *types.Alias:
		typ = types.Unalias(t)
		goto retry
	}
	return false
}

// ----------------------------------------------------------------------------

type bmExargs = []any

type BuiltinMethod struct {
	Name   string
	Fn     types.Object
	Exargs []any
}

func (p *BuiltinMethod) Results() *types.Tuple {
	return p.Fn.Type().(*types.Signature).Results()
}

func (p *BuiltinMethod) Params() *types.Tuple {
	params := p.Fn.Type().(*types.Signature).Params()
	n := params.Len() - len(p.Exargs) - 1
	if n <= 0 {
		return nil
	}
	ret := make([]*types.Var, n)
	for i := 0; i < n; i++ {
		ret[i] = params.At(i + 1)
	}
	return types.NewTuple(ret...)
}

type mthdSignature interface {
	Results() *types.Tuple
	Params() *types.Tuple
}

type BuiltinTI struct {
	typ     types.Type
	methods []*BuiltinMethod
}

func (p *BuiltinTI) AddMethods(mthds ...*BuiltinMethod) {
	p.methods = append(p.methods, mthds...)
}

func (p *BuiltinTI) numMethods() int {
	return len(p.methods)
}

func (p *BuiltinTI) method(i int) *BuiltinMethod {
	return p.methods[i]
}

func (p *BuiltinTI) lookupByName(name string) mthdSignature {
	for i, n := 0, p.numMethods(); i < n; i++ {
		method := p.method(i)
		if method.Name == name {
			return method
		}
	}
	return nil
}

var (
	tyChan  types.Type = types.NewChan(0, types.Typ[types.Invalid])
	tySlice types.Type = types.NewSlice(types.Typ[types.Invalid])
)

func initBuiltinTIs(pkg *Package) {
	var (
		float64TI, intTI, int64TI, uint64TI *BuiltinTI
		osxTI, stringTI, stringSliceTI      *BuiltinTI
	)
	btiMap := new(typeutil.Map)
	strconv := pkg.TryImport("strconv")
	strings := pkg.TryImport("strings")
	btoLen := types.Universe.Lookup("len")
	btoCap := types.Universe.Lookup("cap")
	{
		osxPkg := pkg.conf.PkgPathOsx
		if debugImportOsx && osxPkg == "" {
			osxPkg = "github.com/goplus/gogen/internal/iox"
		}
		if osxPkg != "" {
			if os := pkg.TryImport("os"); os.isValid() {
				if osx := pkg.TryImport(osxPkg); osx.isValid() {
					osxTI = &BuiltinTI{
						typ: os.Ref("File").Type(),
						methods: []*BuiltinMethod{
							{"XGo_Enum", osx.Ref("EnumLines"), nil},
						},
					}
				}
			}
		}
	}
	if strconv.isValid() {
		float64TI = &BuiltinTI{
			typ: types.Typ[types.Float64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatFloat"), bmExargs{'g', -1, 64}},
			},
		}
		intTI = &BuiltinTI{
			typ: types.Typ[types.Int],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("Itoa"), nil},
			},
		}
		int64TI = &BuiltinTI{
			typ: types.Typ[types.Int64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatInt"), bmExargs{10}},
			},
		}
		uint64TI = &BuiltinTI{
			typ: types.Typ[types.Uint64],
			methods: []*BuiltinMethod{
				{"String", strconv.Ref("FormatUint"), bmExargs{10}},
			},
		}
	}
	if strings.isValid() && strconv.isValid() {
		stringTI = &BuiltinTI{
			typ: types.Typ[types.String],
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Count", strings.Ref("Count"), nil},
				{"Int", strconv.Ref("Atoi"), nil},
				{"Int64", strconv.Ref("ParseInt"), bmExargs{10, 64}},
				{"Uint64", strconv.Ref("ParseUint"), bmExargs{10, 64}},
				{"Float", strconv.Ref("ParseFloat"), bmExargs{64}},
				{"Index", strings.Ref("Index"), nil},
				{"IndexAny", strings.Ref("IndexAny"), nil},
				{"IndexByte", strings.Ref("IndexByte"), nil},
				{"IndexRune", strings.Ref("IndexRune"), nil},
				{"LastIndex", strings.Ref("LastIndex"), nil},
				{"LastIndexAny", strings.Ref("LastIndexAny"), nil},
				{"LastIndexByte", strings.Ref("LastIndexByte"), nil},
				{"Contains", strings.Ref("Contains"), nil},
				{"ContainsAny", strings.Ref("ContainsAny"), nil},
				{"ContainsRune", strings.Ref("ContainsRune"), nil},
				{"Compare", strings.Ref("Compare"), nil},
				{"EqualFold", strings.Ref("EqualFold"), nil},
				{"HasPrefix", strings.Ref("HasPrefix"), nil},
				{"HasSuffix", strings.Ref("HasSuffix"), nil},
				{"Quote", strconv.Ref("Quote"), nil},
				{"Unquote", strconv.Ref("Unquote"), nil},
				{"ToTitle", strings.Ref("ToTitle"), nil},
				{"ToUpper", strings.Ref("ToUpper"), nil},
				{"ToLower", strings.Ref("ToLower"), nil},
				{"Fields", strings.Ref("Fields"), nil},
				{"Repeat", strings.Ref("Repeat"), nil},
				{"Split", strings.Ref("Split"), nil},
				{"SplitAfter", strings.Ref("SplitAfter"), nil},
				{"SplitN", strings.Ref("SplitN"), nil},
				{"SplitAfterN", strings.Ref("SplitAfterN"), nil},
				{"Replace", strings.Ref("Replace"), nil},
				{"ReplaceAll", strings.Ref("ReplaceAll"), nil},
				{"Trim", strings.Ref("Trim"), nil},
				{"TrimSpace", strings.Ref("TrimSpace"), nil},
				{"TrimLeft", strings.Ref("TrimLeft"), nil},
				{"TrimRight", strings.Ref("TrimRight"), nil},
				{"TrimPrefix", strings.Ref("TrimPrefix"), nil},
				{"TrimSuffix", strings.Ref("TrimSuffix"), nil},
			},
		}
	}
	if strings.isValid() {
		stringSliceTI = &BuiltinTI{
			typ: types.NewSlice(types.Typ[types.String]),
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Cap", btoCap, nil},
				{"Join", strings.Ref("Join"), nil},
			},
		}
	}
	tis := []*BuiltinTI{
		osxTI,
		float64TI,
		intTI,
		int64TI,
		uint64TI,
		stringTI,
		stringSliceTI,
		{
			typ: tySlice,
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
				{"Cap", btoCap, nil},
			},
		},
		{
			typ: tyChan,
			methods: []*BuiltinMethod{
				{"Len", btoLen, nil},
			},
		},
	}
	for _, ti := range tis {
		if ti != nil {
			btiMap.Set(ti.typ, ti)
		}
	}
	pkg.cb.btiMap = btiMap
}

func (p *CodeBuilder) getBuiltinTI(typ types.Type) *BuiltinTI {
	switch t := typ.(type) {
	case *types.Basic:
		typ = types.Default(typ)
	case *types.Slice:
		if t.Elem() != types.Typ[types.String] {
			typ = tySlice
		}
	case *types.Chan:
		typ = tyChan
	}
	if bti := p.btiMap.At(typ); bti != nil {
		return bti.(*BuiltinTI)
	}
	return nil
}

// ----------------------------------------------------------------------------

func (p *Package) BuiltinTI(typ types.Type) *BuiltinTI {
	return p.cb.getBuiltinTI(typ)
}

// ----------------------------------------------------------------------------
