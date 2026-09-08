// Package web holds the embedded assets for Facet's standalone Studio UI.
//
// FROZEN: working and supported, but not under development. Bug and security
// fixes only; no new UI features. Facet's role is a headless creative toolbox,
// and presentation of its artefacts is moving to the agentic host that invokes
// the module surface. See FROZEN.md in this directory.
//
// Nothing here is shared with internal/module, so the module protocol is
// unaffected by this freeze.
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
