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
	"text/template"

	"github.com/DeedleFake/bog/internal/bufpool"
	"github.com/DeedleFake/bog/markdown"
	"github.com/gosimple/slug"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/sync/errgroup"
)

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

	buf := bufpool.Get()
	_, err = io.Copy(buf, file)
	return buf, err
}

func processFile(ctx context.Context, dst, src string, tmpl *template.Template) error {
	srcbuf, err := readFile(src)
	defer bufpool.Put(srcbuf)
	if err != nil {
		return err
	}

	srcinfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	md := blackfriday.New()
	node := md.Parse(srcbuf.Bytes())

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

	dstbuf := bufpool.Get()
	defer bufpool.Put(dstbuf)

	err = markdown.Render(dstbuf, node, blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{}))
	if err != nil {
		return fmt.Errorf("render %q: %w", dst, err)
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

	err = tmpl.Execute(out, map[string]interface{}{
		"content": dstbuf.String(),
		"meta":    meta,
	})
	if err != nil {
		return fmt.Errorf("execute template for %q: %v", src, err)
	}

	return ctx.Err()
}

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

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %v [options] [source directory]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}
	output := flag.String("out", "", "output directory, or source directory if blank")
	page := flag.String("page", "", "if not blank, path to page template")
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

	pageTmpl, err := loadTemplate(template.New("page"), defaultPage, *page)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load page template: %v\n", err)
		os.Exit(1)
	}

	eg, ctx := errgroup.WithContext(context.Background())
	for _, file := range files {
		if strings.ToLower(filepath.Ext(file.Name())) != ".md" {
			continue
		}

		file := file
		eg.Go(func() error {
			return processFile(ctx, *output, filepath.Join(source, file.Name()), pageTmpl)
		})
	}

	err = eg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
