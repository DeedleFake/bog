package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/DeedleFake/bog/internal/bufpool"
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

// readJSONFile parses JSON data from the file at path.
func readJSONFile(path string) (v interface{}, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return v, nil
}
