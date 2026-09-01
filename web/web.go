package web

import "embed"

// Content holds the embedded static web assets.
//
//go:embed index.html
var Content embed.FS

// IndexHTML contains the raw bytes of index.html.
//
//go:embed index.html
var IndexHTML []byte
