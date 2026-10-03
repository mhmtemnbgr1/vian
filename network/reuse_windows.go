//go:build windows

package network

import (
	"syscall"
)

// reuseAddr aynı UDP portunu birden fazla süreç dinleyebilsin diye SO_REUSEADDR açar.
func reuseAddr(network, address string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
