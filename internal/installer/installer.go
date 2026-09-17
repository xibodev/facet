package installer

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

var hosts = map[string]string{"opencode": ".opencode", "codex": ".agents", "claude": ".claude", "copilot": ".github"}
var validVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$`)

type setup struct {
	in                                                        *bufio.Reader
	out                                                       io.Writer
	version, target, project, root, archive, sums, components string
	yes, skipVerify                                           bool
}

func (s *setup) ask(prompt, fallback string) (string, error) {
	fmt.Fprintf(s.out, "%s [%s]: ", prompt, fallback)
	line, err := s.in.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("interactive input ended; use --yes with explicit options for automation: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = fallback
	}
	return line, nil
}

func Run(args []string, in io.Reader, out io.Writer, version string) error {
	previousPath := os.Getenv("PATH")
	defer os.Setenv("PATH", previousPath)
	s := &setup{in: bufio.NewReader(in), out: out}
	f := flag.NewFlagSet("facet-install", flag.ContinueOnError)
	f.SetOutput(out)
	f.StringVar(&s.version, "version", version, "published Facet version")
	f.StringVar(&s.target, "target", "", "opencode, codex, claude, or copilot (authentication assumed)")
	f.StringVar(&s.project, "project", "", "project to receive Facet skills")
	f.StringVar(&s.root, "install-dir", "", "version-specific Facet installation directory")
	f.StringVar(&s.archive, "archive", "", "offline release ZIP (requires --checksums)")
	f.StringVar(&s.sums, "checksums", "", "offline SHA256SUMS.txt")
	f.StringVar(&s.components, "components", "remotion", "comma-separated remotion,piper,gflow,hyperframes; none for core editing only")
	f.BoolVar(&s.yes, "yes", false, "accept displayed dependency installation commands; requires --target and --project")
	f.BoolVar(&s.skipVerify, "skip-verify", false, "skip real local media checks; report installation as unverified")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if !validVersion.MatchString(s.version) {
		return fmt.Errorf("invalid release version")
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	if (s.archive == "") != (s.sums == "") {
		return fmt.Errorf("--archive and --checksums must be supplied together")
	}
	fmt.Fprintf(out, "Facet %s — agent CLI installation (%s/%s)\n", s.version, runtime.GOOS, runtime.GOARCH)
	var err error
	if !s.yes {
		fmt.Fprintln(out, "Supported CLIs: opencode, codex, claude, copilot. Your CLI's existing authentication is used.")
		if s.target == "" {
			s.target, err = s.ask("Agent CLI", "opencode")
			if err != nil {
				return err
			}
		}
		if s.project == "" {
			s.project, err = s.ask("Project directory", ".")
			if err != nil {
				return err
			}
		}
	}
	if _, ok := hosts[s.target]; !ok {
		return fmt.Errorf("choose --target opencode, codex, claude, or copilot")
	}
	if s.project == "" {
		return fmt.Errorf("--project is required")
	}
	s.project, err = filepath.Abs(s.project)
	if err != nil {
		return err
	}
	if s.root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		s.root = filepath.Join(home, ".facet", "releases", s.version+"-"+runtime.GOOS+"-"+runtime.GOARCH)
	}
	s.root, err = filepath.Abs(s.root)
	if err != nil {
		return err
	}
	if err = s.selectComponents(); err != nil {
		return err
	}
	// Resolve choices before downloading or modifying any installation.
	if err = s.validateComponents(); err != nil {
		return err
	}
	if err = preflightProject(s.project, s.target); err != nil {
		return err
	}
	fmt.Fprintf(out, "Install: %s\nProject: %s\nCLI: %s\nOptional components: %s\n", s.root, s.project, s.target, s.components)
	if !s.yes {
		answer, err := s.ask("Proceed with this installation?", "y")
		if err != nil {
			return err
		}
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("installation cancelled")
		}
	}
	if err = s.installRelease(); err != nil {
		return err
	}
	if err = s.installDependencies(); err != nil {
		return err
	}
	if !s.skipVerify {
		if err = s.verify(); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(out, "Verification skipped: installed, but media readiness is UNVERIFIED.")
	}
	if err = s.integrate(); err != nil {
		return err
	}
	fmt.Fprintln(out, "Setup complete. Start a new agent session in:", s.project)
	fmt.Fprintln(out, "Ask it to use the Facet skill. Optional media services may require API keys or account access; tools report missing configuration when used.")
	return nil
}

func executable(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func (s *setup) run(dir, command string, args ...string) error {
	fmt.Fprintf(s.out, "Run: %s %q\n", command, args)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, command, args...)
	if runtime.GOOS == "windows" && (strings.HasSuffix(command, ".cmd") || strings.HasSuffix(command, ".bat")) {
		script := "& " + psQuote(command)
		for _, arg := range args {
			script += " " + psQuote(arg)
		}
		script += "; exit $LASTEXITCODE"
		units := utf16.Encode([]rune(script))
		data := make([]byte, len(units)*2)
		for i, v := range units {
			binary.LittleEndian.PutUint16(data[i*2:], v)
		}
		c = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	}
	c.Dir = dir
	c.Stdout, c.Stderr = s.out, s.out
	if command == "sudo" {
		c.Stdin = os.Stdin
	}
	c.WaitDelay = 10 * time.Second
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func (s *setup) installRelease() error {
	if _, err := os.Lstat(s.root); err == nil {
		return verifyReceipt(s.root, s.version)
	} else if !os.IsNotExist(err) {
		return err
	}
	temp, err := os.MkdirTemp("", "facet-download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	name := fmt.Sprintf("facet-%s-%s-%s.zip", s.version, runtime.GOOS, runtime.GOARCH)
	archive, sums := s.archive, s.sums
	if archive == "" {
		base := "https://github.com/xibodev/facet/releases/download/v" + s.version + "/"
		archive, sums = filepath.Join(temp, name), filepath.Join(temp, "SHA256SUMS.txt")
		fmt.Fprintln(s.out, "Downloading prebuilt release.")
		if err = download(base+"SHA256SUMS.txt", sums); err != nil {
			return err
		}
		if err = download(base+name, archive); err != nil {
			return fmt.Errorf("no usable prebuilt release for this platform: %w", err)
		}
	}
	if err = verifyChecksum(archive, sums, name); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.root), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(s.root), ".facet-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err = extractZip(archive, stage); err != nil {
		return err
	}
	for _, name := range []string{filepath.Join("bin", executable("facet")), "bundle/skills/facet/SKILL.md", "bundle/remotion-composer/package-lock.json"} {
		if info, err := os.Stat(filepath.Join(stage, name)); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("release missing %s", name)
		}
	}
	version, err := exec.Command(filepath.Join(stage, "bin", executable("facet")), "version").Output()
	if err != nil || strings.TrimSpace(string(version)) != "facet v"+s.version {
		return fmt.Errorf("binary version does not match release")
	}
	if err = saveReceipt(stage, s.version); err != nil {
		return err
	}
	// Windows scanners may briefly retain a handle after the version probe exits.
	for attempt := 0; attempt < 10; attempt++ {
		err = os.Rename(stage, s.root)
		if err == nil {
			return nil
		}
		if _, exists := os.Lstat(s.root); exists == nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return err
}
