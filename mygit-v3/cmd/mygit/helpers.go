package main

import (
	"path/filepath"
	"time"
)

func timeNow() int64 {
	return time.Now().Unix()
}

func filepathRel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}
