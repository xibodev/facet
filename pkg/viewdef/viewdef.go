// Package viewdef provides the product-owned view definition for Facet.
//
// Identical definitions are consumed by Facet standalone (F-APP) and
// the Facet Studio module (F-MOD), ensuring byte-for-byte digest parity.
package viewdef

import "github.com/xibodev/facet-studio/pkg/view"

const (
	FacetViewID = "facet.video_workbench"
	Version     = "1.0.0"
)

// FacetWorkbenchView returns the canonical declarative workbench view for Facet.
func FacetWorkbenchView() *view.ViewDefinition {
	return &view.ViewDefinition{
		ID:             FacetViewID,
		SchemaVersion:  Version,
		Title:          "Facet Video Workbench",
		Module:         "facet",
		ArtifactSchema: "xibodev.facet.render/v1",
		Sections: []view.SectionDefinition{
			{
				ID:        "video_preview",
				Title:     "Video Preview",
				Type:      "tab",
				Primitive: view.Video,
				Binding:   "artifacts.rendered_video",
				EmptyText: "No video rendered yet. Run a production script or compose scenes.",
			},
			{
				ID:        "narration_audio",
				Title:     "Narration & Audio",
				Type:      "tab",
				Primitive: view.Audio,
				Binding:   "artifacts.narration_audio",
				EmptyText: "No narration synthesized yet.",
			},
			{
				ID:        "script_plan",
				Title:     "Script & Scenes",
				Type:      "tab",
				Primitive: view.Markdown,
				Binding:   "artifacts.script",
				EmptyText: "No script drafted yet.",
			},
			{
				ID:        "qa_frames",
				Title:     "QA Frame Samples",
				Type:      "tab",
				Primitive: view.ImageGrid,
				Binding:   "artifacts.qa_samples",
				EmptyText: "No sampled frames available.",
			},
		},
		Actions: []view.ActionDefinition{
			{
				ID:            "rerender",
				Label:         "Render / Re-render Video",
				Tool:          "video_compose",
				DisabledWhen:  "rendering",
			},
			{
				ID:            "renarrate",
				Label:         "Synthesize Narration",
				Tool:          "edge_tts",
			},
			{
				ID:            "trim_cut",
				Label:         "Trim / Cut Footage",
				Tool:          "source_edit",
			},
			{
				ID:               "approve_render",
				Label:            "Approve Video",
				Tool:             "output_review",
				RequiresApproval: true,
			},
		},
	}
}
