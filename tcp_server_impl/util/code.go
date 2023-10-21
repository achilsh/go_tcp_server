package util

var registeredCodecs = make(map[int]ICodecs)

const (
	CODEC_JSON = 1
)

type ICodecs interface {
	Decode(inPayLoad []byte, outBusi any) error
	Encode(inPayLoad any) ([]byte, error)
	GetType() int
}

func RegisterCodec(codec ICodecs) {
	if codec == nil {
		return
	}
	registeredCodecs[codec.GetType()] = codec
}

func GetCodecs(t int) ICodecs {
	codec, ok := registeredCodecs[t]
	if !ok {
		return nil
	}
	return codec
}
