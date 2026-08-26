# Discord-Tor

Baseado no [GoLiveBypass original](https://github.com/bezumiya/GoLiveBypass).
Adaptei a ideia para uma versao que considero mais segura para meu uso pessoal.
Creditos ao projeto original — e quem quiser usar, fique a vontade!

Versao pequena do launcher que nao injeta nem altera arquivos do Discord. Somente
`discord.gg` e seus subdominios passam por um SOCKS5 do Tor local; WebRTC,
`discord.media`, CDN e todo o restante ficam `DIRECT`.

## Fedora (Discord Flatpak)

Dependencias: `tor`, `flatpak`, `curl`, `python3` e `flock`. Neste computador elas
ja estao instaladas.

Execute:

```bash
./golive-minimal/run-fedora.sh
```

O comando inicia o Tor sem root, comprova SOCKS5 + TLS ate
`gateway.discord.gg`, reinicia `com.discordapp.Discord` com um PAC servido apenas
em loopback e permanece aberto. Fechar o Discord encerra o Tor; `Ctrl+C` encerra
ambos.

Arquivos persistentes ficam em `${XDG_STATE_HOME:-$HOME/.local/state}/golive-tor`.
Para remover o estado, feche o launcher e apague somente essa pasta.

## Windows

O codigo e as instrucoes do launcher nativo estao em
[`golive-minimal/README.md`](golive-minimal/README.md).

## Limites

- Esta e uma adaptacao para uso pessoal, nao uma auditoria ou garantia de seguranca.
- A saida para a internet acontece por relays publicos da rede Tor. Nao existe
  proxy privado ou servidor pertencente a este projeto.
- A midia nao passa pelo Tor.
- O uso pode contrariar os termos do Discord por contornar uma restricao regional.
- O executavel Windows local nao tem assinatura Authenticode.
