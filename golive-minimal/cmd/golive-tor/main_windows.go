//go:build windows

// Ícone dos executáveis: gerado a partir de icon/icon.png
//
//	go run github.com/tc-hib/go-winres@v0.3.3 simply --icon ../../../icon/icon.png --arch amd64 --manifest none --product-name Discord-Tor --file-description Discord-Tor --original-filename DiscordTor.exe --out rsrc

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"discord-tor/internal/discord"
	"discord-tor/internal/pac"
	"discord-tor/internal/torbundle"
	"discord-tor/internal/torcheck"
)

var socksAddress = os.Getenv("DISCORD_TOR_SOCKS_ADDRESS")
if socksAddress == "" {
	socksAddress = "127.0.0.1:9060" // Fallback to default if not set in env
}

func main() {
	initConsole()

	logPath := openUserLog()
	if userLog != nil {
		defer userLog.Close()
	}
	if logPath != "" {
		debugf("DEBUG", "arquivo de log: %s", logPath)
	} else {
		debugf("DEBUG", "nao foi possivel abrir o log em Documentos/Discord-Tor")
	}
	logStamp()

	if alreadyRunning() {
		logErr("O launcher ja esta em execucao.")
		debugf("ERRO", "O launcher ja esta em execucao.")
		messageBox("Discord-Tor", "O launcher ja esta em execucao.", 0x40)
		return
	}

	notifyStarted()

	if err := run(); err != nil {
		logErr(err.Error())
		debugf("ERRO", "%v", err)
		messageBox("Discord-Tor - erro", err.Error(), 0x10)
	}
}

func run() error {
	debugf("DEBUG", "resolvendo LOCALAPPDATA")
	logStamp()
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return errors.New("LOCALAPPDATA nao esta definido")
	}
	root := filepath.Join(local, "GoLiveBypassTor")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("criar estado local: %w", err)
	}
	debugf("DEBUG", "estado local: %s", root)

	debugf("DEBUG", "procurando Discord Stable/PTB/Canary")
	logStamp()
	install, err := discord.FindPreferred(local)
	if err != nil {
		return err
	}
	debugf("OK", "cliente encontrado: %s (%s)", install.Flavor, install.Executable)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDownloadNotice := func() {
		debugf("DEBUG", "primeira execucao: baixando Tor Expert Bundle (~21 MB) e conferindo SHA-256")
		messageBox("Discord-Tor", "Primeira execucao: vou baixar 21 MB do Tor Project e conferir o SHA-256 antes de executar.\n\nO Discord sera reiniciado em seguida.", 0x40)
	}
	debugf("DEBUG", "preparando bundle do Tor")
	logStamp()
	torPaths, err := torbundle.Ensure(ctx, root, firstDownloadNotice)
	if err != nil {
		return fmt.Errorf("preparar Tor: %w", err)
	}
	debugf("OK", "Tor pronto: %s", torPaths.Executable)

	debugf("DEBUG", "verificando conflito na porta SOCKS %s", socksAddress)
	logStamp()
	if err := freeConflictingTor(socksAddress); err != nil {
		return err
	}

	debugf("DEBUG", "iniciando Tor local em %s", socksAddress)
	logStamp()
	torCmd, torDone, err := startTor(torPaths, root)
	if err != nil {
		return err
	}
	defer func() {
		debugf("DEBUG", "encerrando processo Tor")
		if torCmd.Process != nil {
			_ = torCmd.Process.Kill()
		}
	}()
	debugf("OK", "processo Tor iniciado (pid %d)", torCmd.Process.Pid)

	debugf("DEBUG", "aguardando SOCKS5 + TLS ate gateway.discord.gg")
	logStamp()
	if err := waitForTor(ctx, torDone); err != nil {
		return err
	}
	debugf("OK", "Tor confirmado: gateway.discord.gg com TLS valido")

	debugf("DEBUG", "subindo servidor PAC em loopback")
	logStamp()
	pacURL, stopPAC, err := servePAC()
	if err != nil {
		return err
	}
	defer func() {
		debugf("DEBUG", "encerrando servidor PAC")
		stopPAC()
	}()
	debugf("OK", "PAC local em %s", pacURL)

	debugf("DEBUG", "encerrando %s se estiver aberto", install.ProcessName)
	logStamp()
	if err := stopDiscord(install.ProcessName); err != nil {
		return err
	}

	debugf("DEBUG", "abrindo %s com PAC e politica WebRTC", install.Flavor)
	logStamp()
	discordDone, err := startDiscord(install, pacURL)
	if err != nil {
		return err
	}
	debugf("OK", "%s aberto. Apenas discord.gg passa pelo Tor; midia fica direta.", install.Flavor)
	debugf("INFO", "Mantenha este processo aberto. Feche o Discord para encerrar.")

	handovers := 0
	for {
		select {
		case err := <-torDone:
			if err == nil {
				return errors.New("Tor encerrou inesperadamente")
			}
			return fmt.Errorf("Tor encerrou inesperadamente: %w", err)
		case <-discordDone:
			debugf("DEBUG", "processo Discord saiu; conferindo se houve atualizacao")
			time.Sleep(2 * time.Second)
			if !processRunning(install.ProcessName) {
				debugf("INFO", "Discord encerrado; finalizando Tor")
				logStamp()
				return nil
			}
			handovers++
			if handovers > 3 {
				return errors.New("Discord reiniciou repetidamente; feche-o e execute o launcher outra vez")
			}
			debugf("DEBUG", "Discord reiniciou (handover %d/3); reaplicando PAC", handovers)
			logStamp()
			if err := stopDiscord(install.ProcessName); err != nil {
				return err
			}
			install, err = discord.FindPreferred(local)
			if err != nil {
				return err
			}
			debugf("DEBUG", "reabrindo %s", install.Flavor)
			discordDone, err = startDiscord(install, pacURL)
			if err != nil {
				return err
			}
		}
	}
}

func startDiscord(install discord.Installation, pacURL string) (<-chan error, error) {
	args := discord.ChromiumArgs(pacURL)
	debugf("DEBUG", "exec %s %s", install.Executable, strings.Join(args, " "))
	cmd := exec.Command(install.Executable, args...)
	cmd.Dir = filepath.Dir(install.Executable)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("abrir %s: %w", install.Flavor, err)
	}
	debugf("OK", "%s pid %d", install.Flavor, cmd.Process.Pid)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return done, nil
}

func startTor(p torbundle.Paths, root string) (*exec.Cmd, <-chan error, error) {
	dataDir := filepath.Join(root, "tor-data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, nil, err
	}
	q := func(s string) string { return `"` + strings.ReplaceAll(filepath.ToSlash(s), `"`, ``) + `"` }
	torrc := strings.Join([]string{
		"SocksPort " + socksAddress,
		"DataDirectory " + q(dataDir),
		"GeoIPFile " + q(p.GeoIP),
		"GeoIPv6File " + q(p.GeoIPv6),
		"AvoidDiskWrites 1",
		"Log notice stdout",
		"",
	}, "\r\n")
	torrcPath := filepath.Join(root, "torrc")
	if err := os.WriteFile(torrcPath, []byte(torrc), 0o600); err != nil {
		return nil, nil, err
	}
	debugf("DEBUG", "torrc em %s", torrcPath)

	cmd := exec.Command(p.Executable, "-f", torrcPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	torLog, openLogErr := os.OpenFile(filepath.Join(root, "tor-stdout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openLogErr != nil {
		debugf("DEBUG", "nao foi possivel abrir tor-stdout.log: %v", openLogErr)
	} else {
		cmd.Stdout, cmd.Stderr = torLog, torLog
	}
	if err := cmd.Start(); err != nil {
		if torLog != nil {
			_ = torLog.Close()
		}
		return nil, nil, fmt.Errorf("iniciar Tor: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		if torLog != nil {
			_ = torLog.Close()
		}
		done <- waitErr
	}()
	return cmd, done, nil
}

func waitForTor(ctx context.Context, done <-chan error) error {
		deadline := time.NewTimer(5 * time.Minute)
		defer deadline.Stop()
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	lastTalk := time.Time{}
	attempt := 0
	for {
		select {
		case err := <-done:
			return fmt.Errorf("Tor encerrou durante o bootstrap: %w", err)
		case <-deadline.C:
			return errors.New("Tor nao completou o bootstrap em 2 minutos")
		case <-ticker.C:
			attempt++
			probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := torcheck.Gateway(probeCtx, socksAddress)
			cancel()
			if err == nil {
				debugf("DEBUG", "prova do gateway ok na tentativa %d", attempt)
				return nil
			}
			if time.Since(lastTalk) >= 5*time.Second {
				debugf("DEBUG", "gateway ainda indisponivel (tentativa %d): %v", attempt, err)
				lastTalk = time.Now()
			}
		}
	}
}

func servePAC() (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/discord-tor.pac", func(w http.ResponseWriter, r *http.Request) {
		debugf("DEBUG", "PAC %s %s de %s", r.Method, r.URL.Path, r.RemoteAddr)
		if r.Method != http.MethodGet {
			debugf("ERRO", "PAC rejeitou metodo %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(pac.Script(socksAddress)))
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			debugf("ERRO", "servidor PAC: %v", err)
		}
	}()
	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second))
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	return "http://" + ln.Addr().String() + "/discord-tor.pac", stop, nil
}

func stopDiscord(processName string) error {
	if !processRunning(processName) {
		debugf("DEBUG", "%s nao estava em execucao", processName)
		return nil
	}
	debugf("DEBUG", "taskkill /F /T /IM %s", processName)
	cmd := exec.Command("taskkill.exe", "/F", "/T", "/IM", processName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fechar %s: %w", processName, err)
	}
	deadline := time.Now().Add(12 * time.Second)
	for processRunning(processName) && time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
	}
	if processRunning(processName) {
		return fmt.Errorf("%s continua aberto", processName)
	}
	debugf("OK", "%s encerrado", processName)
	return nil
}

func processRunning(processName string) bool {
	// Sanitize processName for use in tasklist.exe to prevent injection
	name := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	cleanProcessName := strings.TrimSpace(processName)
	if !name.MatchString(cleanProcessName) {
		// Fallback sanitization for characters that might slip through but are unsafe for command line arguments
		for _, r := range []string{" ", "&", "|", ";", "`", "$"} {
			cleanProcessName = strings.ReplaceAll(cleanProcessName, r, "")
		}
		debugf("WARN", "Nome de processo '%s' contem caracteres inseguros e foi sanitizado para: %s", processName, cleanProcessName)
	} else {
		name = cleanProcessName
	}
	// Use the sanitized name for the command execution
	cmd := exec.Command("tasklist.exe", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		debugf("DEBUG", "tasklist %s: %v", processName, err)
		return false
	}
	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		return false
	}
	for _, record := range records {
		if len(record) > 0 && strings.EqualFold(record[0], processName) {
			return true
		}
	}
	return false
}

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	user32             = syscall.NewLazyDLL("user32.dll")
	createMutexW       = kernel32.NewProc("CreateMutexW")
	messageBoxW        = user32.NewProc("MessageBoxW")
	setConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	setConsoleCP       = kernel32.NewProc("SetConsoleCP")
)

func enableUTF8Console() {
	const utf8 = 65001
	_, _, _ = setConsoleOutputCP.Call(utf8)
	_, _, _ = setConsoleCP.Call(utf8)
}

func alreadyRunning() bool {
	name, _ := syscall.UTF16PtrFromString(`Local\GoLiveBypassTorMinimal`)
	h, _, lastErr := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	return h == 0 || lastErr == syscall.Errno(183)
}

func messageBox(title, text string, flags uintptr) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	messageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
}
