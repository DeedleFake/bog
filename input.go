package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/html"
)

func getMeta(node *blackfriday.Node) (meta map[string]interface{}, err error) {
	var findHTML func(*blackfriday.Node) (map[string]interface{}, error)
	var findComment func(*html.Node) (comment []byte, err error)

	findHTML = func(node *blackfriday.Node) (meta map[string]interface{}, err error) {
		if node.Type == blackfriday.HTMLBlock {
			hnode, err := html.Parse(bytes.NewReader(node.Literal))
			if err != nil {
				return nil, fmt.Errorf("parse HTML: %w", err)
			}

			comment, err := findComment(hnode)
			if err != nil {
				return nil, fmt.Errorf("find comment: %w", err)
			}

			if comment != nil {
				var meta map[string]interface{}
				err = json.Unmarshal(comment, &meta)
				if err == nil {
					return meta, nil
				}
			}
		}

		for node := node.FirstChild; node != nil; node = node.Next {
			meta, err := findHTML(node)
			if (meta != nil) || (err != nil) {
				return meta, err
			}
		}

		return nil, nil
	}

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

	meta, err = findHTML(node)
	if err != nil {
		return nil, err
	}

	if meta == nil {
		meta = make(map[string]interface{})
	}

	return meta, nil
}
