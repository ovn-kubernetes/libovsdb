//go:build !sonic_json && !go_json

package json

import "encoding/json"

var (
	Marshal   = json.Marshal
	Unmarshal = json.Unmarshal
)

type RawMessage = json.RawMessage
