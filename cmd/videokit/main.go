package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/xibodev/facet/internal/config"
	"github.com/xibodev/facet/internal/studio"
	"github.com/xibodev/facet/internal/toolbox"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "studio" {
		fs := flag.NewFlagSet("studio", flag.ExitOnError)
		port := fs.Int("port", 8787, "Port to listen on")
		dir := fs.String("dir", ".", "Working directory / root directory of projects")
		_ = fs.Parse(os.Args[2:])

		addr := fmt.Sprintf(":%d", *port)
		if err := studio.Run(addr, *dir); err != nil {
			fmt.Fprintf(os.Stderr, "Studio error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config warning: %v\n", err)
			cfg = config.DefaultConfig()
		}
		if err := config.RunDoctor(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Doctor error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "init" {
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		engine := fs.String("engine", "claude", "Agent engine (claude, opencode, copilot)")
		_ = fs.Parse(os.Args[2:])
		slug := ""
		if fs.NArg() > 0 {
			slug = fs.Arg(0)
		}
		cfg, err := config.Load()
		if err != nil {
			cfg = config.DefaultConfig()
		}
		if err := config.RunInit(slug, *engine, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	result, ok := toolbox.CLI(os.Args[1:])
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !ok {
		os.Exit(1)
	}
}
