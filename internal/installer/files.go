package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxDownload = int64(512 << 20)

func extractTar(archive, dest string) error {
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
	seen := map[string]bool{}
	var total int64
	for {
		h, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(h.Name, "./"), "/")
		if name == "." || name == "" {
			continue
		}
		if !fs.ValidPath(name) || strings.ContainsAny(name, ":\\") {
			return fmt.Errorf("unsafe archive path: %s", name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate archive path: %s", name)
		}
		seen[key] = true
		if h.Size < 0 || h.Size > 1<<30 || total > (1<<30)-h.Size {
			return fmt.Errorf("archive exceeds extraction limit")
		}
		total += h.Size
		path := filepath.Join(dest, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
		default:
			return fmt.Errorf("archive links or special files are unsupported: %s", name)
		}
	}
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxDownload+1))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if n > maxDownload {
		return fmt.Errorf("download exceeds 512 MiB limit")
	}
	return closeErr
}

func digest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyChecksum(archive, sums, name string) error {
	b, err := os.ReadFile(sums)
	if err != nil {
		return err
	}
	expected := ""
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if expected != "" {
				return fmt.Errorf("duplicate checksum for %s", name)
			}
			expected = strings.ToLower(fields[0])
		}
	}
	actual, err := digest(archive)
	if err != nil {
		return err
	}
	if len(expected) != 64 || actual != expected {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	return nil
}

// Only regular files and directories are accepted; archives never provide links.
func extractZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	seen := map[string]bool{}
	var total uint64
	for _, entry := range r.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		for strings.HasPrefix(name, "./") {
			name = strings.TrimPrefix(name, "./")
		}
		name = strings.TrimSuffix(name, "/")
		if name == "" {
			continue
		}
		if !fs.ValidPath(name) || strings.Contains(name, ":") || strings.Contains(name, "\x00") {
			return fmt.Errorf("unsafe archive path: %s", name)
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
				return fmt.Errorf("unsafe archive path: %s", name)
			}
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate archive path: %s", name)
		}
		seen[key] = true
		mode := entry.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return fmt.Errorf("archive links or special files are unsupported: %s", name)
		}
		if entry.UncompressedSize64 > 1<<30 || total > (1<<30)-entry.UncompressedSize64 {
			return fmt.Errorf("archive exceeds 1 GiB extraction limit")
		}
		total += entry.UncompressedSize64
		target := filepath.Join(dest, filepath.FromSlash(name))
		if mode.IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		perm := fs.FileMode(0644)
		if mode&0111 != 0 {
			perm = 0755
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, int64(entry.UncompressedSize64)+1))
		in.Close()
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing source link: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, info.Mode().Perm())
	})
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
