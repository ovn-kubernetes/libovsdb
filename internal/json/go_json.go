//go:build go_json

package json

import json "github.com/goccy/go-json"

var (
	Marshal   = json.Marshal
	Unmarshal = json.Unmarshal
)

type RawMessage = json.RawMessage
