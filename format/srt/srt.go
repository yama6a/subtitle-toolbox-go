// Package srt reads and writes SubRip, the numbered blocks of a timing line and text lines that most players read.
//
// The reader reads the core markup tags, the ASS override tags that converters leave in the text, the
// coordinates after the end time and the WebVTT cue settings that yt-dlp writes there. The writer writes the
// tags that SubRip players show, the coordinates and an {\anN} tag for an alignment other than bottom center.
package srt

import "regexp"

// formatDataKey is the cue format data key of the SubRip coordinates.
const formatDataKey = "srt"

// ReadOptions holds the settings of the SubRip reader. It has no fields yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the SubRip writer. It has no fields yet.
type WriteOptions struct{}

// signatureRegexp matches a cue number and a timing line, or the same after up to 5 lines of other text, or a
// timing line alone. The last 2 forms need 2 hour digits and a comma, so headerless WebVTT and SBV do not match.
var signatureRegexp = regexp.MustCompile(`\A\d+[ \t]*\n[ \t]*\d+:\d{2}:\d{2}(?:[,.]\d+)?[ \t]*-->` +
	`|\A(?:[^\n]*\n){0,5}?[ \t]*\d+[ \t]*\n[ \t]*\d{2,}:\d{2}:\d{2},\d+[ \t]*-->` +
	`|\A\d{2,}:\d{2}:\d{2},\d+[ \t]*-->`)

// Detect reports whether content has the SubRip signature of stbox.Detect. The content must have no leading
// BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
