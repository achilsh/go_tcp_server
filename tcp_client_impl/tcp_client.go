package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"tcp_server/proto"
	"tcp_server/util"
	"time"
)

var (
	port = flag.Int("port", 8080, "the server port")
	ip   = flag.String("ip", "", "the server ip")
)

func ClientCallOne(ipStr string) {
	c, e := net.Dial("tcp", fmt.Sprintf("%v:%d", ipStr, *port))
	if e != nil {
		log.Printf("connect server %v:%d fail, e: \n", ipStr, *port, e)
		return
	}

	for i := 0; i < 3; i++ {
		req := &proto.LoginReq{
			Name:  fmt.Sprintf("%d + name", i),
			Age:   int32(i),
			Score: float32(1000.0 + i),
		}

		codeHandle := util.GetCodecs(util.CODEC_JSON)
		sendData, e := codeHandle.Encode(req)
		if e != nil {
			log.Printf("encode json to str fail, e: %v\n", e)
			return
		}
		log.Printf("%#v\n, len: %d\n", string(sendData), len(sendData))

		sendBuf := make([]byte, util.HEAD_LEN_MAX+util.CMD_TYPE_LEN+int32(len(sendData)))
		binary.BigEndian.PutUint32(sendBuf[0:util.HEAD_LEN_MAX], uint32(len(sendBuf)))
		binary.BigEndian.PutUint16(sendBuf[util.HEAD_LEN_MAX:util.HEAD_LEN_MAX+util.CMD_TYPE_LEN], util.LOGIN_CMD)

		copy(sendBuf[util.HEAD_LEN_MAX+util.CMD_TYPE_LEN:], sendData)

		retW, e := c.Write(sendBuf)
		if e != nil {
			log.Printf("send data to peer fail, e: %v\n", e)
			return
		}
		log.Printf("send pkg: %#v\n, write ret: %d\n", string(sendBuf[:]), retW) //util.HEAD_LEN_MAX+util.CMD_TYPE_LEN
		//

		recvLenBuf := make([]byte, util.HEAD_LEN_MAX)
		if _, e := io.ReadFull(c, recvLenBuf); e != nil {
			log.Printf("recv response len fail, e: %v\n", e)
			return
		}
		pkgLen := binary.BigEndian.Uint32(recvLenBuf)

		//
		cmdTypeBuf := make([]byte, util.CMD_TYPE_LEN)
		if _, e := io.ReadFull(c, cmdTypeBuf); e != nil {
			log.Printf("read response type fail, e: %v\n", e)
			return
		}
		cmdType := binary.BigEndian.Uint16(cmdTypeBuf)
		log.Printf("cmdType: %v\n", cmdType)

		//
		recvPkgBuf := make([]byte, pkgLen-uint32(util.HEAD_LEN_MAX+util.CMD_TYPE_LEN))
		if _, e := io.ReadFull(c, recvPkgBuf[:]); e != nil {
			log.Printf("recv payload response fail, e: %v\n", e)
			return
		}

		rspStruct := &proto.LoginRsp{}
		codeGen := util.GetCodecs(util.CODEC_JSON)
		if e := codeGen.Decode(recvPkgBuf, rspStruct); e != nil {
			log.Printf("decode bin to json fail, e: %v\n", e)
			return
		}
		log.Printf("recv msg: %#v\n", rspStruct)

	}
}
func main() {
	ipStr := ""
	if ip != nil {
		ipStr = *ip
	}
	//
	ClientCallOne(ipStr)
	time.Sleep(1 * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ClientCallOne(ipStr)
		}()
	}
	wg.Wait()

}
