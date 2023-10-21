package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net_tcp_server_demo/util"
	"sync"
	"time"
)

var (
	port = flag.Int("port", 8080, "the server port")
)

type ServerConfig struct {
	Port    int
	Ip      string
	NetWork string // "tcp", "tcp4", "tcp6", "unix" or "unixpacket"
}

type ServerNode struct {
	config ServerConfig
	//
	l            net.Listener
	LogicHandles map[uint16]IBusiLogic
}

func (s *ServerNode) Listen() error {
	r, e := net.Listen(s.config.NetWork, fmt.Sprintf("%v:%d", s.config.Ip, s.config.Port))
	if e != nil {
		log.Printf("listen fail, err: %v", e)
		return e
	}
	s.l = r
	return nil

}
func (s *ServerNode) ServerWait() error {
	for {
		var tempDelay time.Duration = 0

		rawConn, e := s.l.Accept()
		if e != nil {
			if ne, ok := e.(interface{ Temporary() bool }); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if m := time.Second; tempDelay > m {
					tempDelay = m
				}

				//
				tmr := time.NewTimer(tempDelay)
				select {
				case <-tmr.C:
					//
					// other finish notify.
				}
				continue
			}
			return e
		}
		tempDelay = 0

		go func() {
			// do new connection logic.
			s.ReadWriteOnConn(s.l.Addr().String(), rawConn)
		}()
	}
	return nil
}
func (s *ServerNode) NewServerConnected(c net.Conn) *ServerConn {
	h := NewPackageHead(c, 1024, 1024, true)

	pl := NewPkgPayLoad(c)

	sc := &ServerConn{
		UnderlyingConn: c,
		Header:         h,
		PayLoad:        pl,
		bufferPool:     util.NewBufferPool(),
	}

	return sc
}
func (s *ServerNode) RecvMsg(sc *ServerConn) error {
	for {
		if sc.Header.hasHeadReadLen >= util.HEAD_LEN_MAX {
			break
		}

		retN, e := io.ReadFull(sc.UnderlyingConn, sc.Header.headBuf[sc.Header.hasHeadReadLen:util.HEAD_LEN_MAX])
		if e != nil && e == io.ErrUnexpectedEOF {
			sc.Header.hasHeadReadLen += int32(retN)
			continue
		}
		if e != nil {
			log.Printf("read head from network fail, err: %v\n", e)
			return e
		}
		sc.Header.hasHeadReadLen += int32(retN)
	}
	sc.Header.hasHeadReadLen = 0
	//
	pkgLen := binary.BigEndian.Uint32(sc.Header.headBuf[:])
	if pkgLen > util.PKG_LEN_MAX {
		log.Printf("get pkg len: %v more than: %v\n", pkgLen, util.PKG_LEN_MAX)
		return fmt.Errorf("pkg len field value more than : %v", util.PKG_LEN_MAX)
	}
	if pkgLen < uint32(util.HEAD_LEN_MAX+util.CMD_TYPE_LEN) {
		log.Println("get pkg len less than: ", util.HEAD_LEN_MAX+util.CMD_TYPE_LEN)
		return fmt.Errorf("pkg len field value less than: %v", util.HEAD_LEN_MAX+util.CMD_TYPE_LEN)
	}
	sc.Header.pkgLen = pkgLen

	for {
		if sc.Header.hasCmdReadLen >= util.CMD_TYPE_LEN {
			break
		}

		retN, e := io.ReadFull(sc.UnderlyingConn, sc.Header.cmdBuf[sc.Header.hasCmdReadLen:util.CMD_TYPE_LEN])
		if e != nil && e == io.ErrUnexpectedEOF {
			sc.Header.hasCmdReadLen += int32(retN)
			continue
		}
		if e != nil {
			log.Printf("read cmd type value from network fail, err: %v\n", e)
			return e
		}
		sc.Header.hasCmdReadLen += int32(retN)
	}
	sc.Header.hasCmdReadLen = 0
	cmdType := binary.BigEndian.Uint16(sc.Header.cmdBuf[:])
	sc.Header.cmdValue = cmdType
	//

	needRecvPayLoadLen := sc.Header.pkgLen - uint32(util.HEAD_LEN_MAX+util.CMD_TYPE_LEN)
	if needRecvPayLoadLen <= 0 {
		return nil
	}

	payloadBuf := make([]byte, needRecvPayLoadLen)
	sc.PayLoad.payLoadBuf = payloadBuf

	for {
		if uint32(sc.PayLoad.receivedPkgLen) >= needRecvPayLoadLen {
			break
		}
		//
		retN, e := io.ReadFull(sc.UnderlyingConn, sc.PayLoad.payLoadBuf[int(sc.PayLoad.receivedPkgLen):int(needRecvPayLoadLen)])
		if e != nil && e == io.ErrUnexpectedEOF {
			sc.PayLoad.receivedPkgLen += int32(retN)
			continue
		}
		if e != nil {
			log.Println("recv payload from network fail, e: ", e)
			return e
		}
		sc.PayLoad.receivedPkgLen += int32(retN)
	}
	sc.PayLoad.receivedPkgLen = 0
	return nil
}

func (s *ServerNode) GetLogicHandle(cmd uint16) IBusiLogic {
	ret, ok := s.LogicHandles[cmd]
	if !ok {
		return nil
	}
	return ret
}

func (s *ServerNode) DoBusiness(sc *ServerConn) error {
	e := s.RecvMsg(sc)
	if e != nil {
		return e
	}

	logicHandle := s.GetLogicHandle(sc.Header.cmdValue)
	if logicHandle == nil {
		return fmt.Errorf("not get logic handle")
	}

	data := logicHandle.LogicProc(sc.PayLoad.payLoadBuf)
	if data == nil {
		return fmt.Errorf("proc logic ret is nil")
	}
	cmdTypeRsp := logicHandle.GetResponseCmd()
	return s.SendMsg(sc, data, cmdTypeRsp)
}
func (s *ServerNode) SendMsg(sc *ServerConn, data []byte, rspType uint16) error {
	log.Printf("buf data len: %d\n, value: %s\n", len(data), string(data))
	sendBuf := make([]byte, util.HEAD_LEN_MAX+util.CMD_TYPE_LEN+int32(len(data)))
	binary.BigEndian.PutUint32(sendBuf[0:util.HEAD_LEN_MAX], uint32(len(sendBuf)))
	binary.BigEndian.PutUint16(sendBuf[util.HEAD_LEN_MAX:util.CMD_TYPE_LEN+util.HEAD_LEN_MAX], rspType)
	copy(sendBuf[util.HEAD_LEN_MAX+util.CMD_TYPE_LEN:], data)
	retW, e := sc.UnderlyingConn.Write(sendBuf)
	if e != nil {
		log.Printf("send msg to peer fail, e: %v\n", e)
		return e
	} else {
		log.Printf("send msg to peer succ, send len: %v\n", retW)
	}
	log.Printf("send data len: %d\n", len(sendBuf))
	return nil
}

func (s *ServerNode) ReadWriteOnConn(svrAddr string, newConn net.Conn) error {
	newConn.SetDeadline(time.Now().Add(10000 * time.Second)) //TEST....
	//
	sc := s.NewServerConnected(newConn)
	go func() {
		for {
			e := s.DoBusiness(sc)
			if e != nil {
				break
			}
		}
	}()
	return nil
}

func NewServer() *ServerNode {
	flag.Parse()

	ret := &ServerNode{
		config: ServerConfig{
			Port:    *port,
			Ip:      "",
			NetWork: "tcp",
		},
		LogicHandles: make(map[uint16]IBusiLogic),
	}
	//
	ret.LogicHandles[util.LOGIN_CMD] = NewLoginHandle()
	return ret
}

type ServerConn struct {
	// read buf, write buf
	// read index, write index.
	// underlying connect
	UnderlyingConn net.Conn
	bufferPool     *util.BufferPool
	Header         *PackageHead
	PayLoad        *PackagePayLoad
	//

}

type PackagePayLoad struct {
	r              io.Reader
	payLoadBuf     []byte
	receivedPkgLen int32
}

func NewPkgPayLoad(c net.Conn) *PackagePayLoad {
	return &PackagePayLoad{
		r:              c,
		receivedPkgLen: 0,
	}
}

type PackageHead struct {
	writer     *util.BufWriter
	r          io.Reader
	getReadBuf func(size uint32) []byte
	//readBuf     []byte // cache for default getReadBuf
	headBuf        [util.HEAD_LEN_MAX]byte
	hasHeadReadLen int32
	pkgLen         uint32 //parsed value
	maxReadSize    uint32
	//
	cmdBuf        [util.CMD_TYPE_LEN]byte
	hasCmdReadLen int32
	cmdValue      uint16 // parsed value
}

func NewPackageHead(c net.Conn, wbufSz, rbufSz int, sharedWBuf bool) *PackageHead {
	var r io.Reader = c
	if rbufSz > 0 {
		r = bufio.NewReaderSize(r, rbufSz)
	}

	var pool *sync.Pool
	if sharedWBuf {
		pool = util.GetWriteBufferPool(wbufSz)
	}

	w := util.NewBufWriter(c, wbufSz, pool)

	h := &PackageHead{
		writer: w,
		r:      r,
	}

	//h.getReadBuf = func(size uint32) []byte {
	//	if cap(h.readBuf) >= int(size) {
	//		return h.readBuf[:size]
	//	}
	//	h.readBuf = make([]byte, size)
	//	return h.readBuf
	//}
	return h
}
func main() {
	s := NewServer()
	if e := s.Listen(); e != nil {
		return
	}

	if e := s.ServerWait(); e != nil {
		return
	}
}
