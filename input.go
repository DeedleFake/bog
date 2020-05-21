package main

import (
	"bytes"
	"fmt"

	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

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
