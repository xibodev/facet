// facet-install owns installation policy; the media toolbox does not import it.
package main

import (
	"fmt"
	"os"

	"github.com/xibodev/facet/internal/installer"
)

var version = "1.0.2"

func main() {
	if err := installer.Run(os.Args[1:], os.Stdin, os.Stdout, version); err != nil {
		fmt.Fprintln(os.Stderr, "Setup incomplete:", err)
		os.Exit(1)
	}
}
