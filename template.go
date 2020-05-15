package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// loadTemplate conditionally parses a template from either def or
// path. If path is empty, def is considered to be the source and is
// parsed, otherwise the file at path is opened and the contents are
// parsed.
func loadTemplate(tmpl *template.Template, def, path string) (*template.Template, error) {
	if path == "" {
		return tmpl.Parse(def)
	}

	file, err := os.Open(path)
	if err != nil {
		return tmpl, err
	}
	defer file.Close()

	var sb strings.Builder
	_, err = io.Copy(&sb, file)
	if err != nil {
		return tmpl, fmt.Errorf("copy: %w", err)
	}

	return tmpl.Parse(sb.String())
}
