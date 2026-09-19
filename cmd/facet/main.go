package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/xibodev/facet/internal/bundle"
	"github.com/xibodev/facet/internal/config"
	"github.com/xibodev/facet/internal/module"
	"github.com/xibodev/facet/internal/studio"
	"github.com/xibodev/facet/internal/toolbox"
)

const Version = "1.0.4"

func printUsage() {
	fmt.Println(`Facet - Autonomous Video Production Engine & Agent Toolbox

Usage:
  facet <command> [arguments]

Available Commands:
  doctor           Inspect system dependencies, runtimes, CLIs, and 33 tools
  init [slug]      Initialize a project workspace and link agent skills
  tools <op> ...   Run toolbox operations (list, describe, estimate, run)
  module <op>      Module protocol surface for a host (describe, invoke)
  ui               Start the Facet Studio web interface
  version          Print Facet version information
  help             Show help for Facet commands

Flags:
  -h, --help       Show help
  -v, --version    Show version

Use "facet <command> --help" for more information about a command.`)
}

type initCLIOptions struct {
	ProjectDir string
	Engine     string
	Packs      []string
	NoLaunch   bool
	Help       bool
}

func parseInitArgs(args []string) (initCLIOptions, error) {
	opts := initCLIOptions{Engine: "claude"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h" || arg == "-help":
			opts.Help = true
			return opts, nil
		case arg == "--no-launch" || arg == "-no-launch":
			opts.NoLaunch = true
		case arg == "--engine" || arg == "-engine":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return opts, fmt.Errorf("--engine requires claude, opencode, codex, copilot, or studio")
			}
			opts.Engine = args[i+1]
			i++
		case strings.HasPrefix(arg, "--engine=") || strings.HasPrefix(arg, "-engine="):
			opts.Engine = strings.SplitN(arg, "=", 2)[1]
		case arg == "--pack" || arg == "-pack" || arg == "--production-method" || arg == "-production-method":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return opts, fmt.Errorf("%s requires a pack name", arg)
			}
			opts.Packs = append(opts.Packs, args[i+1])
			i++
		case strings.HasPrefix(arg, "--pack=") || strings.HasPrefix(arg, "-pack=") ||
			strings.HasPrefix(arg, "--production-method=") || strings.HasPrefix(arg, "-production-method="):
			opts.Packs = append(opts.Packs, strings.SplitN(arg, "=", 2)[1])
		case !strings.HasPrefix(arg, "-") && opts.ProjectDir == "":
			opts.ProjectDir = arg
		default:
			return opts, fmt.Errorf("unknown argument %q. Run 'facet init --help' for usage", arg)
		}
	}
	opts.Engine = strings.ToLower(strings.TrimSpace(opts.Engine))
	switch opts.Engine {
	case "claude", "opencode", "codex", "copilot", "studio":
	default:
		return opts, fmt.Errorf("unknown engine %q; choose claude, opencode, codex, copilot, or studio", opts.Engine)
	}
	seen := make(map[string]bool)
	packs := opts.Packs[:0]
	for _, pack := range opts.Packs {
		pack = strings.ToLower(strings.TrimSpace(pack))
		if pack == "" {
			return opts, fmt.Errorf("pack name cannot be empty")
		}
		if !seen[pack] {
			seen[pack] = true
			packs = append(packs, pack)
		}
	}
	opts.Packs = packs
	return opts, nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "Doctor error: unexpected arguments")
			os.Exit(1)
		}
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
		opts, err := parseInitArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
			os.Exit(1)
		}
		if opts.Help {
			fmt.Println("Usage: facet init [project-directory] [--engine claude|opencode|codex|copilot|studio] [--pack name ...] [--production-method name ...] [--no-launch]\n\nInitialize a core-only workspace and launch the selected agent (default: claude).\n--pack and --production-method are repeatable aliases that activate only the named guidance packs.\n--no-launch initializes without starting an agent.\n-h, --help prints this usage without writing files.")
			return
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
			os.Exit(1)
		}

		if _, err := config.RunInitWithOptions(config.InitOptions{
			ProjectDir: opts.ProjectDir,
			Engine:     opts.Engine,
			Packs:      opts.Packs,
		}, cfg, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
			os.Exit(1)
		}

		if !opts.NoLaunch {
			targetDir := "."
			if opts.ProjectDir != "" {
				targetDir = opts.ProjectDir
			}
			if opts.Engine == "studio" {
				if err := studio.RunWithOption(":8787", targetDir, true); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				return
			}
			cliPath := config.FindExecutable(opts.Engine, opts.Engine+".cmd", opts.Engine+".exe", opts.Engine+".ps1")
			if cliPath == "" {
				cliPath = opts.Engine
			}
			fmt.Printf("🚀 Launching %s in %s...\n", opts.Engine, targetDir)
			c := exec.Command(cliPath)
			if runtime.GOOS == "windows" {
				switch strings.ToLower(filepath.Ext(cliPath)) {
				case ".cmd", ".bat":
					c = exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/d", "/c", cliPath)
				case ".ps1":
					c = exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-File", cliPath)
				}
			}
			c.Dir = targetDir
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Launch error: %v\n", err)
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() > 0 {
					os.Exit(exitErr.ExitCode())
				}
				os.Exit(1)
			}
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

	case "module":
		// Host module protocol surface. Additive: `facet tools` is unchanged and
		// remains the documented human-facing contract. stdout carries exactly
		// one JSON envelope; diagnostics go to stderr.
		env, ok := moduleCLI(os.Args[2:])
		// Compact, deliberately. The host parses this; nobody reads it.
		// Indentation was 54% of the descriptor — 232KB against 107KB — and
		// the host truncates at max_output_bytes, after which a partial JSON
		// document cannot be trusted even if it looks complete. Halving the
		// payload doubles the headroom before that becomes a hard failure.
		// `facet tools` keeps its indentation; that output is read by people.
		encoder := json.NewEncoder(os.Stdout)
		if err := encoder.Encode(env); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// An async invocation returns a job handle immediately, but this is a
		// one-shot process: exiting here would kill the goroutine and discard
		// the render it represents. A handle that loses its work is worse than
		// blocking, so wait for jobs this process started. The envelope is
		// already written, so the host has its handle while the work finishes.
		if module.HasRunningJobs() {
			if !module.AwaitJobs(30 * time.Minute) {
				fmt.Fprintln(os.Stderr,
					"facet: exiting with unfinished jobs; their output was not completed")
			}
		}
		if !ok {
			os.Exit(1)
		}

	case "bundle":
		// Release C: build an installable, target-shaped Facet package.
		//
		// The canonical assets and the public tool vocabulary are READ from
		// product truth here and passed in; the builder never holds its own
		// copy of either.
		fs := flag.NewFlagSet("bundle", flag.ExitOnError)
		target := fs.String("target", "", "claude | codex | copilot | opencode | all")
		out := fs.String("out", "dist/bundles", "Output directory")
		_ = fs.Parse(os.Args[2:])

		if *target == "" {
			fmt.Fprintln(os.Stderr, "Usage: facet bundle --target <claude|codex|copilot|opencode|all> [--out dir]")
			os.Exit(1)
		}

		var wanted []bundle.Target
		if *target == "all" {
			wanted = bundle.Targets()
		} else {
			t := bundle.Target(*target)
			valid := false
			for _, k := range bundle.Targets() {
				if k == t {
					valid = true
				}
			}
			if !valid {
				fmt.Fprintf(os.Stderr, "Unknown target %q; choose claude, codex, copilot, opencode or all\n", *target)
				os.Exit(1)
			}
			wanted = []bundle.Target{t}
		}

		src := bundle.Source{
			SkillsDir:    filepath.Join("skills", "facet"),
			PacksDir:     "packs",
			Tools:        toolbox.Names(),
			FacetVersion: Version,
		}

		for _, t := range wanted {
			dir := filepath.Join(*out, string(t))
			// A stale bundle merged with a fresh one produces a manifest that
			// does not describe its own directory.
			_ = os.RemoveAll(dir)
			m, err := bundle.Build(src, t, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "bundle %s: %v\n", t, err)
				os.Exit(1)
			}
			// Verify what was just written rather than trusting the build.
			// "the build returned nil" and "the bundle is whole" are different
			// claims, and this repo has shipped a bundle that reported success
			// while missing the files that made it work.
			if _, err := bundle.Verify(dir); err != nil {
				fmt.Fprintf(os.Stderr, "bundle %s failed verification: %v\n", t, err)
				os.Exit(1)
			}
			fmt.Printf("%-9s %d entries  %s  %s\n", t, len(m.Entries), m.BundleDigest[:19], dir)
		}

	case "ui", "studio":
		fs := flag.NewFlagSet("ui", flag.ExitOnError)
		port := fs.Int("port", 8787, "Port to listen on")
		dir := fs.String("dir", ".", "Working directory / root directory of projects")
		noOpen := fs.Bool("no-open", false, "Do not automatically open browser")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "UI error: unexpected arguments")
			os.Exit(1)
		}

		addr := fmt.Sprintf(":%d", *port)
		if err := studio.RunWithOption(addr, *dir, !*noOpen); err != nil {
			fmt.Fprintf(os.Stderr, "UI error: %v\n", err)
			os.Exit(1)
		}

	case "version", "-v", "--version":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Version error: unexpected arguments")
			os.Exit(1)
		}
		fmt.Printf("facet v%s\n", Version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q. Run 'facet help' for usage.\n", cmd)
		os.Exit(1)
	}
}
