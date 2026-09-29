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
//go:embed skills packs agents schemas/tools
var Assets embed.FS

type GuidanceAsset struct {
	ID      string
	Title   string
	Summary string
	Path    string
}

type Pack struct {
	ID       string
	Title    string
	Summary  string
	Guidance []GuidanceAsset
}

var retainedPacks = []Pack{
	{
		ID: "character-animation", Title: "2D Character Animation",
		Summary: "Produce character-led 2D animation from supplied or licensed assets with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-character-animation", Title: "2D character animation",
			Summary: "Character-led 2D animation using supplied or licensed assets.",
			Path:    "packs/character-animation/SKILL.md",
		}},
	},
	{
		ID: "cinematic", Title: "Cinematic & Documentary",
		Summary: "Produce source-led documentary, montage, and cinematic edits with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-cinematic", Title: "Cinematic and documentary editing",
			Summary: "Source-led documentary, montage, and cinematic editing guidance.",
			Path:    "packs/cinematic/SKILL.md",
		}},
	},
	{
		ID: "explainer", Title: "Animated Explainer",
		Summary: "Produce reviewed 2D explainers and motion graphics with Facet and Remotion.",
		Guidance: []GuidanceAsset{
			{
				ID: "facet-explainer", Title: "Animated explainer",
				Summary: "Entry guidance for reviewed 2D explainers and motion graphics.",
				Path:    "packs/explainer/SKILL.md",
			},
			{
				ID: "facet-explainer-walkthrough", Title: "Narrated explainer, end to end",
				Summary: "Narration-first production order, timing, rendering, and verification.",
				Path:    "packs/explainer/NARRATED-WALKTHROUGH.md",
			},
			{
				ID: "facet-explainer-scene-types", Title: "Explainer scene types",
				Summary: "Supported Explainer scene primitives and their required fields.",
				Path:    "packs/explainer/SCENE-TYPES.md",
			},
		},
	},
	{
		ID: "localization", Title: "Video Localization & Dubbing",
		Summary: "Produce translated subtitles, narration, and localized video variants with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-localization", Title: "Video localization and dubbing",
			Summary: "Translated subtitles, narration, and localized video variants.",
			Path:    "packs/localization/SKILL.md",
		}},
	},
	{
		ID: "screen-demo", Title: "Screen Demo & Walkthrough",
		Summary: "Produce recorded or synthetic software walkthroughs with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-screen-demo", Title: "Screen demo and walkthrough",
			Summary: "Recorded or synthetic software demonstration guidance.",
			Path:    "packs/screen-demo/SKILL.md",
		}},
	},
	{
		ID: "social", Title: "Social & Short-Form",
		Summary: "Repurpose source video into reviewed short-form and vertical edits with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-social", Title: "Social and short-form editing",
			Summary: "Source repurposing for reviewed short-form and vertical edits.",
			Path:    "packs/social/SKILL.md",
		}},
	},
	{
		ID: "talking-head", Title: "Talking Head & Avatar",
		Summary: "Produce presenter-led edits and consented avatar videos with Facet.",
		Guidance: []GuidanceAsset{{
			ID: "facet-talking-head", Title: "Talking head and avatar video",
			Summary: "Presenter-led editing and consented avatar-video guidance.",
			Path:    "packs/talking-head/SKILL.md",
		}},
	},
}

func RetainedPacks() []Pack {
	out := make([]Pack, len(retainedPacks))
	for i, pack := range retainedPacks {
		out[i] = pack
		out[i].Guidance = append([]GuidanceAsset(nil), pack.Guidance...)
	}
	return out
}

func Guidance(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !fs.ValidPath(name) || !(strings.HasPrefix(name, "skills/") || strings.HasPrefix(name, "packs/") || strings.HasPrefix(name, "agents/") || strings.HasPrefix(name, "schemas/")) || !(path.Ext(name) == ".md" || path.Ext(name) == ".json") {
		return "", fmt.Errorf("invalid Facet guidance path %q", name)
	}
	data, err := Assets.ReadFile(name)
	return string(data), err
}

func PackNames() []string {
	packs := RetainedPacks()
	names := make([]string, 0, len(packs))
	for _, pack := range packs {
		names = append(names, pack.ID)
	}
	sort.Strings(names)
	return names
}
