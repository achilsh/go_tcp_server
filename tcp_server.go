package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net_tcp_server_demo/opts"
	"net_tcp_server_demo/util"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	opts   opts.ServerOptions
	config ServerConfig

	mu           sync.Mutex // guards following
	lis          map[net.Listener]bool
	l            net.Listener
	conns        map[string]map[*ServerConn]bool
	LogicHandles map[uint16]IBusiLogic
	//
	quit *util.Event
	done *util.Event

	serveWG sync.WaitGroup
	cv      *sync.Cond
}

func (s *ServerNode) AddConnection(svrAddr string, newConn *ServerConn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	//
	if s.conns == nil {
		newConn.UnderlyingConn.Close()
		return errors.New("addConn called when server has already been stopped")
	}
	if s.conns[svrAddr] == nil {
		s.conns[svrAddr] = make(map[*ServerConn]bool)
	}
	s.conns[svrAddr][newConn] = true
	return nil
}
func (s *ServerNode) Stop() {
	//s.quit.Fire()
	//
	//defer func() {
	//	s.serveWG.Wait()
	//	s.done.Fire()
	//}()
	//
	//s.mu.Lock()
	//toCloseListener := s.l
	//s.l = nil
	//conns := s.conns
	//s.conns = nil
	//
	//s.cv.Broadcast()
	//s.mu.Unlock()
	//
	//toCloseListener.Close()
	//for _, cs := range conns {
	//	for item := range cs {
	//		item.UnderlyingConn.Close()
	//	}
	//}
}

func (s *ServerNode) GracefulStop() {
	log.Println("grace stop......")
	s.quit.Fire()
	defer s.done.Fire()

	s.mu.Lock()
	if s.conns == nil {
		s.mu.Unlock()
		return
	}
	s.l.Close()
	s.l = nil
	s.mu.Unlock()

	s.serveWG.Wait()

	s.mu.Lock()
	for len(s.conns) != 0 {
		log.Printf("wait conn to close, not close conn nums: %v\n", len(s.conns))
		s.cv.Wait()
	}
	s.conns = nil
	s.mu.Unlock()
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
	s.mu.Lock()
	fmt.Println("serving....")
	if s.lis == nil {
		s.mu.Unlock()
		s.l.Close()
		return errors.New("tcp: the server has been stopped")
	}

	s.serveWG.Add(1)
	defer func() {
		log.Println("entry to end.")
		s.serveWG.Done()

		if s.quit.HasFired() {
			log.Println("entry has fired.")

			for _, tmpC := range s.conns {
				for kc, kv := range tmpC {
					if kv == false {
						continue
					}
					if kc == nil {
						continue
					}
					kc.UnderlyingConn.SetDeadline(time.Now().Add(time.Millisecond * 2))
				}
			}
			<-s.done.Done()
		}
		log.Println("at end of wait.")
	}()

	s.lis[s.l] = true
	defer func() {
		s.mu.Lock()
		if s.lis != nil && s.lis[s.l] {
			s.l.Close()
			delete(s.lis, s.l)
		}
		s.mu.Unlock()
	}()
	s.mu.Unlock()

	var tempDelay time.Duration = 0
	for {
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
				case <-s.quit.Done():
					tmr.Stop()
					return nil
				}
				continue
			}

			s.mu.Lock()
			log.Printf("do serving; accept happen err: %v\n", e)
			s.mu.Unlock()

			if s.quit.HasFired() {
				log.Printf("has fired on quit.")
				return nil
			}
			return e
		}

		tempDelay = 0
		s.serveWG.Add(1)
		go func() {
			s.ReadWriteOnConn(s.l.Addr().String(), rawConn)
			s.serveWG.Done()
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
		if e != nil && e == io.EOF {
			log.Printf("peer close connect.")
			return e
		}
		if e != nil {
			log.Printf("read head from network fail, err: %v\n", e)
			return e
		}
		sc.Header.hasHeadReadLen += int32(retN)
	}
	sc.Header.ResetHeadRecvLen()

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
	cmdType := binary.BigEndian.Uint16(sc.Header.cmdBuf[:])
	sc.Header.cmdValue = cmdType
	sc.Header.ResetHeadCmdLen()

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
	sc.PayLoad.ResetRecvPayLoadLen()

	return nil
}

func (s *ServerNode) GetLogicHandle(cmd uint16) IBusiLogic {
	ret, ok := s.LogicHandles[cmd]
	if !ok {
		return nil
	}
	return ret
}
func (s *ServerNode) RegisterLogicHandle(busi IBusiLogic) {
	s.LogicHandles[busi.GetReqCmd()] = busi
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

	data := logicHandle.LogicProc(sc.PayLoad.payLoadBuf) // different logic need to implement and register to ServerNode.
	if data == nil {
		return fmt.Errorf("proc logic ret is nil")
	}

	return s.SendMsg(sc, data, logicHandle.GetResponseCmd())
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
func (s *ServerNode) RemoveConn(svrAddr string, conn *ServerConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conns := s.conns[svrAddr]
	if conns == nil {
		return
	}

	delete(conns, conn)
	if len(conns) == 0 {
		delete(s.conns, svrAddr)
	}
	conn.UnderlyingConn.Close()
	s.cv.Broadcast()
}

func (s *ServerNode) ReadWriteOnConn(svrAddr string, newConn net.Conn) error {
	if s.quit.HasFired() {
		newConn.Close()
		return nil
	}
	newConn.SetDeadline(time.Now().Add(s.opts.ConnectionTimeout)) //TEST....
	sc := s.NewServerConnected(newConn)
	if sc == nil {
		return nil
	}
	if e := s.AddConnection(svrAddr, sc); e != nil {
		return nil
	}

	func() {
		for {
			e := s.DoBusiness(sc)
			if e != nil {
				break
			}
		}
		s.RemoveConn(svrAddr, sc)
	}()
	return nil
}

const (
	defaultServerMaxReceiveMessageSize = 1024 * 1024 * 4
	defaultServerMaxSendMessageSize    = math.MaxInt32
	defaultWriteBufSize                = 32 * 1024
	defaultReadBufSize                 = 32 * 1024
)

var defaultServerOptions = opts.ServerOptions{
	MaxReceiveMessageSize: defaultServerMaxReceiveMessageSize,
	MaxSendMessageSize:    defaultServerMaxSendMessageSize,
	ConnectionTimeout:     120000 * time.Second, //120
	WriteBufferSize:       defaultWriteBufSize,
	ReadBufferSize:        defaultReadBufSize,
	RecvBufferPool:        util.NewSharedBufferPool(),
}

func NewTcpServer(optParams ...opts.IServerOption) *ServerNode {
	flag.Parse()
	optsPtr := &defaultServerOptions
	for _, o := range optParams {
		o.Apply(optsPtr)
	}
	ret := &ServerNode{
		lis:   make(map[net.Listener]bool),
		opts:  *optsPtr,
		quit:  util.NewEvent(),
		done:  util.NewEvent(),
		conns: make(map[string]map[*ServerConn]bool),
		config: ServerConfig{
			Port:    *port,
			Ip:      "",
			NetWork: "tcp",
		},
		LogicHandles: make(map[uint16]IBusiLogic),
	}
	ret.cv = sync.NewCond(&ret.mu)

	// different logic need to register to servernode like this.
	ret.RegisterLogicHandle(NewLoginHandle())
	return ret
}

type ServerConn struct {
	UnderlyingConn net.Conn
	bufferPool     *util.BufferPool
	Header         *PackageHead
	PayLoad        *PackagePayLoad
}

type PackagePayLoad struct {
	r              io.Reader
	payLoadBuf     []byte
	receivedPkgLen int32
}

func (p *PackagePayLoad) ResetRecvPayLoadLen() {
	p.receivedPkgLen = 0
}

func NewPkgPayLoad(c net.Conn) *PackagePayLoad {
	return &PackagePayLoad{
		r:              c,
		receivedPkgLen: 0,
	}
}

type PackageHead struct {
	//writer     *util.BufWriter
	//r          io.Reader
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

func (h *PackageHead) ResetHeadCmdLen() {
	h.hasCmdReadLen = 0
}
func (h *PackageHead) ResetHeadRecvLen() {
	h.hasHeadReadLen = 0
}

func NewPackageHead(c net.Conn, wbufSz, rbufSz int, sharedWBuf bool) *PackageHead {
	var r io.Reader = c
	if rbufSz > 0 {
		r = bufio.NewReaderSize(r, rbufSz)
	}

	//var pool *sync.Pool
	//if sharedWBuf {
	//	pool = util.GetWriteBufferPool(wbufSz)
	//}

	//w := util.NewBufWriter(c, wbufSz, pool)

	h := &PackageHead{
		//writer: w,
		//r:      r,
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
	s := NewTcpServer()
	if e := s.Listen(); e != nil {
		return
	}

	go func() {
		if e := s.ServerWait(); e != nil {
			return
		}
	}()

	chSig := make(chan os.Signal)
	signal.Notify(chSig, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Signal: ", <-chSig)
	s.GracefulStop()
	//s.Stop()
}
