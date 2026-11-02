// Package wire holds JSON helpers shared by the REST and WebSocket packages.
package wire

import "encoding/json"

// Typed encodes v as a JSON object with "type" first. kind must not need escaping.
func Typed(kind string, v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	head := `{"type":"` + kind + `"`
	if string(raw) == "{}" {
		return []byte(head + "}"), nil
	}
	return append([]byte(head+","), raw[1:]...), nil
}
