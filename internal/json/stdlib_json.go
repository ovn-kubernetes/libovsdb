//go:build race || 386 || arm || mips || mipsle || wasm

package json

import (
	"encoding/json"
)

var (
	Marshal   = json.Marshal
	Unmarshal = json.Unmarshal
)

type RawMessage = json.RawMessage
