package util

import "encoding/json"

func init() {
	RegisterCodec(&JsonParser{})
}

type JsonParser struct {
}

func (j *JsonParser) GetType() int {
	return CODEC_JSON
}

func (j *JsonParser) Decode(inPayLoad []byte, outBusi any) error {
	e := json.Unmarshal(inPayLoad, outBusi)
	return e
}
func (j *JsonParser) Encode(inPayLoad any) ([]byte, error) {
	ret, e := json.Marshal(inPayLoad)
	return ret, e
}
