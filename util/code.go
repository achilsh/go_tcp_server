package util

import "encoding/json"

const (
	CODES_JSON = 1
)

type ICodecs interface {
	Decode(inPayLoad []byte, outBusi any) error
	Encode(inPayLoad any) ([]byte, error)
}
type JsonParser struct {
}

func GetCodecs(t int) ICodecs {
	switch t {
	case CODES_JSON:
		return &JsonParser{}
	}
	return nil
}

func (j *JsonParser) Decode(inPayLoad []byte, outBusi any) error {
	e := json.Unmarshal(inPayLoad, outBusi)
	return e
}
func (j *JsonParser) Encode(inPayLoad any) ([]byte, error) {
	ret, e := json.Marshal(inPayLoad)
	return ret, e
}
