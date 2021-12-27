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

package packages

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ----------------------------------------------------------------------------

type pkgExport = string

func loadDeps(tempDir string, pkgPaths ...string) (pkgs map[string]pkgExport, err error) {
	pkgs, err = tryLoadDeps(tempDir, pkgPaths...)
	if err != nil {
		if napp := getProgramList(err.Error(), pkgPaths); napp > 0 {
			if pkgs, err = tryLoadDeps(tempDir, pkgPaths[napp:]...); err == nil {
				for _, appPath := range pkgPaths[:napp] {
					if err = loadDepPkgs(pkgs, appPath); err != nil {
						break
					}
				}
			}
		}
	}
	return
}

func getProgramList(msg string, pkgPaths []string) (napp int) {
	const prefixImp = `: import "`
	const suffixImp = `" is a program,`
	for {
		pos := strings.Index(msg, prefixImp)
		if pos < 0 {
			return
		}
		msg = msg[pos+len(prefixImp):]
		pos = strings.Index(msg, suffixImp)
		if pos < 0 {
			return 0
		}
		napp = addProgram(pkgPaths, napp, msg[:pos])
		msg = msg[pos+len(suffixImp):]
	}
}

func addProgram(pkgPaths []string, napp int, appPath string) int {
	for i := napp; i < len(pkgPaths); i++ {
		if pkgPaths[i] == appPath {
			pkgPaths[napp], pkgPaths[i] = pkgPaths[i], pkgPaths[napp]
			napp++
			break
		}
	}
	return napp
}

var (
	gid = 0
)

func initLoadDeps() {
	rand.Seed(time.Now().UnixNano())
}

func tryLoadDeps(tempDir string, pkgPaths ...string) (pkgs map[string]pkgExport, err error) {
	gid++
	file := tempDir + "/dummy-" + strconv.Itoa(gid) + ".go"
	os.MkdirAll(tempDir, 0755)

	var buf bytes.Buffer
	buf.WriteString(`package main

import (
`)
	for _, pkgPath := range pkgPaths {
		fmt.Fprintf(&buf, "\t_ \"%s\"\n", pkgPath)
	}
	fmt.Fprintf(&buf, `)

// %x, %x
func main() {
}
`, time.Now().UnixNano(), rand.Int63())
	err = os.WriteFile(file, buf.Bytes(), 0644)
	if err != nil {
		return
	}
	if debugRemoveTempFile {
		defer func() {
			os.Remove(file)
			os.Remove(tempDir)
		}()
	}
	pkgs = make(map[string]pkgExport)
	err = loadDepPkgs(pkgs, file)
	return
}

func loadDepPkgs(pkgs map[string]pkgExport, src string) (err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "install", "-work", "-x", src)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return &ExecCmdError{Err: err, Stderr: stderr.Bytes()}
	}
	wd, err := loadDepPkgsFrom(pkgs, stderr.String())
	if err == nil {
		os.RemoveAll(wd)
	}
	return
}

var (
	ErrWorkDirNotFound = errors.New("WorkDir not found")
)

func loadDepPkgsFrom(pkgs map[string]pkgExport, data string) (wd string, err error) {
	const packagefile = "packagefile "
	const workdir = "WORK="
	if !strings.HasPrefix(data, workdir) {
		return "", ErrWorkDirNotFound
	}
	data = data[len(workdir):]
	pos := strings.IndexByte(data, '\n')
	if pos < 0 {
		return "", ErrWorkDirNotFound
	}
	wd, data = data[:pos], data[pos+1:]
	for data != "" {
		pos := strings.IndexByte(data, '\n')
		if pos < 0 {
			break
		}
		if strings.HasPrefix(data, packagefile) {
			line := data[len(packagefile):pos]
			if t := strings.Index(line, "="); t > 0 {
				pkgPath := line[:t]
				if expfile := pkgExport(line[t+1:]); !strings.HasPrefix(expfile, "$") {
					pkgs[pkgPath] = expfile
				}
			}
		}
		data = data[pos+1:]
	}
	return
}

// ----------------------------------------------------------------------------
