//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package main

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"syscall"
)

func (s *ServerNode) Listen() error {
	lc := net.ListenConfig{
		//this control op call after param connection created and before bind op.
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				//control resuse for ip and port.
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		},
	}
	r, e := lc.Listen(context.Background(), s.config.NetWork, fmt.Sprintf("%v:%d", s.config.Ip, s.config.Port))
	if e != nil {
		log.Printf("listen fail, err: %v", e)
		return e
	}
	s.l = r
	return nil

}
