package main

import "path/filepath"

func RemoveExt(path string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)]
}
