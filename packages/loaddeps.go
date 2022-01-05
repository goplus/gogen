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

func loadDeps(tempDir string, pkgPaths ...string) (pkgs map[string]pkgExport, wds []string, err error) {
	pkgs, wd, err := tryLoadDeps(tempDir, pkgPaths...)
	if err != nil {
		if napp := getProgramList(err.Error(), pkgPaths); napp > 0 {
			if pkgs, wd, err = tryLoadDeps(tempDir, pkgPaths[napp:]...); err == nil {
				wds = appendWorkDir(wds, wd)
				for _, appPath := range pkgPaths[:napp] {
					if wd, err = loadDepPkgs(pkgs, tempDir, appPath); err != nil {
						cleanWorkDirs(wds)
						break
					}
					wds = appendWorkDir(wds, wd)
				}
			}
		}
	} else {
		wds = appendWorkDir(wds, wd)
	}
	return
}

func appendWorkDir(wds []string, wd string) []string {
	if wd != "" {
		wds = append(wds, wd)
	}
	return wds
}

func cleanWorkDirs(wds []string) {
	for _, wd := range wds {
		os.RemoveAll(wd)
	}
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

func tryLoadDeps(tempDir string, pkgPaths ...string) (pkgs map[string]pkgExport, wd string, err error) {
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
	wd, err = loadDepPkgs(pkgs, tempDir, file)
	return
}

func loadDepPkgs(pkgs map[string]pkgExport, dir, src string) (wd string, err error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "install", "-work", "-x", src)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if !strings.HasPrefix(dir, ".") {
		cmd.Dir = dir
	}
	err = cmd.Run()
	if err != nil {
		err = &ExecCmdError{Err: err, Stderr: stderr.Bytes()}
		return
	}
	return loadDepPkgsFrom(pkgs, stderr.String())
}

var (
	ErrWorkDirNotFound = errors.New("loadDeps: WorkingDir $WORK not found")
)

func loadDepPkgsFrom(pkgs map[string]pkgExport, data string) (workd string, err error) {
	const packagefile = "packagefile "
	const workdir = "WORK="
	const workvar = "$WORK"
	pos := strings.Index(data, workdir)
	if pos < 0 {
		fmt.Fprint(os.Stderr, data)
		return "", ErrWorkDirNotFound
	}
	data = data[pos+len(workdir):]
	pos = strings.IndexByte(data, '\n')
	if pos < 0 {
		return "", ErrWorkDirNotFound
	}
	wd, data := data[:pos], data[pos+1:]
	defer func() {
		if workd == "" {
			os.RemoveAll(wd)
		}
	}()
	for data != "" {
		pos := strings.IndexByte(data, '\n')
		if pos < 0 {
			break
		}
		if strings.HasPrefix(data, packagefile) {
			line := data[len(packagefile):pos]
			if t := strings.Index(line, "="); t > 0 && line[:t] != "command-line-arguments" {
				expfile := pkgExport(line[t+1:])
				if strings.HasPrefix(expfile, workvar) {
					expfile = wd + expfile[len(workvar):]
					workd = wd
				}
				pkgs[line[:t]] = expfile
			}
		}
		data = data[pos+1:]
	}
	return
}

// ----------------------------------------------------------------------------
