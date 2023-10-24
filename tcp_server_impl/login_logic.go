package main

import (
	"errors"
	"fmt"
	"log"
	"net_tcp_server_demo/proto"
	"net_tcp_server_demo/util"
)

type IBusiLogic interface {
	LogicProc(in []byte) []byte
	GetResponseCmd() uint16
	GetReqCmd() uint16
	//
	checkReqParams([]byte) error
	doInnerLogic() error
}

type LoginLogic struct {
	Req *proto.LoginReq
	Rsp *proto.LoginRsp
}

func NewLoginHandle() IBusiLogic {
	ret := &LoginLogic{
		Req: new(proto.LoginReq),
		Rsp: new(proto.LoginRsp),
	}

	ret.Rsp.Code = 1000
	ret.Rsp.Message = "succc"
	ret.Rsp.Data = "ok"
	return ret
}

func (l *LoginLogic) GetReqCmd() uint16 {
	return util.LOGIN_CMD
}
func (l *LoginLogic) GetResponseCmd() uint16 {
	return util.LOGIN_CMD_RSP
}
func (l *LoginLogic) checkReqParams(data []byte) error {
	if len(data) <= 0 {
		l.Rsp.Code = 2000
		l.Rsp.Message = "input data is empty"
		return errors.New("input param is invalid")
	}
	return nil
}
func (l *LoginLogic) doInnerLogic() error {
	//TODO:
	return nil
}

func (l *LoginLogic) LogicProc(in []byte) []byte {
	codeHandle := util.GetCodecs(util.CODEC_JSON)
	for {
		if l.checkReqParams(in) != nil { //check input parameters.
			break
		}

		e := codeHandle.Decode(in, l.Req) // decode request message.
		if e != nil {
			l.Rsp.Code = 2001
			l.Rsp.Message = fmt.Sprintf("decode fail, e: %v", e)
			log.Printf("decode req msg fail, e: %v\n, msg: %v\n", e, string(in))
			break
		}

		l.doInnerLogic() // do business logic.

		break
	}

	rspData, e := codeHandle.Encode(l.Rsp)
	if e != nil {
		log.Printf("encode response to json fail, e: %v\n", e)
		return nil
	}
	//
	return rspData
}
