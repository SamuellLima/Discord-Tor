#!/usr/bin/env python3
import http.server
import sys
import time


if len(sys.argv) != 2:
    raise SystemExit("usage: pac_server.py HOST:PORT")

socks_address = sys.argv[1]
pac = f'''function FindProxyForURL(url, host) {{
    host = host.toLowerCase();
    if (host === "discord.gg" || dnsDomainIs(host, ".discord.gg")) {{
        return "SOCKS5 {socks_address}";
    }}
    return "DIRECT";
}}
'''.encode("ascii")


def debug(message):
    sys.stderr.write("[%s] [DEBUG] %s\n" % (time.strftime("%H:%M:%S"), message))
    sys.stderr.flush()


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        debug("PAC %s %s de %s" % (self.command, self.path, self.client_address[0]))
        if self.path != "/discord-tor.pac":
            debug("PAC 404 %s" % self.path)
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/x-ns-proxy-autoconfig")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(pac)))
        self.end_headers()
        self.wfile.write(pac)

    def log_message(self, _format, *args):
        debug(_format % args)

    def log_error(self, _format, *args):
        sys.stderr.write("[%s] [ERRO] PAC %s\n" % (time.strftime("%H:%M:%S"), _format % args))
        sys.stderr.flush()


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
debug("PAC escutando em 127.0.0.1:%d" % server.server_port)
print(server.server_port, flush=True)
server.serve_forever()
