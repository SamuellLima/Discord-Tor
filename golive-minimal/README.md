# Discord-Tor para Windows

Este repositorio partiu de um projeto ja pronto. Eu apenas fiz uma versao que
considero mais segura para meu uso pessoal e resolvi deixa-la disponivel. Quem
quiser usar, fica a vontade, ta em casa; por sua propria conta e risco, sem garantia ou
suporte prometido.

Launcher Windows nativo e sem interface web. Ele **nao injeta nada no Discord**.

Para Fedora com Discord Flatpak, execute a partir da raiz do repositorio:

```bash
./golive-minimal/run-fedora.sh
```

Essa variante usa o Tor instalado pelo sistema e permanece no terminal enquanto
o Discord estiver aberto.

Ao abrir `GoLiveTor.exe`, o launcher:

1. abre o CMD e exibe a arte ASCII de inicializacao;
2. encontra Discord Stable (ou PTB/Canary);
3. na primeira execucao, baixa o Tor Expert Bundle oficial 15.0.20;
4. confere o SHA-256 fixado e extrai somente `tor.exe`, `geoip` e `geoip6`;
5. inicia o Tor em `127.0.0.1:9060`;
6. prova um tunel SOCKS + TLS valido ate `gateway.discord.gg`;
7. reinicia o Discord com um PAC local;
8. mantem somente `discord.gg` e `*.discord.gg` no Tor; todo o resto fica `DIRECT`;
9. reaplica o PAC se uma atualizacao reiniciar o Discord;
10. encerra o Tor quando o Discord fecha.

Nao existem Electron, renderer, `app.asar`, proxy publica, telemetry, report de bug,
updater, autostart, servidor externo proprio ou execucao de shell. Os unicos executaveis
chamados sao `tor.exe`, o Discord encontrado, `tasklist.exe` e `taskkill.exe`, sempre com
argumentos separados.

## Build reproduzivel

Requer Go 1.26.5:

```powershell
cd golive-minimal
go test ./...
$env:CGO_ENABLED = '0'
go build -trimpath -buildvcs=true -ldflags="-s -w" -o GoLiveTor.exe ./cmd/golive-tor
Get-FileHash -Algorithm SHA256 .\GoLiveTor.exe
```

O programa usa somente a biblioteca padrao do Go; `go.mod` nao possui dependencias.

Cross-build em Linux:

```sh
cd golive-minimal
go test ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -buildvcs=true \
  -ldflags='-s -w' -o GoLiveTor.exe ./cmd/golive-tor
sha256sum GoLiveTor.exe
```

## Estado local

Os arquivos ficam em `%LOCALAPPDATA%\GoLiveBypassTor`:

- Tor oficial e bases GeoIP fixados por hash;
- `torrc` e estado do Tor;
- `launcher.log`, sem URLs autenticadas nem credenciais.

Para remover, feche Discord e `GoLiveTor.exe` e apague essa pasta. O Discord nunca e alterado.

## Limites importantes

- O launcher precisa continuar rodando enquanto o Discord estiver aberto.
- O launcher tenta reaplicar o PAC automaticamente apos uma atualizacao. Se o Discord entrar em
  um ciclo anormal de reinicios, ele para e pede que voce execute o launcher novamente.
- O binario produzido localmente nao possui assinatura Authenticode. Para distribuicao publica,
  assine o artefato e publique o SHA-256 por um canal independente.
- Ainda existe risco de violacao dos termos do Discord por contornar uma restricao regional,
  embora esta variante nao modifique o cliente.
