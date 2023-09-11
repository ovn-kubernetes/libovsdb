//go:build go_json

package json

import json "github.com/goccy/go-json"

var (
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
)

type RawMessage = json.RawMessage
