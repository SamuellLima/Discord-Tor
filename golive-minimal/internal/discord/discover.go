package discord

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Installation struct {
	Flavor      string
	Executable  string
	ProcessName string
}

var flavors = []struct {
	dir, exe, label string
}{
	{"Discord", "Discord.exe", "Discord"},
	{"DiscordPTB", "DiscordPTB.exe", "Discord PTB"},
	{"DiscordCanary", "DiscordCanary.exe", "Discord Canary"},
}

// FindPreferred returns Stable first, then PTB, then Canary. Within a flavor,
// only the newest complete app-* directory is selected.
func FindPreferred(localAppData string) (Installation, error) {
	for _, flavor := range flavors {
		root := filepath.Join(localAppData, flavor.dir)
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Installation{}, fmt.Errorf("ler %s: %w", root, err)
		}

		var bestName, bestExe string
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "app-") {
				continue
			}
			exe := filepath.Join(root, entry.Name(), flavor.exe)
			info, err := os.Stat(exe)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if bestName == "" || compareVersions(entry.Name()[4:], bestName[4:]) > 0 {
				bestName, bestExe = entry.Name(), exe
			}
		}
		if bestExe != "" {
			return Installation{Flavor: flavor.label, Executable: bestExe, ProcessName: flavor.exe}, nil
		}
	}
	return Installation{}, fmt.Errorf("Discord Stable, PTB ou Canary nao encontrado em %%LOCALAPPDATA%%")
}

func compareVersions(a, b string) int {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	n := len(aa)
	if len(bb) > n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av, _ = strconv.Atoi(aa[i])
		}
		if i < len(bb) {
			bv, _ = strconv.Atoi(bb[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return strings.Compare(a, b)
}
