package pac

// Script routes only Discord's gateway/signalling domain through the local Tor
// SOCKS port. Media (*.discord.media), CDN and every other request stay direct.
func Script(socksAddress string) string {
	return `function FindProxyForURL(url, host) {
    host = host.toLowerCase();
    if (host === "discord.gg" || dnsDomainIs(host, ".discord.gg")) {
        return "SOCKS5 ` + socksAddress + `";
    }
    return "DIRECT";
}
`
}
