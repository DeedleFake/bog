package main

// Default templates.
const (
	defaultPage = `<!DOCTYPE html>
<html>
	<head>
		<meta name="generator" content="bog" />
		{{with .Meta.author}}<meta name="author" content={{. | printf "%q"}} />{{end}}
		{{with .Meta.desc}}<meta name="description" content={{. | printf "%q"}} />{{end}}

		<title>{{.Meta.title}}{{with .Data.title}} - {{.}}{{end}}</title>
	</head>
	<body>
		{{.Content}}
	</body>
</html>`

	defaultIndex = `<!DOCTYPE html>
<html>
	<head>
		<meta name="generator" content="bog" />

		<title>Index{{with .Data.title}} - {{.}}{{end}}</title>
	<head>
	<body>
		{{range .Pages -}}
			<div>
				<a href={{.Meta.title | link_to_title | printf "%q"}}>
					{{- .Meta.title}} ({{.DstTime.Format "2006-01-02"}}){{"" -}}
				</a>
			</div>
		{{end}}
	</body>
</html>`
)
