package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/xibodev/facet/internal/toolbox"
)

type Assessment struct {
	Methods             []MethodAssessment `json:"methods"`
	Executes            bool               `json:"executes"`
	PersistsWorkflow    bool               `json:"persists_workflow_state"`
	AutoSelectProviders bool               `json:"auto_select_providers"`
}

type MethodAssessment struct {
	ID      string            `json:"id"`
	Title   string            `json:"title"`
	Summary string            `json:"summary"`
	Pack    string            `json:"pack"`
	Routes  []RouteAssessment `json:"routes"`
}

type RouteAssessment struct {
	ID                       string                  `json:"id"`
	Title                    string                  `json:"title"`
	Summary                  string                  `json:"summary"`
	Status                   string                  `json:"status"`
	RequiredInputs           []Input                 `json:"required_inputs"`
	MissingInputs            []string                `json:"missing_inputs"`
	InvalidInputs            []string                `json:"invalid_inputs"`
	MissingOperationRequests []string                `json:"missing_operation_requests"`
	InvalidOperationRequests []OperationRequestIssue `json:"invalid_operation_requests"`
	Operations               []OperationAssessment   `json:"operations"`
	Bindings                 []BindingAssessment     `json:"bindings"`
	MissingDependencies      []DependencyCondition   `json:"missing_dependencies"`
	UnverifiedDependencies   []DependencyCondition   `json:"unverified_dependencies"`
	Network                  bool                    `json:"network"`
	MayCharge                bool                    `json:"may_charge"`
	Reasons                  []string                `json:"reasons"`
}

type OperationAssessment struct {
	ID            string                  `json:"id"`
	Title         string                  `json:"title"`
	Effects       toolbox.V2Effects       `json:"effects"`
	Requirements  []toolbox.V2Requirement `json:"requirements"`
	RequestStatus string                  `json:"request_status"`
}

type OperationRequestIssue struct {
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

type BindingAssessment struct {
	Binding       Binding `json:"binding"`
	Constructible bool    `json:"constructible"`
	Reason        string  `json:"reason,omitempty"`
}

type DependencyCondition struct {
	Operation  string `json:"operation"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Strength   string `json:"strength"`
	Resolution string `json:"resolution"`
}

func Assess(request Request) (Assessment, error) {
	if err := ValidateCatalog(); err != nil {
		return Assessment{}, err
	}
	return assess(Catalog(), operationMap(toolbox.V2Operations()), request)
}

func Describe(methodID string) (MethodAssessment, error) {
	got, err := Assess(Request{Method: methodID})
	if err != nil {
		return MethodAssessment{}, err
	}
	return got.Methods[0], nil
}

func operationMap(operations []toolbox.V2Operation) map[string]toolbox.V2Operation {
	out := make(map[string]toolbox.V2Operation, len(operations))
	for _, operation := range operations {
		out[operation.ID] = operation
	}
	return out
}

func assess(methods []Method, operations map[string]toolbox.V2Operation, request Request) (Assessment, error) {
	wanted := request.Methods
	if strings.TrimSpace(request.Method) != "" {
		wanted = append([]string{request.Method}, wanted...)
	}
	filter := map[string]bool{}
	for _, id := range wanted {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" {
			filter[id] = true
		}
	}
	if len(filter) != 0 {
		known := map[string]bool{}
		for _, method := range methods {
			known[method.ID] = true
		}
		var unknown []string
		for id := range filter {
			if !known[id] {
				unknown = append(unknown, id)
			}
		}
		if len(unknown) != 0 {
			sort.Strings(unknown)
			return Assessment{}, fmt.Errorf("unknown method: %s", strings.Join(unknown, ", "))
		}
	}

	result := Assessment{
		Methods:  []MethodAssessment{},
		Executes: false, PersistsWorkflow: false, AutoSelectProviders: false,
	}
	for _, method := range methods {
		if len(filter) != 0 && !filter[method.ID] {
			continue
		}
		current := MethodAssessment{
			ID: method.ID, Title: method.Title, Summary: method.Summary,
			Pack: method.Pack, Routes: []RouteAssessment{},
		}
		for _, route := range method.Routes {
			current.Routes = append(current.Routes, assessRoute(route, operations, request))
		}
		result.Methods = append(result.Methods, current)
	}
	return result, nil
}

func assessRoute(route Route, operations map[string]toolbox.V2Operation, request Request) RouteAssessment {
	result := RouteAssessment{
		ID: route.ID, Title: route.Title, Summary: route.Summary,
		Status: StatusFeasible, RequiredInputs: route.RequiredInputs,
		MissingInputs: []string{}, InvalidInputs: []string{},
		MissingOperationRequests: []string{}, InvalidOperationRequests: []OperationRequestIssue{},
		Operations: []OperationAssessment{}, Bindings: []BindingAssessment{},
		MissingDependencies:    []DependencyCondition{},
		UnverifiedDependencies: []DependencyCondition{},
		Reasons:                []string{},
	}

	inputReady := map[string]bool{}
	for _, input := range route.RequiredInputs {
		value, ok := request.Inputs[input.Name]
		if !ok || missingValue(value) {
			result.MissingInputs = append(result.MissingInputs, input.Name)
			result.Reasons = append(result.Reasons, "required input "+input.Name+" is not supplied")
			makeConditional(&result)
			continue
		}
		if reason := validateInput(input, value); reason != "" {
			result.InvalidInputs = append(result.InvalidInputs, input.Name)
			result.Reasons = append(result.Reasons, "required input "+input.Name+" "+reason)
			makeConditional(&result)
			continue
		}
		inputReady[input.Name] = true
	}

	operationReady := map[string]bool{}
	operationRequests := map[string]json.RawMessage{}
	for _, operationID := range route.Operations {
		operation, ok := operations[operationID]
		if !ok {
			result.Reasons = append(result.Reasons, "canonical operation "+operationID+" is unavailable")
			result.Status = StatusUnavailable
			continue
		}
		requestStatus := "missing"
		operationRequest, hasRequest := request.OperationRequests[route.ID+":"+operation.ID]
		if !hasRequest {
			operationRequest, hasRequest = request.OperationRequests[operation.ID]
		}
		if !hasRequest || missingValue(operationRequest) {
			result.MissingOperationRequests = append(result.MissingOperationRequests, operation.ID)
			result.Reasons = append(result.Reasons, "canonical "+operation.ID+" request is not supplied")
			makeConditional(&result)
		} else {
			operationRequests[operation.ID] = operationRequest
			var err error
			if operation.ID == route.EntryOperation {
				err = toolbox.ValidateRequestShape(operation.ID, operationRequest)
				if err == nil {
					err = toolbox.ValidateRequest(operation.ID, operationRequest)
				}
			} else {
				err = toolbox.ValidateRequestShape(operation.ID, operationRequest)
			}
			if err == nil {
				err = validateRequestConstraints(route, operation.ID, operationRequest)
			}
			if err != nil {
				requestStatus = "invalid"
				result.InvalidOperationRequests = append(result.InvalidOperationRequests, OperationRequestIssue{
					Operation: operation.ID, Message: err.Error(),
				})
				result.Reasons = append(result.Reasons, operation.ID+" request is invalid: "+err.Error())
				makeConditional(&result)
			} else {
				requestStatus = "valid"
				operationReady[operation.ID] = true
			}
		}
		result.Operations = append(result.Operations, OperationAssessment{
			ID: operation.ID, Title: operation.Title,
			Effects: operation.Effects, Requirements: operation.Requirements,
			RequestStatus: requestStatus,
		})
		result.Network = result.Network || operation.Effects.Network
		result.MayCharge = result.MayCharge || operation.Effects.MayCharge
		for _, requirement := range operation.Requirements {
			condition := DependencyCondition{
				Operation: operation.ID, Name: requirement.Name, Kind: requirement.Kind,
				Strength: requirement.Strength, Resolution: requirement.Resolution,
			}
			switch requirement.Resolution {
			case toolbox.ResolutionUnsatisfied:
				if requirement.Strength == "mandatory" {
					result.MissingDependencies = append(result.MissingDependencies, condition)
					result.Reasons = append(result.Reasons,
						"operation "+operation.ID+" requires missing "+requirement.Kind+" "+requirement.Name)
					result.Status = StatusUnavailable
				} else {
					result.Reasons = append(result.Reasons,
						"operation "+operation.ID+" is degraded without preferred "+requirement.Name)
					makeConditional(&result)
				}
			case toolbox.ResolutionUnknown:
				result.UnverifiedDependencies = append(result.UnverifiedDependencies, condition)
				result.Reasons = append(result.Reasons,
					"operation "+operation.ID+" has unverified "+requirement.Kind+" "+requirement.Name)
				makeConditional(&result)
			}
		}
	}

	for _, binding := range route.Bindings {
		item := BindingAssessment{Binding: binding}
		var sourceValue json.RawMessage
		switch {
		case binding.FromInput != "":
			sourceValue = request.Inputs[binding.FromInput]
			item.Constructible = inputReady[binding.FromInput]
			if !item.Constructible {
				item.Reason = "source input " + binding.FromInput + " is not concrete"
			}
		case binding.FromOperation != "":
			sourceValue, item.Constructible = requestField(
				operationRequests[binding.FromOperation], binding.FromParameter,
			)
			item.Constructible = item.Constructible && operationReady[binding.FromOperation]
			if !item.Constructible {
				item.Reason = "source operation " + binding.FromOperation + " has no valid request with " + binding.FromParameter
			}
		default:
			item.Reason = "binding has no source"
		}
		if item.Constructible {
			if !operationReady[binding.ToOperation] {
				item.Constructible = false
				item.Reason = "target operation " + binding.ToOperation + " has no valid request"
			} else if !bindingValuesMatch(binding, sourceValue, operationRequests[binding.ToOperation]) {
				item.Constructible = false
				item.Reason = "target parameter does not consume the bound source value with " + binding.TargetSemantics + " semantics"
			}
		}
		if !item.Constructible {
			result.Reasons = append(result.Reasons,
				"binding to "+binding.ToOperation+"."+binding.ToParameter+" is not constructible: "+item.Reason)
			makeConditional(&result)
		}
		result.Bindings = append(result.Bindings, item)
	}

	applyEffectPolicy(&result, "network", result.Network, request.AllowNetwork)
	applyEffectPolicy(&result, "charge", result.MayCharge, request.AllowCharges)
	return result
}

func validateRequestConstraints(route Route, operation string, data json.RawMessage) error {
	for _, constraint := range route.RequestConstraints {
		if constraint.Operation != operation {
			continue
		}
		value, present := requestField(data, constraint.Parameter)
		if !present {
			return fmt.Errorf(
				"route %s requires %s.%s to equal %q",
				route.ID, operation, constraint.Parameter, constraint.Equals,
			)
		}
		var actual string
		if json.Unmarshal(value, &actual) != nil || actual != constraint.Equals {
			return fmt.Errorf(
				"route %s requires %s.%s to equal %q",
				route.ID, operation, constraint.Parameter, constraint.Equals,
			)
		}
	}
	return nil
}

func requestField(data json.RawMessage, field string) (json.RawMessage, bool) {
	if field == "" {
		return nil, false
	}
	current := data
	for _, part := range strings.Split(field, ".") {
		var object map[string]json.RawMessage
		if json.Unmarshal(current, &object) != nil {
			return nil, false
		}
		value, ok := object[part]
		if !ok || missingValue(value) {
			return nil, false
		}
		current = value
	}
	return current, true
}

func bindingValuesMatch(binding Binding, source, targetRequest json.RawMessage) bool {
	var sourceValue any
	if json.Unmarshal(source, &sourceValue) != nil {
		return false
	}
	switch binding.TargetSemantics {
	case BindingTargetExact:
		target, present := requestField(targetRequest, binding.ToParameter)
		if !present {
			return false
		}
		var targetValue any
		if json.Unmarshal(target, &targetValue) != nil {
			return false
		}
		return reflect.DeepEqual(targetValue, sourceValue)
	case BindingTargetArrayItem:
		target, present := requestField(targetRequest, binding.ToParameter)
		if !present {
			return false
		}
		var targetValue any
		if json.Unmarshal(target, &targetValue) != nil {
			return false
		}
		values, ok := targetValue.([]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if reflect.DeepEqual(value, sourceValue) {
				return true
			}
		}
	case BindingTargetAnyValue:
		for _, target := range requestValues(targetRequest, binding.ToParameter) {
			var targetValue any
			if json.Unmarshal(target, &targetValue) == nil && reflect.DeepEqual(targetValue, sourceValue) {
				return true
			}
		}
	case BindingTargetAllValues:
		sources, ok := sourceValue.([]any)
		if !ok {
			sources = []any{sourceValue}
		}
		if len(sources) == 0 {
			return false
		}
		targets := requestValues(targetRequest, binding.ToParameter)
		for _, wanted := range sources {
			found := false
			for _, target := range targets {
				var targetValue any
				if json.Unmarshal(target, &targetValue) == nil && reflect.DeepEqual(targetValue, wanted) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case BindingTargetTextSequence:
		sourceText, ok := sourceValue.(string)
		if !ok {
			return false
		}
		var parts []string
		for _, target := range requestValues(targetRequest, binding.ToParameter) {
			var text string
			if json.Unmarshal(target, &text) == nil && strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(strings.Fields(strings.Join(parts, " ")), " ") ==
			strings.Join(strings.Fields(sourceText), " ")
	case BindingTargetVideoComposeMediaSource:
		sourcePath, ok := sourceValue.(string)
		if !ok || strings.TrimSpace(sourcePath) == "" {
			return false
		}
		mediaKind := "video"
		if binding.ArtifactKind == "image" {
			mediaKind = "image"
		}
		target, present := requestField(targetRequest, binding.ToParameter)
		if !present {
			return false
		}
		var cuts []any
		ok = json.Unmarshal(target, &cuts) == nil
		if !ok {
			return false
		}
		for _, value := range cuts {
			cut, ok := value.(map[string]any)
			if !ok || cut["type"] != "media" || cut["media_kind"] != mediaKind {
				continue
			}
			if cut["source"] == sourcePath {
				return true
			}
		}
	}
	return false
}

func requestValues(data json.RawMessage, path string) []json.RawMessage {
	parts := strings.Split(path, ".")
	var walk func(json.RawMessage, int) []json.RawMessage
	walk = func(current json.RawMessage, index int) []json.RawMessage {
		if index == len(parts) {
			return []json.RawMessage{current}
		}
		part := parts[index]
		array := strings.HasSuffix(part, "[]")
		name := strings.TrimSuffix(part, "[]")
		var object map[string]json.RawMessage
		if json.Unmarshal(current, &object) != nil {
			return nil
		}
		value, ok := object[name]
		if !ok || missingValue(value) {
			return nil
		}
		if !array {
			return walk(value, index+1)
		}
		var values []json.RawMessage
		if json.Unmarshal(value, &values) != nil {
			return nil
		}
		var out []json.RawMessage
		for _, item := range values {
			out = append(out, walk(item, index+1)...)
		}
		return out
	}
	return walk(data, 0)
}

func validateInput(input Input, value json.RawMessage) string {
	switch input.Kind {
	case "file":
		var path string
		if json.Unmarshal(value, &path) != nil || strings.TrimSpace(path) == "" {
			return "must be a concrete file path"
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return "does not exist as a file: " + path
		}
	case "files":
		var paths []string
		if err := json.Unmarshal(value, &paths); err != nil || len(paths) == 0 {
			var path string
			if json.Unmarshal(value, &path) != nil || strings.TrimSpace(path) == "" {
				return "must contain concrete file paths"
			}
			paths = []string{path}
		}
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return "contains a file that does not exist: " + path
			}
		}
	case "segments":
		var segments []json.RawMessage
		if err := json.Unmarshal(value, &segments); err != nil || len(segments) == 0 {
			return "must be a nonempty segment array"
		}
	case "consent":
		var approved bool
		if err := json.Unmarshal(value, &approved); err != nil || !approved {
			return "must be explicit true"
		}
	case "text":
		var text string
		if err := json.Unmarshal(value, &text); err != nil || strings.TrimSpace(text) == "" {
			return "must be nonblank text"
		}
	}
	return ""
}

func missingValue(value []byte) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed == "" || trimmed == "null" || trimmed == `""` || trimmed == "false" || trimmed == "[]"
}

func makeConditional(result *RouteAssessment) {
	if result.Status == StatusFeasible {
		result.Status = StatusConditional
	}
}

func applyEffectPolicy(result *RouteAssessment, effect string, applies bool, allowed *bool) {
	if !applies {
		return
	}
	if allowed == nil {
		result.Reasons = append(result.Reasons, effect+" effect requires an explicit assessment allowance")
		makeConditional(result)
		return
	}
	if !*allowed {
		result.Reasons = append(result.Reasons, effect+" effect is disallowed by the assessment request")
		result.Status = StatusUnavailable
	}
}
