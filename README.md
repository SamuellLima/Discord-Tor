# Discord-Tor

<p align="center">
  Escolha o idioma da documentação<br/>
  <em>Choose a language · Elige el idioma</em>
</p>

<p align="center">
  <a href="docs/pt-br/README.md"><img src="https://img.shields.io/badge/PT--BR-Português-009c3b?style=for-the-badge" alt="PT-BR" /></a>
  &nbsp;
  <a href="docs/en/README.md"><img src="https://img.shields.io/badge/EN-English-002868?style=for-the-badge" alt="EN" /></a>
  &nbsp;
  <a href="docs/es/README.md"><img src="https://img.shields.io/badge/ES-Español-c60b1e?style=for-the-badge" alt="ES" /></a>
</p>

<p align="center">
  <a href="docs/pt-br/README.md">Português (Brasil)</a> ·
  <a href="docs/en/README.md">English</a> ·
  <a href="docs/es/README.md">Español</a>
</p>

---

O **Discord-Tor** é um launcher mínimo que encaminha somente o tráfego de sinalização do Discord (`discord.gg` e subdomínios) por um proxy SOCKS5 Tor local. O cliente não é injetado nem alterado: mídia, CDN, WebRTC e o restante da rede permanecem em conexão direta. Há variantes para Windows (launcher nativo em Go) e Fedora (Discord Flatpak).

**Discord-Tor** is a minimal launcher that routes only Discord signalling traffic (`discord.gg` and subdomains) through a local Tor SOCKS5 proxy. The client is neither injected nor modified: media, CDN, WebRTC and the rest of the network stay on a direct connection. Variants exist for Windows (native Go launcher) and Fedora (Discord Flatpak).

**Discord-Tor** es un launcher mínimo que encamina únicamente el tráfico de señalización de Discord (`discord.gg` y subdominios) por un proxy SOCKS5 Tor local. El cliente no se inyecta ni se modifica: media, CDN, WebRTC y el resto de la red permanecen en conexión directa. Hay variantes para Windows (launcher nativo en Go) y Fedora (Discord Flatpak).

---

Baseado no [GoLiveBypass original](https://github.com/bezumiya/GoLiveBypass).
Adaptei a ideia para uma versão que considero mais segura para meu uso pessoal.
Créditos ao projeto original.

Based on the [original GoLiveBypass](https://github.com/bezumiya/GoLiveBypass).
I adapted the idea into a version I consider safer for personal use.
Credit to the original project.

Basado en el [GoLiveBypass original](https://github.com/bezumiya/GoLiveBypass).
Adapté la idea a una versión que considero más segura para uso personal.
Créditos al proyecto original.

---

<p align="center">
  O uso pode conflitar com os <a href="https://discord.com/terms">Termos de Serviço do Discord</a>
  ao contornar uma restrição regional — o cliente não é modificado, mas o encaminhamento
  do gateway por Tor pode ser interpretado como violação das regras da plataforma.<br/>
  Use may conflict with the <a href="https://discord.com/terms">Discord Terms of Service</a>
  by circumventing a regional restriction — the client is not modified, but routing the
  gateway through Tor may still be treated as a platform-rules violation.<br/>
  El uso puede entrar en conflicto con los <a href="https://discord.com/terms">Términos de Servicio de Discord</a>
  al eludir una restricción regional — el cliente no se modifica, pero encaminar el
  gateway por Tor puede interpretarse como una infracción de las reglas de la plataforma.
</p>
