// facet-module is the headless capability entry point. It does not import the
// standalone UI or embedded kernel; the installing Studio host owns the runtime.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xibodev/facet/internal/module"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "module" {
		args = args[1:]
	}
	env := module.Usage("usage: facet-module module describe [--json] | invoke <capability> --input <json-or-file>")
	if len(args) >= 1 && args[0] == "describe" && (len(args) == 1 || (len(args) == 2 && args[1] == "--json")) {
		env = module.Describe("1.0.4")
	}
	if len(args) == 4 && args[0] == "invoke" && args[2] == "--input" {
		body := []byte(args[3])
		var err error
		if !strings.HasPrefix(strings.TrimSpace(args[3]), "{") {
			body, err = os.ReadFile(args[3])
		}
		if err != nil {
			env = module.InputError("invoke", args[3], err)
		} else {
			env = module.Invoke(args[1], body)
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(env); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !env.OK {
		os.Exit(1)
	}
}
