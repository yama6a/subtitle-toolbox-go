// Package writing holds the rules that text writers share: the line ending, the BOM rule and the cue text rules.
package writing

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Finish converts the LF output of a writer to the line ending and then applies the BOM rule.
// formatBOM is the BOM default of the format. Finish never changes body.
func Finish(body []byte, common model.CommonWriteOptions, formatBOM bool) []byte {
	out := LineEnding(body, common)
	switch common.BOM {
	case model.BOMAdd:
		return textenc.AddUTF8BOM(out)
	case model.BOMRemove:
		return textenc.TrimUTF8BOMs(out)
	case model.BOMDefault:
		if formatBOM {
			return textenc.AddUTF8BOM(out)
		}
	}
	return out
}

// LineEnding converts the LF output of a writer to the line ending. It never changes body.
// A writer that writes in parts converts each part with it and calls Finish for the first part only.
func LineEnding(body []byte, common model.CommonWriteOptions) []byte {
	if common.LineEnding == model.CRLF {
		return toCRLF(body)
	}
	return body
}

// Write finishes body and writes it to w in one call. The output on w is incomplete after an error.
func Write(w io.Writer, f model.Format, body []byte, common model.CommonWriteOptions, formatBOM bool) error {
	if _, err := w.Write(Finish(body, common, formatBOM)); err != nil {
		return fmt.Errorf("write %s: %w", f, err)
	}
	return nil
}

// WordJoiner is U+2060 WORD JOINER. A writer puts it into cue text that its reader would read as structure, such as a
// text line in the shape of a timing line. It has no width, so the text looks the same.
const WordJoiner = "\u2060"

// PipeText joins lines with |, the line break of MicroDVD, MPL2, TMPlayer and SubViewer 1. These formats have no
// escape for |, so a | in a line becomes U+00A6 BROKEN BAR.
func PipeText(lines []string) string {
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = strings.ReplaceAll(line, "|", "\u00a6")
	}
	return strings.Join(parts, "|")
}

func toCRLF(body []byte) []byte {
	out := make([]byte, 0, len(body)+bytes.Count(body, []byte("\n")))
	for i, b := range body {
		if b == '\n' && (i == 0 || body[i-1] != '\r') {
			out = append(out, '\r')
		}
		out = append(out, b)
	}
	return out
}
