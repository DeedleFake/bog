package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/DeedleFake/bog/internal/bufpool"
	"gopkg.in/yaml.v2"
)

// readFile reads a file into buffer that is retrieved from the buffer
// pool.
func readFile(path string) (*bytes.Buffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := bufpool.Get()
	_, err = io.Copy(buf, file)
	return buf, err
}

// readYAMLFile parses YAML data from the file at path.
func readYAMLFile(path string) (v interface{}, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(&v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return v, nil
}

// fileExists returns true if the file exists.
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	ok := !os.IsNotExist(err)
	if ok && (err != nil) {
		return ok, err
	}
	return ok, nil
}
