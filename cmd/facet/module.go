package main

import (
	"os"
	"strings"

	"github.com/xibodev/facet/internal/module"
)

// moduleCLI implements the host module protocol verb surface.
//
// The host speaks exactly two verbs:
//
//	facet module describe --json
//	facet module invoke <capability> --input <request.json|inline-json>
//
// Estimation is a capability (creative.tools.estimate), not a third verb, so
// the host's verb surface stays at two and no per-module dispatch shape leaks
// into the protocol.
//
// --input accepts either a file path or inline JSON, matching the existing
// `facet tools` behavior so agent guidance stays valid across both surfaces.
func moduleCLI(args []string) (module.Envelope, bool) {
	if len(args) == 0 {
		return module.Usage("usage: facet module <describe|invoke>"), false
	}

	switch args[0] {
	case "describe":
		// --json is accepted and is the only supported encoding; it is allowed
		// as an explicit flag so the host's command line reads unambiguously.
		for _, a := range args[1:] {
			if a != "--json" {
				return module.Usage("usage: facet module describe [--json]"), false
			}
		}
		env := module.Describe(Version)
		return env, env.OK

	case "invoke":
		if len(args) != 4 || args[2] != "--input" {
			return module.Usage(
				"usage: facet module invoke <capability> --input <request.json>"), false
		}
		body, err := readInput(args[3])
		if err != nil {
			return module.InputError("invoke", args[3], err), false
		}
		env := module.Invoke(args[1], body)
		return env, env.OK
	}

	return module.Usage("unknown module operation: " + args[0]), false
}

// readInput accepts inline JSON or a file path, mirroring toolbox.CLI.
func readInput(arg string) ([]byte, error) {
	trimmed := strings.TrimSpace(arg)
	unquoted := strings.TrimSpace(strings.Trim(trimmed, "'`\""))
	if (strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}")) ||
		(strings.HasPrefix(unquoted, "[") && strings.HasSuffix(unquoted, "]")) {
		return []byte(unquoted), nil
	}
	return os.ReadFile(arg)
}
