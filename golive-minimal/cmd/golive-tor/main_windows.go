//go:build windows

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/bezumiya/Discord-Tor/golive-minimal/internal/discord"
	"github.com/bezumiya/Discord-Tor/golive-minimal/internal/pac"
	"github.com/bezumiya/Discord-Tor/golive-minimal/internal/torbundle"
	"github.com/bezumiya/Discord-Tor/golive-minimal/internal/torcheck"
)

const socksAddress = "127.0.0.1:9060"

var logFile *os.File

const startupArt = `                             %@@@@@@@@@@@@
                        @@@@@+-.        -=%@@@@@#
                     *@@@=     +@@@@@@@+.      +#@@@@
                   @@@*   .@@@@@@@@@@@@@@@@@@@.    +@@@+
                -@@@.   @@@@@@@@@@@@@@@@@@@@@@@@@@    #@@@
               @@%   #@@%@@@@@@@@@@@@@@@@@@@@@@@@@@@@*  *@@
              @@.  @@@@      @@@@@@@@@@@@@@@@@@@@@@@@@@  +@@
            @@@:  @@@   @@@@  @@@@@@@@@@@@@@@@@@@@@@@@@@  %@
        =@@@+:        @@@@@@@  +@@@@@@@@@@@.       @@@@@  :@@
       @@#          #@@@@@@@@@  %@@@@@@+    =@%@@=  @@@@%  @@
     .@@          .@@@@@@@@@@@## .#  .  .@@@@@@@@@.  @@@@  @@
     @@          +@@@@@@@@@@###@###@###@@@@@@@@@@@%  @@@#  @@
     @*         .@@@@@@@@@###%###%%##%@@@@@@@@@@@@-       :@@@
     @+         @@@@@@@@@@@@@###@###@@@@@@@@@@@@            .@@@
     @+        %@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@.               *@@
     @@        %@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@                 #@
     %@@       %@#+*@@@@@@@@@@@@@@@@@@@@@@@@@@                 #@
       @@@.     @=    @: :@@ @@@@@@@@@@@@@@@@@                 #@
         @@@@@+  +@@@@@@          +@      @@@@@                @@
            @#      :-@@@@@@@@****%@@@@@@@@@@@=               %@@
            @%  .@@%=++=@@@@@@@@@@@@@@++==                  =@@=
            @@@:  @@@@@@@@@@@@@@@@@@@@@@@@@@@@@@=  +%:..=@@@@%
            @@+  %@@@@@@@@@@@@@@@@@@@@@@@@@@     .@@@++++
         -@@@*  @@@@@@@@@@@@@@@@@@@@@@@@@@@@  #@@@@
        @@#    %@@@@@@@@#####@@@@@@@@@@@@@@  +@@
        @@  @@@@@@@@@@%@#@@#%@@@@@@@@@@@@@  :@@
        @@@                ======@@@@@@@:  #@@
          @@@@@@@@@@@@@###+:..::    .@%   @@@
                          @@@@@@@  @   .@@@
                               =@@   *@@@
                                 %@@@@`

func main() {
	fmt.Printf("\n\n%s\n\n\n\n%s\n\n", startupArt, startupArt)

	if alreadyRunning() {
		messageBox("Discord-Tor", "O launcher ja esta em execucao.", 0x40)
		return
	}
	if err := run(); err != nil {
		log.Printf("erro: %v", err)
		messageBox("Discord-Tor - erro", err.Error(), 0x10)
	}
}

func run() error {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return errors.New("LOCALAPPDATA nao esta definido")
	}
	root := filepath.Join(local, "GoLiveBypassTor")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	var err error
	logFile, err = os.OpenFile(filepath.Join(root, "launcher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		defer logFile.Close()
		log.SetOutput(logFile)
	}
	log.Printf("iniciando")

	install, err := discord.FindPreferred(local)
	if err != nil {
		return err
	}
	log.Printf("cliente encontrado: %s", install.Flavor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDownloadNotice := func() {
		messageBox("Discord-Tor", "Primeira execucao: vou baixar 21 MB do Tor Project e conferir o SHA-256 antes de executar.\n\nO Discord sera reiniciado em seguida.", 0x40)
	}
	torPaths, err := torbundle.Ensure(ctx, root, firstDownloadNotice)
	if err != nil {
		return fmt.Errorf("preparar Tor: %w", err)
	}

	if portInUse(socksAddress) {
		return fmt.Errorf("a porta local 9060 ja esta em uso; feche outro Tor/GoLive antes de continuar")
	}
	torCmd, torDone, err := startTor(torPaths, root)
	if err != nil {
		return err
	}
	defer func() {
		if torCmd.Process != nil {
			_ = torCmd.Process.Kill()
		}
	}()

	if err := waitForTor(ctx, torDone); err != nil {
		return err
	}
	log.Printf("Tor entregando gateway.discord.gg com TLS valido")

	pacURL, stopPAC, err := servePAC()
	if err != nil {
		return err
	}
	defer stopPAC()

	if err := stopDiscord(install.ProcessName); err != nil {
		return err
	}
	discordDone, err := startDiscord(install, pacURL)
	if err != nil {
		return err
	}
	log.Printf("%s iniciado com PAC local", install.Flavor)
	fmt.Printf("\n[OK] Tor iniciado e %s aberto.\n", install.Flavor)
	fmt.Println("Mantenha este terminal aberto enquanto estiver usando o Discord.")
	fmt.Println("Quando terminar, feche o Discord e este terminal sera encerrado automaticamente.")

	// If an update hands over to a new executable, do not leave that unverified
	// process running. Close it, rediscover the current version and relaunch it
	// with the same local PAC. Three handovers avoid an accidental restart loop.
	handovers := 0
	for {
		select {
		case err := <-torDone:
			if err == nil {
				return errors.New("Tor encerrou inesperadamente")
			}
			return fmt.Errorf("Tor encerrou inesperadamente: %w", err)
		case <-discordDone:
			time.Sleep(2 * time.Second)
			if !processRunning(install.ProcessName) {
				log.Printf("Discord encerrado; finalizando Tor")
				return nil
			}
			handovers++
			if handovers > 3 {
				return errors.New("Discord reiniciou repetidamente; feche-o e execute o launcher outra vez")
			}
			log.Printf("Discord reiniciou; reaplicando PAC local")
			if err := stopDiscord(install.ProcessName); err != nil {
				return err
			}
			install, err = discord.FindPreferred(local)
			if err != nil {
				return err
			}
			discordDone, err = startDiscord(install, pacURL)
			if err != nil {
				return err
			}
		}
	}
}

func startDiscord(install discord.Installation, pacURL string) (<-chan error, error) {
	cmd := exec.Command(install.Executable, "--proxy-pac-url="+pacURL)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("abrir %s: %w", install.Flavor, err)
	}
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

	cmd := exec.Command(p.Executable, "-f", torrcPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if logFile != nil {
		cmd.Stdout, cmd.Stderr = logFile, logFile
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("iniciar Tor: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return cmd, done, nil
}

func waitForTor(ctx context.Context, done <-chan error) error {
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("Tor encerrou durante o bootstrap: %w", err)
		case <-deadline.C:
			return errors.New("Tor nao completou o bootstrap em 2 minutos")
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := torcheck.Gateway(probeCtx, socksAddress)
			cancel()
			if err == nil {
				return nil
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
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(pac.Script(socksAddress)))
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	return "http://" + ln.Addr().String() + "/discord-tor.pac", stop, nil
}

func stopDiscord(processName string) error {
	if !processRunning(processName) {
		return nil
	}
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
	return nil
}

func processRunning(processName string) bool {
	cmd := exec.Command("tasklist.exe", "/FI", "IMAGENAME eq "+processName, "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
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

func portInUse(address string) bool {
	c, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	user32       = syscall.NewLazyDLL("user32.dll")
	createMutexW = kernel32.NewProc("CreateMutexW")
	messageBoxW  = user32.NewProc("MessageBoxW")
)

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
