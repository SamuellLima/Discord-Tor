package banner

import (
	"strings"
	"testing"
)

func TestArtContainsMarkAndName(t *testing.T) {
	if !strings.Contains(Art, "⣿") {
		t.Fatal("banner art is missing the braille mark")
	}
	if !strings.Contains(Art, "██████╗") {
		t.Fatal("banner art is missing the BRASA lettering")
	}
}
