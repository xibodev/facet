// Package web holds the embedded assets for Facet's experimental standalone
// Facet UI. The installed agent bundle is the supported product path, and the
// application must project the same stateless Facet contract.
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
