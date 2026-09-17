package main

import (
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
)

func goServiceRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil && !info.IsDir() {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}
