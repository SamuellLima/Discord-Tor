package discord

import (
	"strings"
	"testing"
)

func TestChromiumArgsKeepWebRTCOffTheSOCKSPath(t *testing.T) {
	got := ChromiumArgs("http://127.0.0.1:9/discord-tor.pac")
	if len(got) != 2 {
		t.Fatalf("got %d args, want 2", len(got))
	}
	if got[0] != "--proxy-pac-url=http://127.0.0.1:9/discord-tor.pac" {
		t.Fatalf("unexpected PAC arg %q", got[0])
	}
	if got[1] != "--force-webrtc-ip-handling-policy=default_public_interface_only" {
		t.Fatalf("unexpected WebRTC arg %q", got[1])
	}
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "disable_non_proxied_udp") {
		t.Fatal("must not force disable_non_proxied_udp")
	}
}
