// Package stream reads and writes SubRip and WebVTT one cue block at a time, in constant memory.
//
// Only SubRip and WebVTT stream, because their cue blocks do not depend on each other. A reader gives the
// cues, the warnings and the WebVTT header of the full reader, in file order. A writer gives the bytes of the
// full writer. The package reads UTF-8. Wrap the input with golang.org/x/text/transform for other encodings.
package stream

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// ReadOptions holds the settings of a stream reader.
type ReadOptions struct {
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	Lenient bool
}

func (o ReadOptions) common() model.CommonReadOptions {
	return model.CommonReadOptions{Lenient: o.Lenient}
}

func validateCommon(common model.CommonWriteOptions) error {
	if common.LineEnding > model.CRLF {
		return &model.InvalidArgumentError{Option: "CommonWriteOptions.LineEnding", Msg: "The line ending " + strconv.Itoa(int(common.LineEnding)) + " is unknown."}
	}
	if common.BOM > model.BOMRemove {
		return &model.InvalidArgumentError{Option: "CommonWriteOptions.BOM", Msg: "The BOM rule " + strconv.Itoa(int(common.BOM)) + " is unknown."}
	}
	return nil
}
