//go:build windows

package main

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"

	"discord-tor/internal/portkill"
)

func freeConflictingTor(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("endereco SOCKS invalido: %w", err)
	}
	if !portInUse(address) {
		debugf("DEBUG", "porta %s livre", port)
		logStamp()
		return nil
	}

	debugf("DEBUG", "porta %s em uso; encerrando Tor conflitante", port)
	logStamp()

	pids := listeningPIDs(port)
	if len(pids) == 0 {
		debugf("DEBUG", "PID da porta %s nao encontrado; encerrando tor.exe", port)
		if err := killImage("tor.exe"); err != nil {
			debugf("DEBUG", "taskkill tor.exe: %v", err)
		}
	} else {
		for _, pid := range pids {
			debugf("DEBUG", "encerrando PID %s na porta %s", pid, port)
			if err := killPID(pid); err != nil {
				debugf("DEBUG", "taskkill PID %s: %v", pid, err)
			}
		}
	}

	deadline := time.Now().Add(8 * time.Second)
	for portInUse(address) && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if portInUse(address) {
		return fmt.Errorf("porta %s continua em uso apos encerrar o Tor conflitante", port)
	}
	debugf("DEBUG", "porta %s liberada", port)
	return nil
}

func listeningPIDs(port string) []string {
	cmd := exec.Command("netstat.exe", "-ano", "-p", "tcp")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		debugf("DEBUG", "netstat falhou: %v", err)
		return nil
	}
	return portkill.ListeningPIDsFromNetstat(string(out), port)
}

func killPID(pid string) error {
	cmd := exec.Command("taskkill.exe", "/F", "/PID", pid)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func killImage(name string) error {
	cmd := exec.Command("taskkill.exe", "/F", "/IM", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func portInUse(address string) bool {
	c, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
