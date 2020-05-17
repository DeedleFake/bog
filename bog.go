package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/DeedleFake/bog/internal/bufpool"
	"github.com/DeedleFake/bog/markdown"
	"github.com/gosimple/slug"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/sync/errgroup"
)

// defaultMeta contains a mapping of names to functions that are
// called in order to provide metadata values that haven't been
// explicitly listed.
var defaultMeta = map[string]func(os.FileInfo) interface{}{
	"title": func(file os.FileInfo) interface{} {
		return RemoveExt(filepath.Base(file.Name()))
	},
}

// pageInfo contains information that was collected while processing
// a page.
type pageInfo struct {
	Src, Dst         string
	SrcInfo, DstInfo os.FileInfo
	Meta             map[string]interface{}
}

// processPage loads a page from src, gets the metadata, runs it
// through the provided template with that metadata and the provided
// data, writes it to a file in the directory at dst, and then returns
// information about the page that was processed.
func processPage(ctx context.Context, dst, src string, tmpl *template.Template, data interface{}) (info *pageInfo, err error) {
	srcbuf, err := readFile(src)
	defer bufpool.Put(srcbuf)
	if err != nil {
		return nil, err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, err
	}

	md := blackfriday.New()
	node := md.Parse(srcbuf.Bytes())

	meta, err := getMeta(node, true)
	if err != nil {
		return nil, fmt.Errorf("get meta from %q: %w", src, err)
	}
	for k, f := range defaultMeta {
		if _, ok := meta[k]; ok {
			continue
		}

		meta[k] = f(srcInfo)
	}

	dstbuf := bufpool.Get()
	defer bufpool.Put(dstbuf)

	err = markdown.Render(dstbuf, node, blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{}))
	if err != nil {
		return nil, fmt.Errorf("render %q: %w", dst, err)
	}

	dst = filepath.Join(dst, slug.Make(meta["title"].(string))+".html")

	dstinfo, err := os.Stat(dst)
	if (err != nil) && !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %q: %w", dst, err)
	}
	if (dstinfo != nil) && dstinfo.ModTime().After(srcInfo.ModTime()) {
		return nil, ctx.Err()
	}

	out, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	contentTmpl, err := template.New("content").Funcs(tmplFuncs).Parse(dstbuf.String())
	if err != nil {
		return nil, fmt.Errorf("parse content template for %q: %w", src, err)
	}

	dstbuf.Reset()
	err = contentTmpl.Execute(dstbuf, map[string]interface{}{
		"Data": data,
		"Meta": meta,
	})
	if err != nil {
		return nil, fmt.Errorf("execute content template for %q: %w", src, err)
	}

	err = tmpl.Execute(out, map[string]interface{}{
		"Data":    data,
		"Meta":    meta,
		"Content": dstbuf.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("execute template for %q: %w", src, err)
	}

	dstinfo, err = os.Stat(dst)
	if err != nil {
		return nil, err
	}

	return &pageInfo{
		Src:     src,
		Dst:     dst,
		SrcInfo: srcInfo,
		DstInfo: dstinfo,
		Meta:    meta,
	}, ctx.Err()
}

// genIndex generates an index of the provided pages using the
// provided template and writes it to a file under the directory at
// dst.
func genIndex(dst string, pages []*pageInfo, tmpl *template.Template, data interface{}) error {
	file, err := os.Create(filepath.Join(dst, "index.html"))
	if err != nil {
		return err
	}
	defer file.Close()

	err = tmpl.Execute(file, map[string]interface{}{
		"Data":  data,
		"Pages": pages,
	})
	if err != nil {
		return fmt.Errorf("execute index template: %w", err)
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
	page := flag.String("page", "", "if not blank, path to page template")
	index := flag.String("index", "", "if not blank, path to index template")
	genindex := flag.Bool("genindex", true, "generate an index")
	datafile := flag.String("data", "", "path to optional JSON data file")
	flag.Parse()

	source := flag.Arg(0)
	if source == "" {
		source = "."
	}
	if *output == "" {
		*output = source
	}

	var data interface{}
	if *datafile != "" {
		d, err := readJSONFile(*datafile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read %q: %v\n", *datafile, err)
			os.Exit(1)
		}
		data = d
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

	pageTmpl, err := loadTemplate(template.New("page").Funcs(tmplFuncs), defaultPage, *page)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load page template: %v\n", err)
		os.Exit(1)
	}

	indexTmpl, err := loadTemplate(template.New("index").Funcs(tmplFuncs), defaultIndex, *index)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load index template: %v\n", err)
		os.Exit(1)
	}

	eg, ctx := errgroup.WithContext(context.Background())

	var pages []*pageInfo
	pagec := make(chan *pageInfo)
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(pagec)
				return

			case page := <-pagec:
				i := sort.Search(len(pages), func(i int) bool {
					return page.DstInfo.ModTime().Before(pages[i].DstInfo.ModTime())
				})

				pages = append(pages, nil)
				copy(pages[i+1:], pages[i:])
				pages[i] = page

				fmt.Printf("%q -> %q\n", page.Src, page.Dst)
			}
		}
	}()

	for _, file := range files {
		if strings.ToLower(filepath.Ext(file.Name())) != ".md" {
			continue
		}

		file := file
		eg.Go(func() error {
			page, err := processPage(ctx, *output, filepath.Join(source, file.Name()), pageTmpl, data)
			if page == nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case pagec <- page:
				return nil
			}
		})
	}

	err = eg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !*genindex {
		return
	}

	<-pagec
	err = genIndex(*output, pages, indexTmpl, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: generate index: %v\n", err)
		os.Exit(1)
	}
}
