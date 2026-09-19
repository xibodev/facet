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
	live := map[string]toolbox.V2Operation{}
	for _, operation := range toolbox.V2Operations() {
		live[operation.ID] = operation
	}
	covered := map[string]string{}
	seenPacks := map[string]bool{}
	for _, method := range Catalog() {
		if seenPacks[method.Pack] {
			continue
		}
		seenPacks[method.Pack] = true
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
				NetworkIntent  string   `json:"networkIntent"`
				CostIntent     string   `json:"costIntent"`
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
			if _, ok := live[operation]; !ok {
				t.Errorf("%s suggests unavailable operation %q", path, operation)
			}
		}
		wantTools := packOperations(method.Pack)
		if !reflect.DeepEqual(manifest.Declarations.SuggestedTools, wantTools) {
			t.Errorf("%s suggestedTools = %v, want route union %v", path, manifest.Declarations.SuggestedTools, wantTools)
		}
		wantNetwork, wantCost := "none", "none"
		var networkRoutes, chargedRoutes int
		for _, candidate := range Catalog() {
			if candidate.Pack != method.Pack {
				continue
			}
			for _, route := range candidate.Routes {
				var network, charged bool
				for _, operationID := range route.Operations {
					operation := live[operationID]
					network = network || operation.Effects.Network
					charged = charged || operation.Effects.MayCharge
				}
				if network {
					networkRoutes++
				}
				if charged {
					chargedRoutes++
				}
			}
		}
		if networkRoutes > 0 {
			wantNetwork = "provider-optional"
		}
		if chargedRoutes > 0 {
			wantCost = "may-charge-optional"
		}
		if manifest.Declarations.NetworkIntent != wantNetwork {
			t.Errorf("%s networkIntent = %q, want %q", path, manifest.Declarations.NetworkIntent, wantNetwork)
		}
		if manifest.Declarations.CostIntent != wantCost {
			t.Errorf("%s costIntent = %q, want %q", path, manifest.Declarations.CostIntent, wantCost)
		}
	}
	for _, method := range Catalog() {
		if covered[method.ID] != method.Pack {
			t.Errorf("method %s is not declared by pack %s", method.ID, method.Pack)
		}
	}
}

func TestCatalogDeclaresEntryRequestsAndConstructibleBindings(t *testing.T) {
	operations := operationMap(toolbox.V2Operations())
	for _, method := range Catalog() {
		for _, route := range method.Routes {
			if route.EntryOperation == "" {
				t.Errorf("%s/%s has no entry operation", method.ID, route.ID)
			}
			if len(route.Operations) == 0 || route.Operations[0] != route.EntryOperation {
				t.Errorf("%s/%s entry %q is not the first operation: %v", method.ID, route.ID, route.EntryOperation, route.Operations)
			}
			consumedInputs := map[string]bool{}
			targeted := map[string]bool{}
			for _, binding := range route.Bindings {
				targeted[binding.ToOperation] = true
				if binding.FromInput != "" {
					consumedInputs[binding.FromInput] = true
				}
				if binding.TargetSemantics == "" {
					t.Errorf("%s/%s binding to %s.%s has no target semantics", method.ID, route.ID, binding.ToOperation, binding.ToParameter)
				}
				if binding.FromOperation != "" {
					source := operations[binding.FromOperation]
					if !routeContains(source.Produces, binding.ArtifactKind) {
						t.Errorf("%s/%s binds %s from %s, which produces %v", method.ID, route.ID, binding.ArtifactKind, binding.FromOperation, source.Produces)
					}
				}
			}
			for _, operation := range route.Operations[1:] {
				if !targeted[operation] {
					t.Errorf("%s/%s has no input or artifact binding for downstream operation %s", method.ID, route.ID, operation)
				}
			}
			for _, input := range route.RequiredInputs {
				switch input.Consumption {
				case InputConsumptionOperation:
					if !consumedInputs[input.Name] {
						t.Errorf("%s/%s consumable input %s has no operation binding", method.ID, route.ID, input.Name)
					}
				case InputConsumptionInformational:
					if input.Name != "consent" && input.Name != "action_plan" {
						t.Errorf("%s/%s executable input %s was classified informational", method.ID, route.ID, input.Name)
					}
					if consumedInputs[input.Name] {
						t.Errorf("%s/%s informational input %s has an operation binding", method.ID, route.ID, input.Name)
					}
				default:
					t.Errorf("%s/%s input %s has unknown consumption %q", method.ID, route.ID, input.Name, input.Consumption)
				}
			}
		}
	}
}

func TestWikimediaDocumentaryCatalogRequiresVideoRequests(t *testing.T) {
	method, ok := Find("documentary-cinematic")
	if !ok {
		t.Fatal("documentary-cinematic method is missing")
	}
	for _, route := range method.Routes {
		if route.ID != "wikimedia-documentary" {
			continue
		}
		want := []RequestConstraint{{
			Operation: "wikimedia",
			Parameter: "kind",
			Equals:    "video",
		}}
		if !reflect.DeepEqual(route.RequestConstraints, want) {
			t.Fatalf("Wikimedia route constraints = %#v, want %#v", route.RequestConstraints, want)
		}
		return
	}
	t.Fatal("wikimedia-documentary route is missing")
}

func TestEntryRequestRequiresCanonicalShapeBeforeEstimate(t *testing.T) {
	methods := []Method{{
		ID: "explainer", Title: "Explainer", Pack: "explainer",
		Routes: []Route{{
			ID:             "local-explainer",
			Title:          "Local explainer",
			EntryOperation: "video_compose",
			Operations:     []string{"video_compose"},
		}},
	}}
	got, err := assess(methods, operationMap([]toolbox.V2Operation{{ID: "video_compose"}}), Request{
		Method: "explainer",
		OperationRequests: map[string]json.RawMessage{
			"video_compose": json.RawMessage(`{"cuts":[{}]}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusConditional {
		t.Fatalf("malformed entry request status = %q, want conditional: %#v", route.Status, route)
	}
	if len(route.InvalidOperationRequests) != 1 || route.InvalidOperationRequests[0].Operation != "video_compose" {
		t.Fatalf("malformed cuts were not rejected by canonical schema: %#v", route.InvalidOperationRequests)
	}
}

func TestWikimediaDocumentaryRequiresExplicitVideoKind(t *testing.T) {
	methods := []Method{{
		ID: "documentary-cinematic", Title: "Documentary", Pack: "cinematic",
		Routes: []Route{
			requireRequestValue(
				route(
					"wikimedia-documentary",
					"Wikimedia documentary",
					"Use Wikimedia video in a documentary.",
					nil,
					"wikimedia",
					[]string{"wikimedia"},
				),
				"wikimedia",
				"kind",
				"video",
			),
		},
	}}
	operations := operationMap([]toolbox.V2Operation{{ID: "wikimedia", Produces: []string{"render_video"}}})
	for _, kind := range []string{"", "image", "any"} {
		name := kind
		if name == "" {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			entryRequest := map[string]any{
				"query":       "ocean",
				"output_path": "stock.bin",
			}
			if kind != "" {
				entryRequest["kind"] = kind
			}
			raw, err := json.Marshal(entryRequest)
			if err != nil {
				t.Fatal(err)
			}
			got, err := assess(methods, operations, Request{
				Method: "documentary-cinematic",
				OperationRequests: map[string]json.RawMessage{
					"wikimedia": raw,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			route := got.Methods[0].Routes[0]
			if route.Status != StatusConditional {
				t.Fatalf("kind %q status = %q, want conditional: %#v", kind, route.Status, route)
			}
			if len(route.InvalidOperationRequests) != 1 {
				t.Fatalf("kind %q did not violate route constraint: %#v", kind, route.InvalidOperationRequests)
			}
		})
	}

	got, err := assess(methods, operations, Request{
		Method: "documentary-cinematic",
		OperationRequests: map[string]json.RawMessage{
			"wikimedia": json.RawMessage(`{"query":"ocean","kind":"video","output_path":"stock.mp4"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusFeasible || len(route.InvalidOperationRequests) != 0 {
		t.Fatalf("explicit video request was not feasible: %#v", route)
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

func TestSymbolicOrMissingFilesNeverMakeRouteFeasible(t *testing.T) {
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
			RequiredInputs: []Input{{Name: "source_media", Kind: "file"}},
			EntryOperation: "source_edit",
			Operations:     []string{"source_edit"},
		}},
	}}
	got, err := assess(methods, operations, Request{
		Method: "source-edit",
		Inputs: map[string]json.RawMessage{"source_media": json.RawMessage(`"missing.mp4"`)},
		OperationRequests: map[string]json.RawMessage{
			"source_edit": sourceEditRequest("missing.mp4", "out.mp4"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status == StatusFeasible {
		t.Fatalf("nonexistent input made route feasible: %#v", route)
	}
	reasons := strings.Join(route.Reasons, " ")
	if !strings.Contains(reasons, "does not exist") || !strings.Contains(reasons, "source_edit request") {
		t.Fatalf("missing concrete evidence is not reported: %v", route.Reasons)
	}
}

func TestRequiredInputMustMatchNestedArrayConsumption(t *testing.T) {
	dir := t.TempDir()
	required := filepath.Join(dir, "required.mp4")
	other := filepath.Join(dir, "other.mp4")
	for _, path := range []string{required, other} {
		if err := os.WriteFile(path, []byte("shape-only"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	methods := []Method{{
		ID: "source-edit", Title: "Source edit", Pack: "cinematic",
		Routes: []Route{{
			ID: "source-edit-local", Title: "Local source edit",
			RequiredInputs: []Input{{
				Name: "source_media", Kind: "file", Consumption: InputConsumptionOperation,
			}},
			EntryOperation: "source_edit",
			Operations:     []string{"source_edit"},
			Bindings: []Binding{{
				FromInput:       "source_media",
				ToOperation:     "source_edit",
				ToParameter:     "segments[].input",
				TargetSemantics: BindingTargetAnyValue,
			}},
		}},
	}}
	request := Request{
		Method: "source-edit",
		Inputs: map[string]json.RawMessage{
			"source_media": json.RawMessage(mustJSONString(t, required)),
		},
		OperationRequests: map[string]json.RawMessage{
			"source_edit": sourceEditRequest(other, filepath.Join(dir, "out.mp4")),
		},
	}
	got, err := assess(methods, operationMap([]toolbox.V2Operation{{ID: "source_edit"}}), request)
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusConditional || route.Bindings[0].Constructible {
		t.Fatalf("unrelated source file made route feasible: %#v", route)
	}

	request.OperationRequests["source_edit"] = sourceEditRequest(required, filepath.Join(dir, "out.mp4"))
	got, err = assess(methods, operationMap([]toolbox.V2Operation{{ID: "source_edit"}}), request)
	if err != nil {
		t.Fatal(err)
	}
	route = got.Methods[0].Routes[0]
	if route.Status != StatusFeasible || !route.Bindings[0].Constructible {
		t.Fatalf("matching nested source binding was not feasible: %#v", route)
	}
}

func TestRequiredInputBindingShapesMatchAndMismatch(t *testing.T) {
	t.Run("exact scalar", func(t *testing.T) {
		methods := []Method{{
			ID: "archive", Title: "Archive", Pack: "cinematic",
			Routes: []Route{{
				ID: "archive-query", Title: "Archive query",
				RequiredInputs: []Input{{
					Name: "research_query", Kind: "text", Consumption: InputConsumptionOperation,
				}},
				EntryOperation: "wikimedia",
				Operations:     []string{"wikimedia"},
				Bindings: []Binding{{
					FromInput:       "research_query",
					ToOperation:     "wikimedia",
					ToParameter:     "query",
					TargetSemantics: BindingTargetExact,
				}},
			}},
		}}
		request := Request{
			Method: "archive",
			Inputs: map[string]json.RawMessage{"research_query": json.RawMessage(`"ocean"`)},
			OperationRequests: map[string]json.RawMessage{
				"wikimedia": json.RawMessage(`{"query":"forest","kind":"video"}`),
			},
		}
		operations := operationMap([]toolbox.V2Operation{{ID: "wikimedia"}})
		got, err := assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		if route := got.Methods[0].Routes[0]; route.Status != StatusConditional || route.Bindings[0].Constructible {
			t.Fatalf("mismatched exact input = %#v", route)
		}
		request.OperationRequests["wikimedia"] = json.RawMessage(`{"query":"ocean","kind":"video"}`)
		got, err = assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		if route := got.Methods[0].Routes[0]; route.Status != StatusFeasible || !route.Bindings[0].Constructible {
			t.Fatalf("matching exact input = %#v", route)
		}
	})

	t.Run("array item", func(t *testing.T) {
		dir := t.TempDir()
		required := filepath.Join(dir, "required.mp4")
		other := filepath.Join(dir, "other.mp4")
		for _, path := range []string{required, other} {
			if err := os.WriteFile(path, []byte("shape-only"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		methods := []Method{{
			ID: "documentary", Title: "Documentary", Pack: "cinematic",
			Routes: []Route{{
				ID: "source-documentary", Title: "Source documentary",
				RequiredInputs: []Input{{
					Name: "source_media", Kind: "file", Consumption: InputConsumptionOperation,
				}},
				EntryOperation: "video_stitch",
				Operations:     []string{"video_stitch"},
				Bindings: []Binding{{
					FromInput:       "source_media",
					ToOperation:     "video_stitch",
					ToParameter:     "clips",
					TargetSemantics: BindingTargetArrayItem,
				}},
			}},
		}}
		request := Request{
			Method: "documentary",
			Inputs: map[string]json.RawMessage{
				"source_media": json.RawMessage(mustJSONString(t, required)),
			},
			OperationRequests: map[string]json.RawMessage{
				"video_stitch": stitchRequest(other),
			},
		}
		operations := operationMap([]toolbox.V2Operation{{ID: "video_stitch"}})
		got, err := assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		if route := got.Methods[0].Routes[0]; route.Status != StatusConditional || route.Bindings[0].Constructible {
			t.Fatalf("mismatched array item = %#v", route)
		}
		request.OperationRequests["video_stitch"] = stitchRequest(required)
		got, err = assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		if route := got.Methods[0].Routes[0]; route.Status != StatusFeasible || !route.Bindings[0].Constructible {
			t.Fatalf("matching array item = %#v", route)
		}
	})

	t.Run("all media and text sequence", func(t *testing.T) {
		dir := t.TempDir()
		first := filepath.Join(dir, "first.png")
		second := filepath.Join(dir, "second.png")
		for _, path := range []string{first, second} {
			if err := os.WriteFile(path, []byte("shape-only"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		methods := []Method{{
			ID: "explainer", Title: "Explainer", Pack: "explainer",
			Routes: []Route{{
				ID: "local-explainer", Title: "Local explainer",
				RequiredInputs: []Input{
					{Name: "script", Kind: "text", Consumption: InputConsumptionOperation},
					{Name: "visuals", Kind: "files", Consumption: InputConsumptionOperation},
				},
				EntryOperation: "video_compose",
				Operations:     []string{"video_compose"},
				Bindings: []Binding{
					{FromInput: "script", ToOperation: "video_compose", ToParameter: "cuts[].text", TargetSemantics: BindingTargetTextSequence},
					{FromInput: "visuals", ToOperation: "video_compose", ToParameter: "cuts[].source", TargetSemantics: BindingTargetAllValues},
				},
			}},
		}}
		request := Request{
			Method: "explainer",
			Inputs: map[string]json.RawMessage{
				"script":  json.RawMessage(`"hello world"`),
				"visuals": json.RawMessage(`[` + mustJSONString(t, first) + `,` + mustJSONString(t, second) + `]`),
			},
			OperationRequests: map[string]json.RawMessage{
				"video_compose": composeRequestWithAssets("hello", []string{first}),
			},
		}
		operations := operationMap([]toolbox.V2Operation{{ID: "video_compose"}})
		got, err := assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		route := got.Methods[0].Routes[0]
		if route.Status != StatusConditional || route.Bindings[0].Constructible || route.Bindings[1].Constructible {
			t.Fatalf("partial text/assets made route feasible: %#v", route)
		}

		request.OperationRequests["video_compose"] = composeRequestWithAssets("hello world", []string{first, second})
		got, err = assess(methods, operations, request)
		if err != nil {
			t.Fatal(err)
		}
		route = got.Methods[0].Routes[0]
		if route.Status != StatusFeasible || !route.Bindings[0].Constructible || !route.Bindings[1].Constructible {
			t.Fatalf("matching text/assets were not feasible: %#v", route)
		}
	})

	t.Run("nested exact", func(t *testing.T) {
		binding := Binding{
			FromInput:       "music",
			ToOperation:     "audio_mix",
			ToParameter:     "music.input",
			TargetSemantics: BindingTargetExact,
		}
		source := json.RawMessage(`"song.wav"`)
		if bindingValuesMatch(binding, source, json.RawMessage(`{"music":{"input":"other.wav"}}`)) {
			t.Fatal("mismatched nested exact value matched")
		}
		if !bindingValuesMatch(binding, source, json.RawMessage(`{"music":{"input":"song.wav"}}`)) {
			t.Fatal("matching nested exact value did not match")
		}
	})
}

func TestRealFileAndCanonicalEntryRequestCanBeFeasible(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(source, []byte("shape-only"), 0600); err != nil {
		t.Fatal(err)
	}
	operations := operationMap([]toolbox.V2Operation{{
		ID: "source_edit",
		Requirements: []toolbox.V2Requirement{{
			Name: "ffmpeg", Kind: "binary", Strength: "mandatory",
			Resolution: toolbox.ResolutionSatisfied,
		}},
		Produces: []string{"render_video"},
	}})
	methods := []Method{{
		ID: "source-edit", Title: "Source edit", Pack: "cinematic",
		Routes: []Route{{
			ID: "source-edit-local", Title: "Local source edit",
			RequiredInputs: []Input{{Name: "source_media", Kind: "file"}},
			EntryOperation: "source_edit",
			Operations:     []string{"source_edit"},
		}},
	}}
	got, err := assess(methods, operations, Request{
		Method: "source-edit",
		Inputs: map[string]json.RawMessage{"source_media": json.RawMessage(`"` + filepath.ToSlash(source) + `"`)},
		OperationRequests: map[string]json.RawMessage{
			"source_edit": sourceEditRequest(source, filepath.Join(dir, "out.mp4")),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusFeasible || len(route.Reasons) != 0 {
		t.Fatalf("real canonical request = %#v", route)
	}
}

func TestDownstreamRequestsAndBindingsRemainConditionalUntilSupplied(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(source, []byte("shape-only"), 0600); err != nil {
		t.Fatal(err)
	}
	operations := operationMap([]toolbox.V2Operation{
		{ID: "source_edit", Produces: []string{"render_video"}},
		{ID: "output_review"},
	})
	methods := []Method{{
		ID: "source-edit", Title: "Source edit", Pack: "cinematic",
		Routes: []Route{{
			ID: "source-edit-local", Title: "Local source edit",
			RequiredInputs: []Input{{Name: "source_media", Kind: "file"}},
			EntryOperation: "source_edit",
			Operations:     []string{"source_edit", "output_review"},
			Bindings: []Binding{artifact(
				"source_edit", "output", "render_video", "output_review", "input",
			)},
		}},
	}}
	got, err := assess(methods, operations, Request{
		Method: "source-edit",
		Inputs: map[string]json.RawMessage{"source_media": json.RawMessage(`"` + filepath.ToSlash(source) + `"`)},
		OperationRequests: map[string]json.RawMessage{
			"source_edit": sourceEditRequest(source, filepath.Join(dir, "out.mp4")),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusConditional {
		t.Fatalf("missing downstream request = %#v", route)
	}
	if !reflect.DeepEqual(route.MissingOperationRequests, []string{"output_review"}) {
		t.Fatalf("missing operation requests = %v", route.MissingOperationRequests)
	}
	if len(route.Bindings) != 1 || route.Bindings[0].Constructible {
		t.Fatalf("binding should not be constructible: %#v", route.Bindings)
	}

	request := map[string]json.RawMessage{
		"source_edit":   sourceEditRequest(source, filepath.Join(dir, "out.mp4")),
		"output_review": json.RawMessage(`{"input":"different.mp4"}`),
	}
	got, err = assess(methods, operations, Request{
		Method:            "source-edit",
		Inputs:            map[string]json.RawMessage{"source_media": json.RawMessage(`"` + filepath.ToSlash(source) + `"`)},
		OperationRequests: request,
	})
	if err != nil {
		t.Fatal(err)
	}
	route = got.Methods[0].Routes[0]
	if route.Status != StatusConditional || route.Bindings[0].Constructible {
		t.Fatalf("mismatched artifact binding became feasible: %#v", route)
	}
}

func TestVideoComposeArtifactMustBeAMediaCutSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "clip.mp4")
	output := filepath.Join(dir, "edited.mp4")
	if err := os.WriteFile(source, []byte("shape-only"), 0600); err != nil {
		t.Fatal(err)
	}
	operations := operationMap([]toolbox.V2Operation{
		{ID: "source_edit", Produces: []string{"render_video"}},
		{ID: "video_compose"},
	})
	methods := []Method{{
		ID: "animation", Title: "Animation", Pack: "explainer",
		Routes: []Route{{
			ID:             "source-animation",
			Title:          "Source animation",
			EntryOperation: "source_edit",
			Operations:     []string{"source_edit", "video_compose"},
			Bindings: []Binding{mediaCutArtifact(
				"source_edit", "output", "render_video", "video_compose", "cuts",
			)},
		}},
	}}
	request := Request{
		Method: "animation",
		OperationRequests: map[string]json.RawMessage{
			"source_edit": sourceEditRequest(source, output),
			"video_compose": json.RawMessage(`{"cuts":[{
				"type":"text_card","text":` + mustJSONString(t, output) + `,
				"in_seconds":0,"out_seconds":1
			}]}`),
		},
	}
	got, err := assess(methods, operations, request)
	if err != nil {
		t.Fatal(err)
	}
	route := got.Methods[0].Routes[0]
	if route.Status != StatusConditional || route.Bindings[0].Constructible {
		t.Fatalf("unrelated text reference satisfied media binding: %#v", route)
	}

	request.OperationRequests["video_compose"] = json.RawMessage(`{"cuts":[{
		"type":"media","source":` + mustJSONString(t, output) + `,"media_kind":"video",
		"in_seconds":0,"out_seconds":1
	}]}`)
	got, err = assess(methods, operations, request)
	if err != nil {
		t.Fatal(err)
	}
	route = got.Methods[0].Routes[0]
	if route.Status != StatusFeasible || !route.Bindings[0].Constructible {
		t.Fatalf("media cut source did not satisfy binding: %#v", route)
	}
}

func TestVideoComposeMediaBindingRequiresMatchingArtifactKind(t *testing.T) {
	binding := mediaCutArtifact("image_generator", "output_path", "image", "video_compose", "cuts")
	source := json.RawMessage(`"generated.png"`)
	videoCut := json.RawMessage(`{"cuts":[{
		"type":"media","source":"generated.png","media_kind":"video",
		"in_seconds":0,"out_seconds":1
	}]}`)
	if bindingValuesMatch(binding, source, videoCut) {
		t.Fatal("image artifact matched a video media cut")
	}
	imageCut := json.RawMessage(`{"cuts":[{
		"type":"media","source":"generated.png","media_kind":"image",
		"in_seconds":0,"out_seconds":1
	}]}`)
	if !bindingValuesMatch(binding, source, imageCut) {
		t.Fatal("image artifact did not match an image media cut source")
	}
}

func sourceEditRequest(input, output string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"segments": []map[string]any{{"input": input, "start": 0, "end": 1}},
		"target": map[string]any{
			"width": 1920, "height": 1080, "fps": 30, "fit": "contain",
			"video_codec": "h264", "pixel_format": "yuv420p",
			"audio_codec": "aac", "audio_sample_rate": 48000, "audio_channels": 2,
		},
		"output": output,
	})
	return raw
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func composeRequestWithAssets(text string, assets []string) json.RawMessage {
	cuts := []map[string]any{{
		"type": "text_card", "text": text, "in_seconds": 0, "out_seconds": 1,
	}}
	for index, asset := range assets {
		cuts = append(cuts, map[string]any{
			"type": "media", "source": asset, "media_kind": "image",
			"in_seconds": float64(index + 1), "out_seconds": float64(index + 2),
		})
	}
	raw, _ := json.Marshal(map[string]any{"cuts": cuts})
	return raw
}

func stitchRequest(clip string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"operation": "validate",
		"clips":     []string{clip},
	})
	return raw
}

func routeContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
