package pac

import (
	"strings"
	"testing"
)

func TestScriptIsNarrowAndHasNoProxyFallback(t *testing.T) {
	s := Script("127.0.0.1:9060")
	for _, want := range []string{
		`host === "discord.gg"`,
		`dnsDomainIs(host, ".discord.gg")`,
		`SOCKS5 127.0.0.1:9060`,
		`return "DIRECT"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("PAC does not contain %q", want)
		}
	}
	for _, forbidden := range []string{"discord.media", "discord.com", "SOCKS5 127.0.0.1:9060; DIRECT"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("PAC unexpectedly contains %q", forbidden)
		}
	}
}
