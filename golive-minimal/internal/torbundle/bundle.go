package torbundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Version       = "15.0.20"
	ArchiveURL    = "https://archive.torproject.org/tor-package-archive/torbrowser/15.0.20/tor-expert-bundle-windows-x86_64-15.0.20.tar.gz"
	ArchiveSHA256 = "d59bff934e3ad876e1623e24ae60c19aeea56f50178093b9f86fba230639f949"
	MaxArchive    = 64 << 20
)

var selectedFiles = map[string]string{
	"tor/tor.exe": "ea61ba0ed5b89d0622d2894b2a86f5ff34ce9b48e6e40d64341e7c0c7ee03e08",
	"data/geoip":  "af9ccd060a712d090ee07d5678b5d45b0038ec1573116fae724a6695a8485703",
	"data/geoip6": "2393124667ba2ccb4c806f226a33b2ef7a8188d1ba55831c1a5d3dca2b062514",
}

type Paths struct {
	Root, Executable, GeoIP, GeoIPv6 string
}

func paths(root string) Paths {
	versionRoot := filepath.Join(root, "Tor", Version)
	return Paths{
		Root:       versionRoot,
		Executable: filepath.Join(versionRoot, "tor", "tor.exe"),
		GeoIP:      filepath.Join(versionRoot, "data", "geoip"),
		GeoIPv6:    filepath.Join(versionRoot, "data", "geoip6"),
	}
}

func Ensure(ctx context.Context, root string, notify func()) (Paths, error) {
	p := paths(root)
	if validInstall(p) {
		return p, nil
	}
	if notify != nil {
		notify()
	}
	if err := os.MkdirAll(filepath.Join(root, "Tor"), 0o700); err != nil {
		return Paths{}, err
	}

	tmp, err := os.CreateTemp(filepath.Join(root, "Tor"), "download-*.tar.gz")
	if err != nil {
		return Paths{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := download(ctx, tmp, ArchiveURL); err != nil {
		tmp.Close()
		return Paths{}, err
	}
	if err := tmp.Close(); err != nil {
		return Paths{}, err
	}
	if err := verifyFile(tmpName, ArchiveSHA256); err != nil {
		return Paths{}, fmt.Errorf("o pacote oficial do Tor nao passou no SHA-256: %w", err)
	}

	stage, err := os.MkdirTemp(filepath.Join(root, "Tor"), "extract-*")
	if err != nil {
		return Paths{}, err
	}
	defer os.RemoveAll(stage)
	if err := ExtractSelected(tmpName, stage); err != nil {
		return Paths{}, err
	}
	stagePaths := Paths{
		Root: stage, Executable: filepath.Join(stage, "tor", "tor.exe"),
		GeoIP: filepath.Join(stage, "data", "geoip"), GeoIPv6: filepath.Join(stage, "data", "geoip6"),
	}
	if !validInstall(stagePaths) {
		return Paths{}, fmt.Errorf("arquivos extraidos do Tor nao conferem")
	}

	// The exact target is app-owned and versioned. RemoveAll does not follow a
	// symlink at the final path, and extraction never accepts links.
	if err := os.RemoveAll(p.Root); err != nil {
		return Paths{}, err
	}
	if err := os.Rename(stage, p.Root); err != nil {
		return Paths{}, err
	}
	return p, nil
}

func download(ctx context.Context, dst io.Writer, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "archive.torproject.org" {
		return fmt.Errorf("URL do Tor recusada")
	}
	client := &http.Client{
		Timeout: 3 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Hostname() != "archive.torproject.org" {
				return fmt.Errorf("redirecionamento do download recusado")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download do Tor respondeu HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, MaxArchive+1)
	n, err := io.Copy(dst, limited)
	if err != nil {
		return err
	}
	if n > MaxArchive {
		return fmt.Errorf("download do Tor excedeu o limite de %d bytes", MaxArchive)
	}
	return nil
}

// ExtractSelected deliberately ignores everything except tor.exe and GeoIP
// databases. A selected entry must be a regular file at its exact safe path.
func ExtractSelected(archivePath, destination string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	found := make(map[string]bool)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(filepath.ToSlash(h.Name), "./")
		if _, wanted := selectedFiles[name]; !wanted {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return fmt.Errorf("entrada nao regular recusada: %s", name)
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("caminho inseguro no pacote: %s", name)
		}
		out := filepath.Join(destination, clean)
		rel, err := filepath.Rel(destination, out)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("caminho escapou do destino: %s", name)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return err
		}
		wf, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(wf, io.LimitReader(tr, h.Size))
		closeErr := wf.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		found[name] = true
	}
	for name := range selectedFiles {
		if !found[name] {
			return fmt.Errorf("arquivo obrigatorio ausente: %s", name)
		}
	}
	return nil
}

func validInstall(p Paths) bool {
	checks := map[string]string{p.Executable: selectedFiles["tor/tor.exe"], p.GeoIP: selectedFiles["data/geoip"], p.GeoIPv6: selectedFiles["data/geoip6"]}
	for file, expected := range checks {
		if verifyFile(file, expected) != nil {
			return false
		}
	}
	return true
}

func verifyFile(name, expected string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("esperado %s, obtido %s", expected, got)
	}
	return nil
}
