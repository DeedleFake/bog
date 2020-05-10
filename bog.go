package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

func processFile(ctx context.Context, dst, src string) error {
	panic("Not implemented.")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %v [options] [source directory]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}
	output := flag.String("out", "", "output directory, or source directory if blank")
	flag.Parse()

	source := flag.Arg(0)
	if source == "" {
		source = "."
	}

	dir, err := os.Open(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: open source directory: %v\n", err)
		os.Exit(1)
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: readdir on source directory: %v\n", err)
		os.Exit(1)
	}

	eg, ctx := errgroup.WithContext(context.Background())
	for _, file := range files {
		if strings.ToLower(filepath.Ext(file.Name())) != ".md" {
			continue
		}

		file := file
		eg.Go(func() error {
			return processFile(ctx, *output, filepath.Join(source, file.Name()))
		})
	}

	err = eg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
