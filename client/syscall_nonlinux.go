//go:build !linux

package client

import (
	"net"
	"time"
)

func setTCPUserTimeout(_ net.Conn, _ time.Duration) error {
	return nil
}
