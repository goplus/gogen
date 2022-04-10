package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func GOMODCACHE() string {
	val := os.Getenv("GOMODCACHE")
	if val == "" {
		return filepath.Join(runtime.GOROOT(), "pkg/mod")
	}
	return val
}

func initGoEnv() {
	val := os.Getenv("GOMODCACHE")
	if val == "" {
		os.Setenv("GOMODCACHE", filepath.Join(runtime.GOROOT(), "pkg/mod"))
	}
}
