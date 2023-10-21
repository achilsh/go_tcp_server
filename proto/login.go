package proto

type LoginReq struct {
	Name  string  `json:"name"`
	Age   int32   `json:"age"`
	Score float32 `json:"score"`
}
type LoginRsp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}
