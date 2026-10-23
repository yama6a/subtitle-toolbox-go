// Package lrc reads and writes LRC, the lyrics format of music players, with the word times of enhanced LRC.
//
// A line is [mm:ss.xx] and the text. Several time tags may start one line. ID tags such as [ti:Title] hold the
// metadata, [#:text] holds a comment, and [offset:N] shows every line N milliseconds earlier. The extension is .lrc.
package lrc

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// ReadOptions holds the settings of the LRC reader. LRC has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the LRC writer. LRC has none yet.
type WriteOptions struct{}

const formatDataKey = string(model.LRC)

// metadataTags maps the ID tags that hold metadata to their metadata keys, in the order the writer writes them.
//
//nolint:gochecknoglobals // lookup table of the metadata tags
var metadataTags = []struct{ tag, key string }{
	{"ti", model.MetadataTitle},
	{"ar", model.MetadataArtist},
	{"al", model.MetadataAlbum},
	{"au", model.MetadataAuthor},
}

// timestampPattern is the time of a time tag. Groups 1 and 2 hold hours and minutes, or group 3 holds the minutes.
// Group 4 holds the seconds and group 5 the fraction. Hours count only when 1 or 2 minute digits up to 59 and :
// follow, which PHP checks with the lookahead (?=[0-5]?\d:).
const timestampPattern = `(?:(\d{1,2}):([0-5]?\d)|(\d{1,3})):([0-5]\d)(?:\.(\d{1,3}))?`

const timeTagPattern = `\[[ \t]*` + timestampPattern + `[ \t]*\]`

var (
	detectStartRegexp = regexp.MustCompile(`\A(?:` + timeTagPattern + `|\[[A-Za-z#][A-Za-z0-9_]*:[^\]\n]*\])`)
	detectLineRegexp  = regexp.MustCompile(`(?m)^[ \t]*` + timeTagPattern)
)

// Detect reports whether content starts with an LRC time tag or ID tag and has a line that starts with a time tag.
//
// A time tag is [, optional spaces or tabs, optional hours of 1 or 2 digits and :, minutes of 1 to 3 digits, :,
// seconds 00 to 59, an optional . and 1 to 3 digits, optional spaces or tabs and ]. The hours count only when 1 or
// 2 minute digits up to 59 and : follow them. An ID tag is [, a letter or #, letters, digits or _, :, a value
// without ] or LF, and ]. The time tag line may come after spaces or tabs. So [ti:Rain] then [00:12.00]Rain
// matches, and [1234:45.67] does not. The content must have no leading BOM or white space and LF line endings,
// as stbox.Detect passes it.
func Detect(content []byte) bool {
	return detectStartRegexp.Match(content) && detectLineRegexp.Match(content)
}
