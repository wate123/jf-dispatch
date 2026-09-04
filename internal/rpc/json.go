package rpc

import "encoding/json"

type JSONCodec struct{}

func (JSONCodec) Name() string                    { return "json" }
func (JSONCodec) Marshal(v any) ([]byte, error)   { return json.Marshal(v) }
func (JSONCodec) Unmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
