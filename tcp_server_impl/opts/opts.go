package opts

import (
	"net_tcp_server_demo/util"
	"time"
)

type ServerOptions struct {
	MaxReceiveMessageSize int
	MaxSendMessageSize    int
	ConnectionTimeout     time.Duration //120 * time.Second
	WriteBufferSize       int
	ReadBufferSize        int
	RecvBufferPool        util.SharedBufferPool
}

type IServerOption interface {
	Apply(*ServerOptions)
}
