//go:build windows

package main

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"net"
	"runtime"
	"syscall"
)

func (s *ServerNode) Listen() error {
	iSysType := 0
	sysType := runtime.GOOS
	switch sysType {
	case "linux":
		iSysType = 1
	case "windows":
		iSysType = 2
	default:
		iSysType = 1
	}
	lc := net.ListenConfig{
		//this control op call after param connection created and before bind op.
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				//control resuse for ip and port.
				if iSysType == 1 {
					syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
					syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				} else if iSysType == 2 {
					syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, windows.SO_REUSEADDR, 1)
					syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, windows.SO_REUSEPORT, 1)
				}

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
