//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func main() {
	var buf bytes.Buffer
	outf := func(format string, args ...interface{}) {
		fmt.Fprintf(&buf, format, args...)
	}
	outf(`/*
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

`)
	outf("// Code generated by mkstdpkg.go. DO NOT EDIT.\n\n")
	outf("package gox\n")
	outf("var stdpkg = map[string]bool{\n")
	pkgs := stdList()
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		outf("\t%q : true,\n", pkg)
	}
	outf("}\n")
	fmtbuf, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("zstdpkg.go", fmtbuf, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func stdList() []string {
	cmd := exec.Command("go", "list", "std")
	data, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	var ar []string
	for _, v := range strings.Split(string(data), "\n") {
		if v == "" {
			continue
		}
		if isSkipPkg(v) {
			continue
		}
		ar = append(ar, v)
	}
	return append(ar, "syscall/js")
}

func isSkipPkg(pkg string) bool {
	if strings.HasPrefix(pkg, "vendor/") {
		return true
	}
	for _, v := range strings.Split(pkg, "/") {
		if v == "internal" {
			return true
		}
	}
	return false
}
