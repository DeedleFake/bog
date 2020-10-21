bog
===

bog is an ultra-simplistic static blog generator. It only converts articles from markdown to HTML, optionally with a template, and it generates a table of contents `index.html`, also optionally with a template. That's it. Nothing fancy.

Usage
-----

```
Usage: bog [options] [source directory]

Options:
  -data string
    	path to optional YAML data file
  -extras value
    	comma-seperated template:output pairs of extra files to render
  -genindex
    	generate an index (default true)
  -hlstyle string
    	chroma syntax highlighting style (default "monokai")
  -index string
    	if not blank, path to index template
  -out string
    	output directory, or source directory if blank
  -page string
    	if not blank, path to page template
```
