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
	"encoding/json"
	"errors"
	"os/exec"
)

// ----------------------------------------------------------------------------

type PkgToLoad struct {
	ImportPath     string
	Root           string
	Export         string
	GoFiles        []string
	IgnoredGoFiles []string
	Imports        []string
	Deps           []string
}

type ExecCmdError struct {
	Err    error
	Stderr []byte
}

func (p *ExecCmdError) Error() string {
	if e := p.Stderr; e != nil {
		return string(e)
	}
	return p.Err.Error()
}

func LoadPkgs(dir string, pattern ...string) (pkgs []*PkgToLoad, err error) {
	args := make([]string, len(pattern)+3)
	args[0], args[1], args[2] = "list", "-export", "-json"
	copy(args[3:], pattern)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil || stderr.Len() != 0 {
		return nil, &ExecCmdError{Err: err, Stderr: stderr.Bytes()}
	}
	return loadPkgsFrom(make([]*PkgToLoad, 0, len(pattern)), stdout.Bytes())
}

func loadPkgsFrom(pkgs []*PkgToLoad, data []byte) (out []*PkgToLoad, err error) {
	prefixStart := []byte{'{', '\n'}
	prefixEnd := []byte{'}', '\n'}
	for bytes.HasPrefix(data, prefixStart) {
		end := 2
		for !bytes.HasPrefix(data[end:], prefixEnd) {
			if t := bytes.IndexByte(data[end:], '\n'); t < 0 {
				goto failed
			} else {
				end += t + 1
			}
		}
		end += 2
		pkg := new(PkgToLoad)
		if err = json.Unmarshal(data[:end], pkg); err != nil {
			return
		}
		pkgs = append(pkgs, pkg)
		data = data[end:]
	}
	if len(data) == 0 {
		return pkgs, nil
	}
failed:
	return nil, errors.New("invalid results of `go list`")
}

// ----------------------------------------------------------------------------
