package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/DeedleFake/bog/internal/bufpool"
	"github.com/DeedleFake/bog/markdown"
	"github.com/gosimple/slug"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

// defaultMeta contains a mapping of names to functions that are
// called in order to provide metadata values that haven't been
// explicitly listed.
var defaultMeta = map[string]func(os.FileInfo) interface{}{
	"title": func(file os.FileInfo) interface{} {
		return RemoveExt(filepath.Base(file.Name()))
	},

	"time": func(file os.FileInfo) interface{} {
		return file.ModTime()
	},
}

// PageInfo contains information about a page.
type PageInfo struct {
	InputInfo os.FileInfo
	Meta      map[string]interface{}
	Content   string
}

func LoadPage(path string, data interface{}) (*PageInfo, error) {
	buf, err := readFile(path)
	defer bufpool.Put(buf)
	if err != nil {
		return nil, err
	}

	inputInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	md := blackfriday.New(blackfriday.WithExtensions(blackfriday.CommonExtensions))
	node := md.Parse(buf.Bytes())

	meta, err := getMeta(node, true)
	if err != nil {
		return nil, fmt.Errorf("get meta: %w", err)
	}
	for k, f := range defaultMeta {
		if _, ok := meta[k]; ok {
			continue
		}

		meta[k] = f(inputInfo)
	}

	page := &PageInfo{
		InputInfo: inputInfo,
		Meta:      meta,
	}

	mdbuf := bufpool.Get()
	defer bufpool.Put(mdbuf)
	err = page.render(mdbuf, node, blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{}), data)
	if err != nil {
		return nil, fmt.Errorf("render HTML: %w", err)
	}
	page.Content = mdbuf.String()

	return page, nil
}

func (page *PageInfo) render(buf *bytes.Buffer, root *blackfriday.Node, renderer blackfriday.Renderer, data interface{}) error {
	err := markdown.Render(buf, root, blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{}))
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}

	delimLeft, _ := page.getMeta("template", "delims", "left").(string)
	delimRight, _ := page.getMeta("template", "delims", "right").(string)

	tmpl, err := template.New("content").Funcs(tmplFuncs).Delims(delimLeft, delimRight).Parse(buf.String())
	if err != nil {
		return fmt.Errorf("template parse: %w", err)
	}

	buf.Reset()
	err = tmpl.Execute(buf, map[string]interface{}{
		"Page": page,
		"Data": data,
	})
	if err != nil {
		return fmt.Errorf("template execute: %w", err)
	}

	return nil
}

func (page *PageInfo) getMeta(keys ...string) interface{} {
	if len(keys) == 0 {
		panic(errors.New("no keys provided"))
	}

	meta := page.Meta

	for len(keys) > 1 {
		next, ok := page.Meta[keys[0]].(map[string]interface{})
		if !ok {
			return nil
		}

		keys = keys[1:]
		meta = next
	}

	return meta[keys[1]]
}

func (page *PageInfo) Input() string {
	return page.InputInfo.Name()
}

func (page *PageInfo) Output() string {
	return slug.Make(fmt.Sprint(page.Meta["title"])) + ".html"
}

func (page *PageInfo) Execute(w io.Writer, tmpl *template.Template, data interface{}) error {
	err := tmpl.Execute(w, map[string]interface{}{
		"Page": page,
		"Data": data,
	})
	if err != nil {
		return fmt.Errorf("template execute: %w", err)
	}

	return nil
}

// getMeta finds and retrieves metadata from a parsed markdown tree.
// If unlink is true, the node containing the metadata is removed from
// the tree.
func getMeta(node *blackfriday.Node, unlink bool) (meta map[string]interface{}, werr error) {
	var findComment func(*html.Node) (comment []byte, err error)
	findComment = func(node *html.Node) (comment []byte, err error) {
		if node.Type == html.CommentNode {
			return []byte(node.Data), nil
		}

		for node := node.FirstChild; node != nil; node = node.NextSibling {
			comment, err = findComment(node)
			if (comment != nil) || (err != nil) {
				return comment, err
			}
		}

		return nil, nil
	}

	meta = make(map[string]interface{})
	node.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if !entering || (node.Type != blackfriday.HTMLBlock) {
			return blackfriday.GoToNext
		}

		hnode, err := html.Parse(bytes.NewReader(node.Literal))
		if err != nil {
			werr = fmt.Errorf("parse HTML: %w", err)
			return blackfriday.Terminate
		}

		comment, err := findComment(hnode)
		if err != nil {
			werr = fmt.Errorf("find comment: %w", err)
			return blackfriday.Terminate
		}
		if !bytes.HasPrefix(comment, []byte("meta")) {
			return blackfriday.SkipChildren
		}

		if comment != nil {
			err = yaml.Unmarshal(comment[4:], &meta)
			if err != nil {
				werr = fmt.Errorf("unmarshal: %w", err)
				return blackfriday.Terminate
			}

			if unlink {
				node.Unlink()
			}

			return blackfriday.Terminate
		}

		return blackfriday.GoToNext
	})

	return meta, werr
}
