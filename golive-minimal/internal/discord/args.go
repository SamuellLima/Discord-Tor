package discord

// ChromiumArgs starts Discord with the local PAC while keeping WebRTC UDP on
// the public network interface.
//
// Watching Go Live opens a second peer connection. Chromium otherwise
// disables non-proxied UDP whenever a PAC exists; Tor cannot carry UDP, so
// the viewer spinner never ends even though voice on the first connection
// still works.
func ChromiumArgs(pacURL string) []string {
	return []string{
		"--proxy-pac-url=" + pacURL,
		"--force-webrtc-ip-handling-policy=default_public_interface_only",
	}
}
