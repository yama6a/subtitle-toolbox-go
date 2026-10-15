package stboxjson

import (
	"bytes"
	"encoding/json"
)

// DetectJSON reports whether the top-level keys of a JSON object match the library JSON rule: version is a JSON
// number and cues is a JSON array.
func DetectJSON(keys map[string]json.RawMessage) bool {
	version := bytes.TrimLeft(keys["version"], " \t\r\n")
	cues := bytes.TrimLeft(keys["cues"], " \t\r\n")
	return len(version) > 0 && (version[0] == '-' || version[0] >= '0' && version[0] <= '9') &&
		len(cues) > 0 && cues[0] == '['
}
