package main

const (
	defaultPage = `<!DOCTYPE html>
<html>
	<head>
		<title>{{.Meta.title}}{{with .Data.title}} - {{.}}{{end}}</title>
	</head>
	<body>
		{{.Content}}
	</body>
</html>`

	defaultIndex = `<!DOCTYPE html>
<html>
	<head>
		<title>Index{{with .Data.title}} - {{.}}{{end}}</title>
	<head>
	<body>
		{{range .Pages -}}
			<div>{{.Meta.title}} ({{.DstInfo.ModTime.Format "2006-01-02"}})</div>
		{{end}}
	</body>
</html>`
)
