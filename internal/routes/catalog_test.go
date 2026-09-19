package routes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xibodev/facet/internal/toolbox"
)

func TestCatalogIsStableAndReferencesLiveOperations(t *testing.T) {
	first := Catalog()
	second := Catalog()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("catalog output changed between identical reads")
	}

	rawFirst, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	rawSecond, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(rawFirst) != string(rawSecond) {
		t.Fatal("catalog JSON is not byte-stable")
	}

	wantMethods := []string{
		"animation",
		"avatar",
		"character-animation",
		"content-repurpose",
		"documentary-cinematic",
		"explainer",
		"localization",
		"music-led",
		"product-demo",
		"source-edit",
	}
	gotMethods := make([]string, 0, len(first))
	live := map[string]bool{}
	for _, op := range toolbox.V2Operations() {
		live[op.ID] = true
	}
	for _, method := range first {
		gotMethods = append(gotMethods, method.ID)
		if method.Pack == "" {
			t.Errorf("%s has no retained pack", method.ID)
		}
		if len(method.Routes) == 0 {
			t.Errorf("%s has no production routes", method.ID)
		}
		for _, route := range method.Routes {
			for _, operation := range route.Operations {
				if !live[operation] {
					t.Errorf("%s/%s references unavailable operation %q", method.ID, route.ID, operation)
				}
			}
		}
	}
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Fatalf("method order = %v, want %v", gotMethods, wantMethods)
	}
}

func TestRetainedPackMetadataCoversCatalogAndUsesLiveOperations(t *testing.T) {
	live := map[string]bool{}
	for _, operation := range toolbox.V2Operations() {
		live[operation.ID] = true
	}
	covered := map[string]string{}
	for _, method := range Catalog() {
		path := filepath.Join("..", "..", "packs", method.Pack, "facet-pack.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s pack metadata: %v", method.ID, err)
			continue
		}
		var manifest struct {
			Methods      []string `json:"methods"`
			Declarations struct {
				SuggestedTools []string `json:"suggestedTools"`
			} `json:"declarations"`
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		for _, id := range manifest.Methods {
			if previous := covered[id]; previous != "" && previous != method.Pack {
				t.Errorf("method %s is declared by both %s and %s", id, previous, method.Pack)
			}
			covered[id] = method.Pack
		}
		for _, operation := range manifest.Declarations.SuggestedTools {
			if !live[operation] {
				t.Errorf("%s suggests unavailable operation %q", path, operation)
			}
		}
	}
	for _, method := range Catalog() {
		if covered[method.ID] != method.Pack {
			t.Errorf("method %s is not declared by pack %s", method.ID, method.Pack)
		}
	}
}

func TestAssessmentReportsMissingInputsAndDependencies(t *testing.T) {
	operations := operationMap([]toolbox.V2Operation{
		{
			ID: "source_edit",
			Requirements: []toolbox.V2Requirement{{
				Name: "ffmpeg", Kind: "binary", Strength: "mandatory",
				Resolution: toolbox.ResolutionUnsatisfied,
			}},
		},
		{ID: "output_review"},
	})
	methods := []Method{{
		ID: "source-edit", Title: "Source edit", Pack: "cinematic",
		Routes: []Route{{
			ID: "source-edit-local", Title: "Local source edit",
			RequiredInputs: []Input{{Name: "source_media", Description: "Supplied footage"}},
			Operations:     []string{"source_edit", "output_review"},
		}},
	}}

	got, err := assess(methods, operations, Request{Method: "source-edit"})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusUnavailable {
		t.Fatalf("status = %q, want %q: %#v", route.Status, StatusUnavailable, route)
	}
	if !reflect.DeepEqual(route.MissingInputs, []string{"source_media"}) {
		t.Fatalf("missing inputs = %v", route.MissingInputs)
	}
	if len(route.MissingDependencies) != 1 || route.MissingDependencies[0].Name != "ffmpeg" {
		t.Fatalf("missing dependencies = %#v", route.MissingDependencies)
	}
	reasons := strings.Join(route.Reasons, " ")
	if !strings.Contains(reasons, "source_media") || !strings.Contains(reasons, "ffmpeg") {
		t.Fatalf("reasons do not name both blockers: %v", route.Reasons)
	}
}

func TestAssessmentDoesNotAutoSelectProviders(t *testing.T) {
	allow := true
	inputs := map[string]json.RawMessage{
		"script":  json.RawMessage(`"approved script"`),
		"visuals": json.RawMessage(`["shot"]`),
	}
	got, err := Assess(Request{
		Method:       "explainer",
		Inputs:       inputs,
		AllowNetwork: &allow,
		AllowCharges: &allow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.AutoSelectProviders {
		t.Fatal("assessment claims it auto-selects providers")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "selected_provider") {
		t.Fatalf("assessment selected a provider: %s", raw)
	}
	for _, method := range got.Methods {
		for _, route := range method.Routes {
			for _, operation := range route.Operations {
				if operation.ID == "image_selector" || operation.ID == "video_selector" {
					t.Errorf("route %s delegates provider choice to %s", route.ID, operation.ID)
				}
			}
		}
	}
}

func TestUnknownCredentialAndUnapprovedEffectsAreConditional(t *testing.T) {
	operations := operationMap([]toolbox.V2Operation{{
		ID: "openai_image",
		Effects: toolbox.V2Effects{
			Network: true, MayCharge: true,
		},
		Requirements: []toolbox.V2Requirement{{
			Name: "OPENAI_API_KEY", Kind: "env", Strength: "mandatory",
			Resolution: toolbox.ResolutionUnknown,
		}},
	}})
	methods := []Method{{
		ID: "animation", Title: "Animation", Pack: "explainer",
		Routes: []Route{{
			ID: "openai-animation", Title: "OpenAI animation",
			Operations: []string{"openai_image"},
		}},
	}}

	got, err := assess(methods, operations, Request{Method: "animation"})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusConditional {
		t.Fatalf("status = %q, want conditional: %#v", route.Status, route)
	}
	reasons := strings.Join(route.Reasons, " ")
	for _, want := range []string{"OPENAI_API_KEY", "network", "charge"} {
		if !strings.Contains(reasons, want) {
			t.Errorf("reasons %q do not mention %q", reasons, want)
		}
	}
}

func TestSatisfiedLocalRouteIsFeasible(t *testing.T) {
	operations := operationMap([]toolbox.V2Operation{{
		ID: "source_edit",
		Requirements: []toolbox.V2Requirement{{
			Name: "ffmpeg", Kind: "binary", Strength: "mandatory",
			Resolution: toolbox.ResolutionSatisfied,
		}},
	}})
	methods := []Method{{
		ID: "source-edit", Title: "Source edit", Pack: "cinematic",
		Routes: []Route{{
			ID: "source-edit-local", Title: "Local source edit",
			RequiredInputs: []Input{{Name: "source_media"}},
			Operations:     []string{"source_edit"},
		}},
	}}
	got, err := assess(methods, operations, Request{
		Method: "source-edit",
		Inputs: map[string]json.RawMessage{"source_media": json.RawMessage(`"clip.mp4"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusFeasible || len(route.Reasons) != 0 {
		t.Fatalf("satisfied local route = %#v", route)
	}
}
