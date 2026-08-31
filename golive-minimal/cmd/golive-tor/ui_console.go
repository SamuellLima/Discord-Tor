//go:build windows && !silent

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"discord-tor/internal/banner"
)

func debugf(tag, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] [%s] %s\n", time.Now().Format("15:04:05"), tag, msg)
}

func showBanner() {
	fmt.Printf("\n%s\n\n", strings.TrimRight(banner.Art, "\n"))
	debugf("DEBUG", "launcher em modo CMD (debug)")
}

func notifyStarted() {}

func initConsole() {
	enableUTF8Console()
	showBanner()
}
