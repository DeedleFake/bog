package main

import "path/filepath"

// RemoveExt removes any extensions from the path provided.
func RemoveExt(path string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)]
}
