package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/xibodev/facet/internal/config"
	"github.com/xibodev/facet/internal/studio"
	"github.com/xibodev/facet/internal/toolbox"
)

const Version = "1.0.1"

func printUsage() {
	fmt.Println(`Facet - Autonomous Video Production Engine & Agent Toolbox

Usage:
  facet <command> [arguments]

Available Commands:
  doctor           Inspect system dependencies, runtimes, CLIs, and 33 tools
  init [slug]      Initialize a project workspace and link agent skills
  tools <op> ...   Run toolbox operations (list, describe, estimate, run)
  ui               Start the Facet Studio web interface
  version          Print Facet version information
  help             Show help for Facet commands

Flags:
  -h, --help       Show help
  -v, --version    Show version

Use "facet <command> --help" for more information about a command.`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "doctor":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config warning: %v\n", err)
			cfg = config.DefaultConfig()
		}
		if err := config.RunDoctor(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Doctor error: %v\n", err)
			os.Exit(1)
		}

	case "init":
		engine := "claude"
		noLaunch := false
		slug := ""

		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "--no-launch" || arg == "-no-launch" {
				noLaunch = true
			} else if arg == "--engine" || arg == "-engine" {
				if i+1 < len(args) {
					engine = args[i+1]
					i++
				}
			} else if strings.HasPrefix(arg, "--engine=") || strings.HasPrefix(arg, "-engine=") {
				parts := strings.SplitN(arg, "=", 2)
				engine = parts[1]
			} else if !strings.HasPrefix(arg, "-") && slug == "" {
				slug = arg
			}
		}

		cfg, err := config.Load()
		if err != nil {
			cfg = config.DefaultConfig()
		}

		if err := config.RunInit(slug, engine, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
			os.Exit(1)
		}

		if !noLaunch {
			targetDir := "."
			if slug != "" {
				targetDir = slug
			}
			cliPath := config.FindExecutable(engine, engine+".cmd", engine+".exe", engine+".ps1")
			if cliPath == "" {
				cliPath = engine
			}
			fmt.Printf("🚀 Launching %s in %s...\n", engine, targetDir)
			c := exec.Command(cliPath)
			c.Dir = targetDir
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			_ = c.Run()
		}

	case "tools":
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

	case "ui", "studio":
		fs := flag.NewFlagSet("ui", flag.ExitOnError)
		port := fs.Int("port", 8787, "Port to listen on")
		dir := fs.String("dir", ".", "Working directory / root directory of projects")
		noOpen := fs.Bool("no-open", false, "Do not automatically open browser")
		_ = fs.Parse(os.Args[2:])

		addr := fmt.Sprintf(":%d", *port)
		if err := studio.RunWithOption(addr, *dir, !*noOpen); err != nil {
			fmt.Fprintf(os.Stderr, "UI error: %v\n", err)
			os.Exit(1)
		}

	case "version", "-v", "--version":
		fmt.Printf("facet v%s\n", Version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q. Run 'facet help' for usage.\n", cmd)
		os.Exit(1)
	}
}
