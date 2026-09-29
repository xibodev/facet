package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Envelope struct {
	OK        bool     `json:"ok"`
	Operation string   `json:"operation"`
	Result    any      `json:"result,omitempty"`
	Error     *Error   `json:"error,omitempty"`
	Warnings  []string `json:"warnings"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func CLI(args []string) (Envelope, bool) {
	fail := func(operation, message string) (Envelope, bool) {
		return Envelope{
			OK: false, Operation: operation,
			Error:    &Error{Code: "invalid_request", Message: message},
			Warnings: []string{},
		}, false
	}
	if len(args) == 0 {
		return fail("", "usage: facet routes <list|describe|assess>")
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fail("list", "routes list accepts no arguments")
		}
		if err := ValidateCatalog(); err != nil {
			return fail("list", err.Error())
		}
		return Envelope{
			OK: true, Operation: "list", Warnings: []string{},
			Result: map[string]any{
				"methods": Catalog(), "auto_select_providers": false,
				"executes": false, "persists_workflow_state": false,
			},
		}, true
	case "describe":
		if len(args) != 2 {
			return fail("describe", "usage: facet routes describe <method>")
		}
		method, err := Describe(args[1])
		if err != nil {
			return fail("describe", err.Error())
		}
		return Envelope{
			OK: true, Operation: "describe", Result: method, Warnings: []string{},
		}, true
	case "assess":
		if len(args) != 3 || args[1] != "--input" {
			return fail("assess", "usage: facet routes assess --input <json-or-path>")
		}
		data, err := readInput(args[2])
		if err != nil {
			return fail("assess", err.Error())
		}
		var request Request
		if err := json.Unmarshal(data, &request); err != nil {
			return fail("assess", "invalid assessment JSON: "+err.Error())
		}
		result, err := Assess(request)
		if err != nil {
			return fail("assess", err.Error())
		}
		return Envelope{
			OK: true, Operation: "assess", Result: result, Warnings: []string{},
		}, true
	default:
		return fail(args[0], "unknown routes operation: "+args[0])
	}
}

func readInput(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	unquoted := strings.Trim(strings.TrimSpace(trimmed), "'`\"")
	if strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}") {
		return []byte(unquoted), nil
	}
	data, err := os.ReadFile(value)
	if err != nil {
		return nil, fmt.Errorf("assessment input could not be read: %s", value)
	}
	return data, nil
}
