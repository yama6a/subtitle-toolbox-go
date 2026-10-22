// Package writing applies the line ending and the BOM rule that every text writer shares.
package writing

import (
	"bytes"
	"fmt"
	"io"

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
