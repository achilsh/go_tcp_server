package util

// we define protocal format:
// len(4byte) + command(2byte) + playload

const (
	HEAD_LEN_MAX int32  = 4
	CMD_TYPE_LEN int32  = 2
	PKG_LEN_MAX  uint32 = 1024 * 10
)

// ///
const (
	LOGIN_CMD     uint16 = 1
	LOGIN_CMD_RSP uint16 = 2
)
