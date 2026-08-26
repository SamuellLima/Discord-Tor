package torbundle

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSelectedRejectsLinkForSelectedFile(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "tor/tor.exe", Typeflag: tar.TypeSymlink, Linkname: "../../evil"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ExtractSelected(archive, t.TempDir()); err == nil {
		t.Fatal("expected unsafe link to be rejected")
	}
}

func TestVerifyFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(file, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	const abcSHA = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := verifyFile(file, abcSHA); err != nil {
		t.Fatal(err)
	}
	if err := verifyFile(file, ArchiveSHA256); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestOfficialBundleFixture(t *testing.T) {
	archive := os.Getenv("TOR_BUNDLE_FIXTURE")
	if archive == "" {
		t.Skip("TOR_BUNDLE_FIXTURE not set")
	}
	if err := verifyFile(archive, ArchiveSHA256); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := ExtractSelected(archive, destination); err != nil {
		t.Fatal(err)
	}
	p := Paths{
		Root: destination, Executable: filepath.Join(destination, "tor", "tor.exe"),
		GeoIP: filepath.Join(destination, "data", "geoip"), GeoIPv6: filepath.Join(destination, "data", "geoip6"),
	}
	if !validInstall(p) {
		t.Fatal("extracted official bundle did not match pinned hashes")
	}
}
