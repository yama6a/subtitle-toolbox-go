// Package stream reads and writes SubRip and WebVTT one cue block at a time, in constant memory.
//
// Only SubRip and WebVTT stream, because their cue blocks do not depend on each other. A reader gives the
// cues, the warnings and the WebVTT header of the full reader, in file order. A writer gives the bytes of the
// full writer. The package reads UTF-8. Wrap the input with golang.org/x/text/transform for other encodings.
package stream

import (
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// ReadOptions holds the settings of a stream reader.
type ReadOptions struct {
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	Lenient bool
}

func (o ReadOptions) common() model.CommonReadOptions {
	return model.CommonReadOptions{Lenient: o.Lenient}
}
