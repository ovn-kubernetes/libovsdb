//go:build linux

package client

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// setTCPUserTimeout limits how long transmitted data may remain unacknowledged.
// Together with inactivity probes, this turns a silent network failure into a disconnect.
func setTCPUserTimeout(conn net.Conn, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}

	if tlsConn, ok := conn.(*tls.Conn); ok {
		conn = tlsConn.NetConn()
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	const maxTCPUserTimeoutMillis = int64(1<<31 - 1)
	timeoutMillis := timeout.Milliseconds()
	if timeoutMillis == 0 {
		timeoutMillis = 1
	}
	if timeoutMillis > maxTCPUserTimeoutMillis {
		return fmt.Errorf("TCP_USER_TIMEOUT %s exceeds %dms", timeout, maxTCPUserTimeoutMillis)
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw TCP connection: %w", err)
	}
	var socketErr error
	if err := rawConn.Control(func(fd uintptr) {
		socketErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(timeoutMillis))
	}); err != nil {
		return fmt.Errorf("access TCP socket: %w", err)
	}
	if socketErr != nil {
		return fmt.Errorf("set TCP_USER_TIMEOUT: %w", socketErr)
	}
	return nil
}
