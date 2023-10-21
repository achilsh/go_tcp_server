package main

import (
	"fmt"
	"log"
	"net_tcp_server_demo/proto"
	"net_tcp_server_demo/util"
)

type IBusiLogic interface {
	LogicProc(in []byte) []byte
	GetResponseCmd() uint16
	GetReqCmd() uint16
}

type LoginLogic struct {
	Req *proto.LoginReq
	Rsp *proto.LoginRsp
}

func NewLoginHandle() IBusiLogic {
	return &LoginLogic{}
}

func (l *LoginLogic) GetReqCmd() uint16 {
	return util.LOGIN_CMD
}
func (l *LoginLogic) GetResponseCmd() uint16 {
	return util.LOGIN_CMD_RSP
}
func (l *LoginLogic) LogicProc(in []byte) []byte {
	l.Rsp = new(proto.LoginRsp)
	codeHandle := util.GetCodecs(util.CODEC_JSON)

	for {
		if len(in) <= 0 {
			l.Rsp.Code = 2000
			l.Rsp.Message = "input data is empty"
			break
		}

		l.Req = new(proto.LoginReq)
		e := codeHandle.Decode(in, l.Req)
		if e != nil {
			l.Rsp.Code = 2001
			l.Rsp.Message = fmt.Sprintf("decode fail, e: %v", e)
			log.Printf("decode req msg fail, e: %v\n, msg: %v\n", e, string(in))
			break
		}

		log.Printf("req, Name: %s, Age: %d, score: %f", l.Req.Name, l.Req.Age, l.Req.Score)

		l.Rsp.Code = 1000
		l.Rsp.Message = "succc"
		l.Rsp.Data = "ok"

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
