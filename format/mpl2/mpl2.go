// Package mpl2 reads and writes MPL2, the one-line-per-cue format of Polish subtitle downloads.
//
// A line is [start][end]text with times in tenths of a second. A | ends a text line, and a / at the start of a
// line makes it italic. The extension is .txt.
package mpl2

import "regexp"

// ReadOptions holds the settings of the MPL2 reader. MPL2 has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the MPL2 writer. MPL2 has none yet.
type WriteOptions struct{}

var signatureRegexp = regexp.MustCompile(`\A\[\d+\]\[\d+\]`)

// Detect reports whether content starts with [digits][digits]. The content must have no leading BOM or white
// space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
