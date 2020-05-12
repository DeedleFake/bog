package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gosimple/slug"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/sync/errgroup"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

var defaultMeta = map[string]func(os.FileInfo) interface{}{
	"title": func(file os.FileInfo) interface{} {
		return RemoveExt(filepath.Base(file.Name()))
	},
}

func readFile(path string) (*bytes.Buffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := bufPool.Get().(*bytes.Buffer)
	_, err = io.Copy(buf, file)
	return buf, err
}

func processFile(ctx context.Context, dst, src string) error {
	buf, err := readFile(src)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()
	if err != nil {
		return err
	}

	srcinfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	md := blackfriday.New()
	node := md.Parse(buf.Bytes())

	meta, err := getMeta(node)
	if err != nil {
		return fmt.Errorf("get meta from %q: %w", src, err)
	}
	for k, f := range defaultMeta {
		if _, ok := meta[k]; ok {
			continue
		}

		meta[k] = f(srcinfo)
	}

	dst = filepath.Join(dst, slug.Make(meta["title"].(string))+".html")
	dstinfo, err := os.Stat(dst)
	if (err != nil) && !os.IsNotExist(err) {
		return fmt.Errorf("stat %q: %w", dst, err)
	}
	if (dstinfo != nil) && dstinfo.ModTime().After(srcinfo.ModTime()) {
		return nil
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	err = RenderMarkdown(out, node, blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{}))
	if err != nil {
		return fmt.Errorf("render %q: %w", dst, err)
	}

	return nil
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
	if *output == "" {
		*output = source
	}

	err := os.MkdirAll(*output, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: make output directory: %v\n", err)
		os.Exit(1)
	}

	files, err := ioutil.ReadDir(source)
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
