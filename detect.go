package stbox

import (
	"bytes"
	"encoding/json"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

// Detect returns the format of content, or false when no auto-detected format matches.
// Pass UTF-8 text, or the bytes of a binary format.
//
// Detect removes the leading UTF-8 BOMs and white space first. Content that is then a JSON object goes to the
// JSON checks only. Other content gets LF line endings, and the signatures of the formats decide in a fixed
// order. docs/detection.md lists the order.
func Detect(content []byte) (Format, bool) {
	content = bytes.TrimLeft(textenc.TrimUTF8BOMs(content), markupcore.PHPTrimSet)
	if len(content) > 0 && content[0] == '{' {
		var keys map[string]json.RawMessage
		if json.Unmarshal(content, &keys) == nil {
			for _, c := range jsonChecks {
				if c.match(keys) {
					return c.format, true
				}
			}
			return "", false
		}
	}
	content = lfLineEnds(content)
	for _, s := range signatures {
		if s.match(content) {
			return s.format, true
		}
	}
	return "", false
}

// lfLineEnds turns each run of CRs followed by LF into one LF, and each other CR into LF.
func lfLineEnds(content []byte) []byte {
	if bytes.IndexByte(content, '\r') < 0 {
		return content
	}
	out := make([]byte, 0, len(content))
	for i := 0; i < len(content); i++ {
		if content[i] != '\r' {
			out = append(out, content[i])
			continue
		}
		j := i + 1
		for j < len(content) && content[j] == '\r' {
			j++
		}
		if j < len(content) && content[j] == '\n' {
			out = append(out, '\n')
		} else {
			out = append(out, bytes.Repeat([]byte{'\n'}, j-i)...)
			j--
		}
		i = j
	}
	return out
}

// isBlank reports whether content holds nothing but UTF-8 BOMs and the bytes of PHP trim.
func isBlank(content []byte) bool {
	return len(bytes.Trim(textenc.TrimUTF8BOMs(content), markupcore.PHPTrimSet)) == 0
}
