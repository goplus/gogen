package packages

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
)

// pkgPath Caches
type Cache struct {
	dirCache          map[string]bool
	dirCacheMutex     sync.RWMutex
	packageCacheMap   map[string]CacheInfo
	packageCacheMutex sync.RWMutex
	waitCache         sync.WaitGroup
}

type CacheInfo struct {
	ImportPath string
	PkgDir     string
	PkgExport  string
}

// https://github.com/goplus/gop/issues/1710
// Not fully optimized
// Retrieve all imports in the specified directory and cache them
func (c *Cache) goListExportCache(dir string, pkgs ...string) {
	c.dirCacheMutex.Lock()
	if c.dirCache[dir] {
		c.dirCacheMutex.Unlock()
		return
	}
	c.dirCache[dir] = true
	c.dirCacheMutex.Unlock()
	var stdout, stderr bytes.Buffer
	commandStr := []string{"list", "-f", "{{.ImportPath}},{{.Dir}},{{.Export}}", "-export", "-e"}
	commandStr = append(commandStr, pkgs...)
	commandStr = append(commandStr, "all")
	cmd := exec.Command("go", commandStr...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		ret := stdout.String()
		for _, v := range strings.Split(ret, "\n") {
			s := strings.Split(v, ",")
			if len(s) != 3 {
				continue
			}
			c.packageCacheMutex.Lock()
			c.packageCacheMap[s[0]] = CacheInfo{s[0], s[1], s[2]}
			c.packageCacheMutex.Unlock()
		}
	}
}

// Reduce wait time when performing `go list` on multiple pkgDir at the same time
func (c *Cache) GoListExportCacheSync(dir string, pkgs ...string) {
	c.waitCache.Add(1)
	go func() {
		defer c.waitCache.Done()
		c.goListExportCache(dir, pkgs...)
	}()
}

func (c *Cache) GetPkgCache(pkgPath string) (ret CacheInfo, ok bool) {
	c.waitCache.Wait()
	c.packageCacheMutex.RLock()
	ret, ok = c.packageCacheMap[pkgPath]
	c.packageCacheMutex.RUnlock()
	return
}

func (c *Cache) DelPkgCache(pkgPath string) bool {
	_, ok := c.GetPkgCache(pkgPath)
	if ok {
		c.packageCacheMutex.Lock()
		delete(c.packageCacheMap, pkgPath)
		c.packageCacheMutex.Unlock()
		return true
	}
	return false
}

func NewGoListCache(dir string) *Cache {
	c := &Cache{
		dirCache:        make(map[string]bool),
		packageCacheMap: make(map[string]CacheInfo),
	}
	// get the dir pkg cache
	c.GoListExportCacheSync(dir)
	return c
}
