// Package sbv reads and writes SBV, the caption format of YouTube Studio.
//
// Each cue is a block of a timing line such as "0:00:01.500,0:00:04.000" and text lines. An empty line ends the block.
// SBV has no markup, so "<i>" in an SBV file is text.
package sbv

import "regexp"

// ReadOptions holds the settings of the SBV reader. It has no fields yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the SBV writer. It has no fields yet.
type WriteOptions struct{}

var signatureRegexp = regexp.MustCompile(`(?m)\A\d+:\d{2}:\d{2}\.\d{3},\d+:\d{2}:\d{2}\.\d{3}[ \t]*$`)

// Detect reports whether content has the SBV signature of stbox.Detect: a first line that is an SBV timing line
// plus optional spaces or tabs. The content must have no leading BOM or white space and LF line endings, as
// stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
