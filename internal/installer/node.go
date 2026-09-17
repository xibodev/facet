package installer

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Linux distro Node versions can lag the renderers. Use the official prebuilt
// runtime, confined to this Facet installation, rather than a shell bootstrap.
func (s *setup) installLinuxNode() error {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	stem := "node-v24.18.0-linux-" + arch
	name := stem + ".tar.gz"
	base := "https://nodejs.org/dist/v24.18.0/"
	if err := s.approve("download verified Node.js archive", []string{base + name}); err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "facet-node-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	archive, sums := filepath.Join(temp, name), filepath.Join(temp, "SHASUMS256.txt")
	if err = download(base+name, archive); err != nil {
		return err
	}
	if err = download(base+"SHASUMS256.txt", sums); err != nil {
		return err
	}
	if err = verifyChecksum(archive, sums, name); err != nil {
		return err
	}
	stage := filepath.Join(temp, "node")
	if err = os.Mkdir(stage, 0755); err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	r := tar.NewReader(gz)
	var total int64
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !strings.HasPrefix(h.Name, stem+"/") {
			return fmt.Errorf("unexpected Node archive root")
		}
		name := strings.TrimSuffix(strings.TrimPrefix(h.Name, stem+"/"), "/")
		if name == "" {
			continue
		}
		if !fs.ValidPath(name) || strings.ContainsAny(name, ":\\") {
			return fmt.Errorf("unsafe Node archive entry")
		}
		// Node ships npm/npx/corepack symlinks. Do not extract links; create
		// explicit npm/npx launchers below and leave Corepack uninstalled.
		if h.Typeflag == tar.TypeSymlink {
			continue
		}
		if h.Size < 0 || h.Size > 1<<30 || total > (1<<30)-h.Size {
			return fmt.Errorf("Node archive too large")
		}
		total += h.Size
		path := filepath.Join(stage, filepath.FromSlash(name))
		if h.Typeflag == tar.TypeDir {
			if err = os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("unexpected Node archive file type")
		}
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		perm := fs.FileMode(0644)
		if h.Mode&0111 != 0 {
			perm = 0755
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, r)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	dest := filepath.Join(s.root, "dependencies", "node")
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("existing managed Node failed compatibility check; choose a fresh installation directory")
	}
	for _, command := range []string{"npm", "npx"} {
		content := "#!/bin/sh\nexec " + shellQuote(filepath.Join(dest, "bin", "node")) + " " + shellQuote(filepath.Join(dest, "lib", "node_modules", "npm", "bin", command+"-cli.js")) + " \"$@\"\n"
		if err := os.WriteFile(filepath.Join(stage, "bin", command), []byte(content), 0755); err != nil {
			return err
		}
	}
	if err = copyTree(stage, dest); err != nil {
		return err
	}
	os.Setenv("PATH", filepath.Join(dest, "bin")+":"+os.Getenv("PATH"))
	return nil
}
