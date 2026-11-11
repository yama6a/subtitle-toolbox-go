// Package microdvd reads and writes MicroDVD, the {start}{end}text lines that count time in video frames.
//
// The reader and the writer need the frame rate of the video, from the options or from a {1}{1}<fps> first line.
// The control codes {y:...} and {c:$BBGGRR} become core markup and back. The other codes stay in the microdvd
// format data, so a round trip keeps them.
package microdvd

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const formatDataKey = string(model.MicroDVD)

// ReadOptions holds the settings of the MicroDVD reader.
type ReadOptions struct {
	// FrameRate is the frame rate of the video. It wins over a {1}{1}<fps> line. The zero value takes that line.
	FrameRate model.FrameRate
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that fails model.FrameRate.Validate.
func (o ReadOptions) Validate() error {
	if o.FrameRate.IsZero() {
		return nil
	}
	return parsing.FrameRateOption(o.FrameRate.Validate(), "ReadOptions.MicroDVD.FrameRate", "The frame rate")
}

// WriteOptions holds the settings of the MicroDVD writer.
type WriteOptions struct {
	// FrameRate is the output frame rate. The zero value takes the frame rate that Read stored.
	FrameRate model.FrameRate
	// WriteFrameRateLine writes {1}{1}<fps> as the first line. A subtitle read from a file with that line gets it anyway.
	WriteFrameRateLine bool
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that fails model.FrameRate.Validate.
func (o WriteOptions) Validate() error {
	if o.FrameRate.IsZero() {
		return nil
	}
	return parsing.FrameRateOption(o.FrameRate.Validate(), "WriteOptions.MicroDVD.FrameRate", "The MicroDVD frame rate")
}

var signatureRegexp = regexp.MustCompile(`\A\{\d+\}\{\d*\}`)

// Detect reports whether content starts with {digits}{digits or nothing}. The content must have no leading BOM or
// white space, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
