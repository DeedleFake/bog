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
	"time"

	"golang.org/x/sync/errgroup"
)

// genIndex generates an index of the provided pages using the
// provided template and writes it to a file under the directory at
// dst.
func genIndex(dst string, pages []*PageInfo, tmpl *template.Template, data interface{}) error {
	file, err := os.Create(filepath.Join(dst, "index.html"))
	if err != nil {
		return err
	}
	defer file.Close()

	err = tmpl.Execute(file, map[string]interface{}{
		"Pages": pages,
		"Data":  data,
	})
	if err != nil {
		return fmt.Errorf("template execute: %w", err)
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
	datafile := flag.String("data", "", "path to optional YAML data file")
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
		d, err := readYAMLFile(*datafile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read %q: %v\n", *datafile, err)
			os.Exit(1)
		}
		data = d
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

	var pages []*PageInfo
	pagec := make(chan *PageInfo)
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(pagec)
				return

			case page := <-pagec:
				i := sort.Search(len(pages), func(i int) bool {
					return page.Meta["time"].(time.Time).After(pages[i].Meta["time"].(time.Time))
				})

				pages = append(pages, nil)
				copy(pages[i+1:], pages[i:])
				pages[i] = page
			}
		}
	}()

	for _, file := range files {
		if strings.ToLower(filepath.Ext(file.Name())) != ".md" {
			continue
		}

		file := file
		eg.Go(func() error {
			path := filepath.Join(source, file.Name())
			page, err := LoadPage(path, data)
			if err != nil {
				return fmt.Errorf("load %q: $w", path, err)
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
	<-pagec

	err = os.MkdirAll(*output, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: make output directory: %v\n", err)
		os.Exit(1)
	}

	eg, ctx = errgroup.WithContext(context.Background())

	eg.Go(func() error {
		if !*genindex {
			return nil
		}

		err = genIndex(*output, pages, indexTmpl, data)
		if err != nil {
			return fmt.Errorf("generate index: %w", err)
		}

		fmt.Printf("Generated %q\n", filepath.Join(*output, "index.html"))
		return nil
	})

	for _, page := range pages {
		page := page
		eg.Go(func() error {
			path := filepath.Join(*output, page.Output())
			ok, err := fileExists(path)
			if ok || (err != nil) {
				return err
			}

			file, err := os.Create(path)
			if err != nil {
				return err
			}
			defer file.Close()

			err = page.Execute(file, pageTmpl, data)
			if err != nil {
				return fmt.Errorf("execute %q: %w", page.Input(), err)
			}

			fmt.Printf("Generated %q\n", path)
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
