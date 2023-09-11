//go:build !jsoniter && !go_json

package json

import "encoding/json"

var (
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
)

type RawMessage = json.RawMessage
