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

	"github.com/DeedleFake/bog/internal/cli"
	"github.com/DeedleFake/bog/multierr"
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

// printErrors prints the provided intro and then the list of errors,
// indented, to stderr.
func printErrors(intro string, errs []error) {
	fmt.Fprintln(os.Stderr, intro)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "\t%v\n", err)
	}
}

// extraFlag parses the -extras flag.
type extraFlag map[string]string

func (f extraFlag) String() string {
	var sb strings.Builder

	var sep string
	for k, v := range f {
		fmt.Fprintf(&sb, "%s%v:%v", sep, k, v)
		sep = ","
	}

	return sb.String()
}

func (f extraFlag) Set(v string) error {
	pairs := strings.Split(v, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid extra specification: %q", pair)
		}

		f[parts[0]] = parts[1]
	}

	return nil
}

type flags struct {
	Output   string    `flag:"out,,output directory, or source directory if blank"`
	Page     string    `flag:"page,,if not blank, path to page template"`
	Index    string    `flag:"index,,if not blank, path to index template"`
	GenIndex bool      `flag:"genindex,true,generate an index"`
	Data     string    `flag:"data,,path to optional YAML data file"`
	HLStyle  string    `flag:"hlstyle,monokai,Chroma syntax highlighting style"`
	Extras   extraFlag `flag:"extras,comma-separated template:output pairs of extra files to render"`

	Source string `flag:"0,."`
}

func main() {
	ctx := cli.SignalContext(context.Background(), os.Interrupt)

	var flags flags
	err := cli.ParseFlags(&flags, func() {
		fmt.Fprintf(os.Stderr, "Usage: %v [options] [source directory]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: parse flags: %v\n", err)
		os.Exit(2)
	}
	if flags.Output == "" {
		flags.Output = flags.Source
	}

	var data interface{}
	if flags.Data != "" {
		d, err := readYAMLFile(flags.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: read %q: %v\n", flags.Data, err)
			os.Exit(1)
		}
		data = d
	}

	files, err := ioutil.ReadDir(flags.Source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: readdir on source directory: %v\n", err)
		os.Exit(1)
	}

	pageTmpl, err := loadTemplate(template.New("page").Funcs(tmplFuncs), defaultPage, flags.Page)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load page template: %v\n", err)
		os.Exit(1)
	}

	indexTmpl, err := loadTemplate(template.New("index").Funcs(tmplFuncs), defaultIndex, flags.Index)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load index template: %v\n", err)
		os.Exit(1)
	}

	// BUG: This way of doing the parsing results in an inability to use
	// two files with the same name in different directories.
	var extraTmpls *template.Template
	if len(flags.Extras) > 0 {
		extraSrcs := make([]string, 0, len(flags.Extras))
		for src := range flags.Extras {
			extraSrcs = append(extraSrcs, src)
		}
		extraTmpls, err = template.New("extras").Funcs(tmplFuncs).ParseFiles(extraSrcs...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error load extra templates: %v\n", err)
			os.Exit(1)
		}
	}

	var pages []*PageInfo
	pagec := make(chan *PageInfo)
	pagesDone := make(chan struct{})
	go func() {
		defer close(pagesDone)

		for page := range pagec {
			i := sort.Search(len(pages), func(i int) bool {
				return page.Meta["time"].(time.Time).After(pages[i].Meta["time"].(time.Time))
			})

			pages = append(pages, nil)
			copy(pages[i+1:], pages[i:])
			pages[i] = page
		}
	}()

	eg, ctx := multierr.WithContext(ctx)
	for _, file := range files {
		if strings.ToLower(filepath.Ext(file.Name())) != ".md" {
			continue
		}

		file := file
		eg.Go(func() error {
			path := filepath.Join(flags.Source, file.Name())
			page, err := LoadPage(path, data, WithStyle(flags.HLStyle))
			if err != nil {
				return fmt.Errorf("load %q: %w", path, err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case pagec <- page:
				return nil
			}
		})
	}

	errs := eg.Wait()
	if len(errs) > 0 {
		printErrors("Error(s) while loading pages:", errs)
		os.Exit(1)
	}
	close(pagec)
	<-pagesDone

	err = os.MkdirAll(flags.Output, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: make output directory: %v\n", err)
		os.Exit(1)
	}

	eg, ctx = multierr.WithContext(ctx)

	eg.Go(func() error {
		if !flags.GenIndex {
			return nil
		}

		err = genIndex(flags.Output, pages, indexTmpl, data)
		if err != nil {
			return fmt.Errorf("generate index: %w", err)
		}

		fmt.Printf("Generated %q\n", filepath.Join(flags.Output, "index.html"))
		return nil
	})

	for _, page := range pages {
		page := page
		eg.Go(func() error {
			path := filepath.Join(flags.Output, page.Output())
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

	for src, dst := range flags.Extras {
		src, dst := src, dst
		eg.Go(func() error {
			path := filepath.Join(flags.Output, dst)

			file, err := os.Create(path)
			if err != nil {
				return err
			}
			defer file.Close()

			err = extraTmpls.ExecuteTemplate(file, filepath.Base(src), map[string]interface{}{
				"Data":  data,
				"Pages": pages,
			})
			if err != nil {
				return fmt.Errorf("execute %q: %w", src, err)
			}

			fmt.Printf("Generated %q\n", path)
			return nil
		})
	}

	errs = eg.Wait()
	if len(errs) > 0 {
		printErrors("Error(s) while generating output:", errs)
		os.Exit(1)
	}
}
