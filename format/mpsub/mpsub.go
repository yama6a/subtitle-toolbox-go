// Package mpsub reads and writes MPSub, the subtitle format of MPlayer.
//
// A header of KEY=value lines sets the time unit with FORMAT=TIME for seconds or FORMAT=<n> for frames at n
// frames per second. Each cue has a timing line with 2 numbers, then its text lines until an empty line. The
// first number is the wait after the end of the previous cue. The second is the duration. The extension is .mpsub.
package mpsub

import (
	"bytes"
	"math"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.MPSub)

// ReadOptions holds the settings of the MPSub reader. MPSub has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the MPSub writer.
type WriteOptions struct {
	// FrameRate writes the times as frames at this whole number of frames per second. Zero writes seconds.
	// The reader does not store the FORMAT line, so a file with frames written without FrameRate has seconds.
	FrameRate float64
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that is negative, not whole, NaN or infinite.
func (o WriteOptions) Validate() error {
	f := o.FrameRate
	if f == 0 || f > 0 && !math.IsInf(f, 1) && f == math.Floor(f) {
		return nil
	}
	return &model.InvalidArgumentError{
		Option: "WriteOptions.MPSub.FrameRate",
		Msg:    "The MPSub frame rate must be a positive integer, got " + timecode.Number(f) + ".",
	}
}

// Detect reports whether content starts with 1 or more ASCII capitals and =, and has a line that starts with
// FORMAT=, the first line included. A file without a FORMAT= line is not detected, but the reader reads it.
// MPlayer files often have the extension .sub, so content detection decides before FormatFromPath. The content
// must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	n := 0
	for n < len(content) && 'A' <= content[n] && content[n] <= 'Z' {
		n++
	}
	if n == 0 || n == len(content) || content[n] != '=' {
		return false
	}
	return bytes.HasPrefix(content, []byte("FORMAT=")) || bytes.Contains(content, []byte("\nFORMAT="))
}
