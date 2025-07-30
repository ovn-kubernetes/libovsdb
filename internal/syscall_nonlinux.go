//go:build !linux
// +build !linux

package internal

import (
	"net"
	"time"
)

// SetTCPUserTimeout is a no-op function under non-linux environments.
func SetTCPUserTimeout(conn net.Conn, timeout time.Duration) error {
	return nil
}
