package discord

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPreferredSelectsNewestStable(t *testing.T) {
	root := t.TempDir()
	for _, version := range []string{"app-1.0.9", "app-1.0.12", "app-incomplete"} {
		if err := os.MkdirAll(filepath.Join(root, "Discord", version), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, version := range []string{"app-1.0.9", "app-1.0.12"} {
		if err := os.WriteFile(filepath.Join(root, "Discord", version, "Discord.exe"), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := FindPreferred(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "Discord", "app-1.0.12", "Discord.exe")
	if got.Executable != want {
		t.Fatalf("got %q, want %q", got.Executable, want)
	}
}

func TestFindPreferredFallsBackToPTB(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "DiscordPTB", "app-2.3.4")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "DiscordPTB.exe"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := FindPreferred(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Flavor != "Discord PTB" {
		t.Fatalf("unexpected flavor %q", got.Flavor)
	}
}
