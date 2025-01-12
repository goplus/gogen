/*
 Copyright 2022 The GoPlus Authors (goplus.org)
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

package cache

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	HashInvalid = "?"
	HashSkip    = ""
)

type depPkg struct {
	path string
	hash string // empty means dirty cache
}

type pkgCache struct {
	expfile string
	hash    string
	deps    []depPkg
}

// PkgHash represents a package hash function.
type PkgHash func(pkgPath string, self bool) (hash string)

// Impl represents a cache.
type Impl struct {
	cache sync.Map // map[string]*pkgCache
	h     PkgHash  // package hash func
	nlist int32    // list count
	tags  string
}

// New creates a new cache.
func New(h PkgHash) *Impl {
	return &Impl{h: h}
}

func (p *Impl) SetTags(tags string) {
	p.tags = tags
}

func (p *Impl) Tags() string {
	return p.tags
}

// ----------------------------------------------------------------------------

// ListTimes returns the number of times of calling `go list`.
func (p *Impl) ListTimes() int {
	return int(atomic.LoadInt32(&p.nlist))
}

// Prepare prepares the cache for a list of pkgPath.
func (p *Impl) Prepare(dir string, pkgPath ...string) (err error) {
	atomic.AddInt32(&p.nlist, 1)
	ret, err := golistExport(dir, pkgPath, p.tags)
	if err != nil {
		return
	}
	h := p.h
	for _, v := range ret {
		deps := make([]depPkg, 0, len(v.deps))
		pkg := &pkgCache{expfile: v.expfile, hash: h(v.path, true), deps: deps}
		for _, dep := range v.deps {
			if hash := h(dep, false); hash != HashSkip {
				pkg.deps = append(pkg.deps, depPkg{dep, hash})
			}
		}
		p.cache.Store(v.path, pkg)
	}
	return
}

// Find finds the cache for a pkgPath.
func (p *Impl) Find(dir, pkgPath string) (f io.ReadCloser, err error) {
	val, ok := p.cache.Load(pkgPath)
	if !ok || isDirty(&f, pkgPath, val, p.h) {
		err = p.Prepare(dir, pkgPath)
		if val, ok = p.cache.Load(pkgPath); ok {
			return os.Open(val.(*pkgCache).expfile)
		}
		if err == nil {
			err = os.ErrNotExist
		}
	}
	return
}

func isDirty(pf *io.ReadCloser, pkgPath string, val any, h PkgHash) bool {
	pkg := val.(*pkgCache)
	if pkg.hash == HashInvalid || h(pkgPath, true) != pkg.hash {
		return true
	}
	for _, dep := range pkg.deps {
		if h(dep.path, false) != dep.hash {
			return true
		}
	}
	f, err := os.Open(pkg.expfile)
	*pf = f
	return err != nil
}

type exportPkg struct {
	path    string
	expfile string
	deps    []string
}

func golistExport(dir string, pkgPath []string, tags string) (ret []exportPkg, err error) {
	var stdout, stderr bytes.Buffer
	var args = make([]string, 0, 3+len(pkgPath))
	args = append(args, "list", "-f={{.ImportPath}}\t{{.Export}}\t{{.Deps}}")
	if tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, "-export")
	args = append(args, pkgPath...)
	cmd := exec.Command("go", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err = cmd.Run()
	if err == nil {
		return parseExports(stdout.String())
	} else if stderr.Len() > 0 {
		err = errors.New(stderr.String())
	}
	return
}

func parseExports(s string) (ret []exportPkg, err error) {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	ret = make([]exportPkg, 0, len(lines))
	for _, line := range lines {
		v, e := parseExport(line)
		if e != nil {
			return nil, e
		}
		ret = append(ret, v)
	}
	return
}

var (
	errInvalidFormat = errors.New("invalid format")
)

// {{.ImportPath}}\t{{.Export}}\t{{.Deps}}
//
// <path>	<expfile>	[<depPkg1> <depPkg2> ...]
func parseExport(line string) (ret exportPkg, err error) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		err = errInvalidFormat
		return
	}
	deps := parts[2]
	if len(deps) > 2 {
		ret.deps = strings.Split(deps[1:len(deps)-1], " ")
	}
	ret.path, ret.expfile = parts[0], parts[1]
	return
}

// ----------------------------------------------------------------------------

var (
	readFile  = os.ReadFile
	writeFile = os.WriteFile
)

/*
DiskCache cacheFile format:
	<pkgPath> <exportFile> <pkgHash> <depPkgNum>
		<depPkgPath1> <depPkgHash1>
		<depPkgPath2> <depPkgHash2>
		...
	<pkgPath> <exportFile> <depPkgNum>
		...
*/

// Load loads the cache from a file.
func (p *Impl) Load(cacheFile string) (err error) {
	b, err := readFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	lines := strings.Split(string(bytes.TrimRight(b, "\n")), "\n")
	return p.loadCachePkgs(lines)
}

func (p *Impl) loadCachePkgs(lines []string) error {
	for len(lines) > 0 {
		line := lines[0]
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 || parts[0] == "" {
			return errInvalidFormat
		}
		n, e := strconv.Atoi(parts[3])
		if e != nil || len(lines) < n+1 {
			return errInvalidFormat
		}
		deps := make([]depPkg, 0, n)
		for i := 1; i <= n; i++ {
			line = lines[i]
			if !strings.HasPrefix(line, "\t") {
				return errInvalidFormat
			}
			line = line[1:]
			pos := strings.IndexByte(line, '\t')
			if pos <= 0 {
				return errInvalidFormat
			}
			deps = append(deps, depPkg{line[:pos], line[pos+1:]})
		}
		pkg := &pkgCache{expfile: parts[1], hash: parts[2], deps: deps}
		p.cache.Store(parts[0], pkg)
		lines = lines[n+1:]
	}
	return nil
}

// Save saves the cache to a file.
func (p *Impl) Save(cacheFile string) (err error) {
	if atomic.LoadInt32(&p.nlist) == 0 { // not dirty
		return
	}
	var buf bytes.Buffer
	p.cache.Range(func(key, val any) bool {
		pkg := val.(*pkgCache)
		buf.WriteString(key.(string))
		buf.WriteByte('\t')
		buf.WriteString(pkg.expfile)
		buf.WriteByte('\t')
		buf.WriteString(pkg.hash)
		buf.WriteByte('\t')
		buf.WriteString(strconv.Itoa(len(pkg.deps)))
		buf.WriteByte('\n')
		for _, dep := range pkg.deps {
			buf.WriteByte('\t')
			buf.WriteString(dep.path)
			buf.WriteByte('\t')
			buf.WriteString(dep.hash)
			buf.WriteByte('\n')
		}
		return true
	})
	return writeFile(cacheFile, buf.Bytes(), 0666)
}

// ----------------------------------------------------------------------------
