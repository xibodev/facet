package routes

import (
	"fmt"
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
	ID                     string                `json:"id"`
	Title                  string                `json:"title"`
	Summary                string                `json:"summary"`
	Status                 string                `json:"status"`
	RequiredInputs         []Input               `json:"required_inputs"`
	MissingInputs          []string              `json:"missing_inputs"`
	Operations             []OperationAssessment `json:"operations"`
	MissingDependencies    []DependencyCondition `json:"missing_dependencies"`
	UnverifiedDependencies []DependencyCondition `json:"unverified_dependencies"`
	Network                bool                  `json:"network"`
	MayCharge              bool                  `json:"may_charge"`
	Reasons                []string              `json:"reasons"`
}

type OperationAssessment struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Effects      toolbox.V2Effects       `json:"effects"`
	Requirements []toolbox.V2Requirement `json:"requirements"`
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
		MissingInputs: []string{}, Operations: []OperationAssessment{},
		MissingDependencies:    []DependencyCondition{},
		UnverifiedDependencies: []DependencyCondition{},
		Reasons:                []string{},
	}

	for _, input := range route.RequiredInputs {
		value, ok := request.Inputs[input.Name]
		if !ok || missingValue(value) {
			result.MissingInputs = append(result.MissingInputs, input.Name)
			result.Reasons = append(result.Reasons, "required input "+input.Name+" is not supplied")
			makeConditional(&result)
		}
	}

	for _, operationID := range route.Operations {
		operation, ok := operations[operationID]
		if !ok {
			result.Reasons = append(result.Reasons, "canonical operation "+operationID+" is unavailable")
			result.Status = StatusUnavailable
			continue
		}
		result.Operations = append(result.Operations, OperationAssessment{
			ID: operation.ID, Title: operation.Title,
			Effects: operation.Effects, Requirements: operation.Requirements,
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

	applyEffectPolicy(&result, "network", result.Network, request.AllowNetwork)
	applyEffectPolicy(&result, "charge", result.MayCharge, request.AllowCharges)
	return result
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
