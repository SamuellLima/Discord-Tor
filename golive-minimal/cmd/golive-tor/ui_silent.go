//go:build windows && silent

package main

func debugf(tag, format string, args ...any) {}

func notifyStarted() {
	messageBox("Discord-Tor", "Discord-Tor Iniciado", 0x40)
}

func initConsole() {}
