// Package ass reads and writes ASS (Advanced SubStation Alpha) and its older form SSA (SubStation Alpha v4.00).
//
// The reader turns the override tags that core markup holds into core markup and keeps the header, the styles,
// the other sections and each event in the ass format data. The writer writes an event that did not change as
// stored, so tags such as \pos, \fad and drawings survive a retiming. A subtitle read from SSA writes SSA.
package ass

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.ASS)

// ReadOptions holds the settings of the ASS reader. It has no fields yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the ASS and SSA writer.
type WriteOptions struct {
	// KaraokeTag is the override tag of a karaoke syllable in a cue that the writer converts. Zero writes \k.
	KaraokeTag KaraokeTag
	// Style holds comma-separated Field=Value pairs that change the Default style, as FFmpeg force_style. Empty keeps the styles.
	Style string
}

// KaraokeTag is the ASS override tag that marks the time of a karaoke syllable.
type KaraokeTag string

// The karaoke tags. The zero value "" acts as KaraokeInstant.
const (
	KaraokeInstant KaraokeTag = "k"  // highlights the whole syllable when its time starts
	KaraokeFill    KaraokeTag = "kf" // fills the syllable from left to right
	KaraokeOutline KaraokeTag = "ko" // hides the outline of the syllable until its time starts
)

// String returns the tag name without the backslash, "k" for the zero value.
func (k KaraokeTag) String() string {
	if k == "" {
		return string(KaraokeInstant)
	}
	return string(k)
}

// Validate reports an invalid Style or KaraokeTag.
func (o WriteOptions) Validate() error {
	switch o.KaraokeTag {
	case "", KaraokeInstant, KaraokeFill, KaraokeOutline:
	default:
		return &model.InvalidArgumentError{
			Option: "WriteOptions.ASS.KaraokeTag",
			Msg:    "The ASS karaoke tag " + strconv.Quote(string(o.KaraokeTag)) + " is unknown. Use one of: k, kf, ko.",
		}
	}
	if o.Style == "" {
		return nil
	}
	_, err := parseStyle(o.Style)
	return err
}

// signatureRegexp is the signature of PHP FormatDetector: a first line [Script Info] in any case.
var signatureRegexp = regexp.MustCompile(`(?i)\A\[Script Info\][ \t]*(?:\n|\z)`)

// Detect reports whether content has the ASS signature of stbox.Detect. The content must have no leading
// BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}

//nolint:gochecknoglobals // the default field lists of the Format lines
var (
	assStyleFields = []string{
		"Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "OutlineColour", "BackColour",
		"Bold", "Italic", "Underline", "StrikeOut", "ScaleX", "ScaleY", "Spacing", "Angle",
		"BorderStyle", "Outline", "Shadow", "Alignment", "MarginL", "MarginR", "MarginV", "Encoding",
	}
	ssaStyleFields = []string{
		"Name", "Fontname", "Fontsize", "PrimaryColour", "SecondaryColour", "TertiaryColour", "BackColour",
		"Bold", "Italic", "BorderStyle", "Outline", "Shadow", "Alignment", "MarginL", "MarginR", "MarginV",
		"AlphaLevel", "Encoding",
	}
	assEventFields = []string{"Layer", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"}
	ssaEventFields = []string{"Marked", "Start", "End", "Style", "Name", "MarginL", "MarginR", "MarginV", "Effect", "Text"}
	// defaultStyle holds the values of the Default style of a new file, in the order of assStyleFields.
	defaultStyle = []string{
		"Default", "Arial", "16", "&H00FFFFFF", "&H00FFFFFF", "&H00000000", "&H00000000",
		"0", "0", "0", "0", "100", "100", "0", "0", "1", "1", "0", "2", "10", "10", "10", "1",
	}
)

const (
	scriptInfoSection = "Script Info"
	assStylesSection  = "V4+ Styles"
	ssaStylesSection  = "V4 Styles"
	eventsSection     = "Events"
	ssaScriptType     = "v4.00"
)

// phpTrimSet holds the bytes that PHP trim removes without a character list.
const phpTrimSet = " \t\n\r\x00\v"

func phpTrim(s string) string {
	return strings.Trim(s, phpTrimSet)
}

// equalFold compares a and b without ASCII case, as PHP strcasecmp does.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lower(a[i]) != lower(b[i]) {
			return false
		}
	}
	return true
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

func isStylesSection(name string) bool {
	return equalFold(name, assStylesSection) || equalFold(name, ssaStylesSection)
}
