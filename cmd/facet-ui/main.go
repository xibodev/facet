package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/xibodev/facet/internal/studio"
)

const Version = "1.0.0"

func main() {
	fs := flag.NewFlagSet("facet-ui", flag.ExitOnError)
	port := fs.Int("port", 8787, "Port for Facet Studio UI to listen on")
	dir := fs.String("dir", ".", "Working directory / root directory of projects")
	noOpen := fs.Bool("no-open", false, "Do not automatically open default web browser")
	version := fs.Bool("version", false, "Print version information")
	v := fs.Bool("v", false, "Print version information")

	_ = fs.Parse(os.Args[1:])

	if *version || *v {
		fmt.Printf("facet-ui v%s\n", Version)
		return
	}

	addr := fmt.Sprintf(":%d", *port)
	if err := studio.RunWithOption(addr, *dir, !*noOpen); err != nil {
		fmt.Fprintf(os.Stderr, "Facet UI error: %v\n", err)
		os.Exit(1)
	}
}
