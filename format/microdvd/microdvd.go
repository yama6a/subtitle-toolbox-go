// Package microdvd reads and writes MicroDVD, the {start}{end}text lines that count time in video frames.
//
// The reader and the writer need the frame rate of the video, from the options or from a {1}{1}<fps> first line.
// The control codes {y:...} and {c:$BBGGRR} become core markup and back. The other codes stay in the microdvd
// format data, so a round trip keeps them.
package microdvd

import (
	"math"
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.MicroDVD)

// ReadOptions holds the settings of the MicroDVD reader.
type ReadOptions struct {
	// FrameRate is the frame rate of the video in frames per second. It wins over a {1}{1}<fps> line. Zero takes that line.
	FrameRate float64
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that is negative, NaN or infinite.
func (o ReadOptions) Validate() error {
	if o.FrameRate == 0 {
		return nil
	}
	return checkFrameRate(o.FrameRate, "ReadOptions.MicroDVD.FrameRate", "The frame rate")
}

// WriteOptions holds the settings of the MicroDVD writer.
type WriteOptions struct {
	// FrameRate is the output frame rate in frames per second. Zero takes the frame rate that Read stored.
	FrameRate float64
	// WriteFrameRateLine writes {1}{1}<fps> as the first line. A subtitle read from a file with that line gets it anyway.
	WriteFrameRateLine bool
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that is negative, NaN or infinite.
func (o WriteOptions) Validate() error {
	if o.FrameRate == 0 {
		return nil
	}
	return checkFrameRate(o.FrameRate, "WriteOptions.MicroDVD.FrameRate", "The MicroDVD frame rate")
}

// checkFrameRate returns an *InvalidArgumentError unless fps is finite and above 0. subject starts the message.
func checkFrameRate(fps float64, option, subject string) error {
	if fps > 0 && !math.IsInf(fps, 1) {
		return nil
	}
	return &model.InvalidArgumentError{Option: option, Msg: frameRateMessage(subject, fps)}
}

func frameRateMessage(subject string, fps float64) string {
	return subject + " must be greater than 0, got " + timecode.Number(fps) + "."
}

var signatureRegexp = regexp.MustCompile(`\A\{\d+\}\{\d*\}`)

// Detect reports whether content starts with {digits}{digits or nothing}. The content must have no leading BOM or
// white space, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
