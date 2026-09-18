// Package facet contains the canonical creative assets shared by Facet hosts.
package facet

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Assets is compiled from the same files used by the CLI bundle and module.
// It is independent of the executable's working directory.
//
//go:embed skills packs agents schemas
var Assets embed.FS

func Guidance(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !fs.ValidPath(name) || !(strings.HasPrefix(name, "skills/") || strings.HasPrefix(name, "packs/") || strings.HasPrefix(name, "agents/") || strings.HasPrefix(name, "schemas/")) || !(path.Ext(name) == ".md" || path.Ext(name) == ".json") {
		return "", fmt.Errorf("invalid Facet guidance path %q", name)
	}
	data, err := Assets.ReadFile(name)
	return string(data), err
}

func PackNames() []string {
	entries, _ := Assets.ReadDir("packs")
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := Assets.ReadFile("packs/" + entry.Name() + "/SKILL.md"); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Strings(names)
	return names
}
