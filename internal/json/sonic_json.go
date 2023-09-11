package json

import (
	"github.com/bytedance/sonic"
)

var (
	Marshal   = sonic.Marshal
	Unmarshal = sonic.Unmarshal
)

type RawMessage = sonic.NoCopyRawMessage
