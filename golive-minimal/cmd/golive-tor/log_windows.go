//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var userLog *os.File

func documentsLogDir() (string, error) {
	if p, err := knownDocuments(); err == nil && p != "" {
		return filepath.Join(p, "Discord-Tor"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents", "Discord-Tor"), nil
}

func openUserLog() string {
	dir, err := documentsLogDir()
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, "discord-tor.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ""
	}
	userLog = f
	return path
}

func logStamp() {
	writeUserLog(time.Now().Format("15:04:05") + "\n")
}

func logErr(msg string) {
	writeUserLog(time.Now().Format("15:04:05") + " ERRO " + msg + "\n")
}

func writeUserLog(line string) {
	if userLog == nil {
		return
	}
	_, _ = userLog.WriteString(line)
	_ = userLog.Sync()
}

func knownDocuments() (string, error) {
	var p *uint16
	hr, _, _ := shGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(&folderDocuments)),
		0,
		0,
		uintptr(unsafe.Pointer(&p)),
	)
	if hr != 0 || p == nil {
		return "", syscall.Errno(hr)
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(p))[:]), nil
}

var folderDocuments = syscall.GUID{
	Data1: 0xFDD39AD0,
	Data2: 0x238F,
	Data3: 0x46AF,
	Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7},
}

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	ole32                = syscall.NewLazyDLL("ole32.dll")
	shGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")
	coTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)
