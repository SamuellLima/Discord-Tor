package torcheck

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// Gateway proves both that the endpoint speaks unauthenticated SOCKS5 and that
// a valid, hostname-checked TLS connection reaches Discord through it.
func Gateway(ctx context.Context, socksAddress string) error {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", socksAddress)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(8 * time.Second)
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return err
	}
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return err
	}
	if greeting != [2]byte{5, 0} {
		return fmt.Errorf("resposta SOCKS5 invalida")
	}

	host := []byte("gateway.discord.gg")
	request := append([]byte{5, 1, 0, 3, byte(len(host))}, host...)
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], 443)
	request = append(request, port[:]...)
	if _, err := conn.Write(request); err != nil {
		return err
	}
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return err
	}
	if head[0] != 5 || head[1] != 0 {
		return fmt.Errorf("Tor recusou o gateway (codigo %d)", head[1])
	}
	var addressLen int
	switch head[3] {
	case 1:
		addressLen = 4
	case 4:
		addressLen = 16
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return err
		}
		addressLen = int(n[0])
	default:
		return fmt.Errorf("tipo de endereco SOCKS5 invalido")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addressLen+2)); err != nil {
		return err
	}

	tlsConn := tls.Client(conn, &tls.Config{ServerName: "gateway.discord.gg", MinVersion: tls.VersionTLS12})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("TLS do gateway nao conferiu: %w", err)
	}
	return nil
}
