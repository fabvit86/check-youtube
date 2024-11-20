package web

import "embed"

//go:embed static
var StaticContent embed.FS

//go:embed template/htmlTemplate.tmpl
var HtmlTemplate []byte
