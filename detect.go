package stbox

import (
	"bytes"
	"encoding/json"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
)

// ContainerHeadSize is the number of bytes at the start of a file that Detect needs to find a container: 2*192+5.
const ContainerHeadSize = 389

// Detection is the result of Detect: a format or a container.
type Detection struct {
	// Format is the subtitle format, "" for a container.
	Format Format
	// Container is the video container, "" for a subtitle file.
	Container Container
}

// Detect returns the format or container of content, or false when nothing matches.
// Pass UTF-8 text, or the bytes of a binary format. Pass at least ContainerHeadSize bytes to find a container.
//
// A container wins over every format. Then the formats with a raw signature, such as PGS, check the bytes as they are.
// Detect then removes the leading UTF-8 BOMs and white space. Content that is a JSON object goes to the JSON checks only.
// Other content gets LF line endings, and the signatures of the formats decide in a fixed order.
// docs/detection.md lists the order.
func Detect(content []byte) (Detection, bool) {
	if c, ok := containerOf(content); ok {
		return Detection{Container: c}, true
	}
	f, ok := detectFormat(content)
	return Detection{Format: f}, ok
}

// detectFormat returns the subtitle format of content. It never returns a container.
func detectFormat(content []byte) (Format, bool) {
	for _, s := range rawSignatures {
		if s.match(content) {
			return s.format, true
		}
	}
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
