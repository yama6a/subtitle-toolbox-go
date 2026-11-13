// Package sprucestl reads and writes Spruce STL, the text subtitle file of DVD Studio Pro and Spruce DVDMaestro.
//
// A cue is one line "start , end , text". The times count frames, and the file does not name its frame rate. The text
// has "|" as the line break and ^B, ^I and ^U as the toggles of bold, italics and underline. A "$" line such as
// "$VertAlign = Top" sets the style and the position of every later cue. The reader keeps the "$" lines, the "//"
// comments and the shape of the file in the spruce-stl format data, so a round trip keeps them.
//
// Spruce STL shares the extension .stl with EBU STL.
package sprucestl

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.SpruceSTL)

const (
	readFrameRateOption  = "ReadOptions.SpruceSTL.FrameRate"
	writeFrameRateOption = "WriteOptions.SpruceSTL.FrameRate"
)

// ReadOptions holds the settings of the Spruce STL reader.
type ReadOptions struct {
	// FrameRate is the frame rate of the time codes. The zero value guesses it: 30 fps when a frame number is 25 or
	// more, else 25 fps.
	FrameRate model.FrameRate
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that fails model.FrameRate.Validate.
func (o ReadOptions) Validate() error {
	if o.FrameRate.IsZero() {
		return nil
	}
	return parsing.FrameRateOption(o.FrameRate.Validate(), readFrameRateOption, "The frame rate")
}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

// WriteOptions holds the settings of the Spruce STL writer.
type WriteOptions struct {
	// FrameRate is the output frame rate. The zero value takes the frame rate that Read stored, else 25.
	FrameRate model.FrameRate
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that fails model.FrameRate.Validate.
func (o WriteOptions) Validate() error {
	if o.FrameRate.IsZero() {
		return nil
	}
	return parsing.FrameRateOption(o.FrameRate.Validate(), writeFrameRateOption, "The Spruce STL frame rate")
}

const timing = `\d{2}:[0-5]\d:[0-5]\d:\d{2}[ \t]*,[ \t]*\d{2}:[0-5]\d:[0-5]\d:\d{2}[ \t]*,`

var (
	afterCommandsRegexp = regexp.MustCompile(`\A(?:(?:\$|//)[^\n]*\n[ \t\n]*)+` + timing)
	cueStartRegexp      = regexp.MustCompile(`\A` + timing)
)

// Detect reports whether content starts with "$" or "//" lines and a cue line, or with a cue line whose text does
// not start with a double quote. A headerless CSV row such as 00:00:01:10,00:00:03:10,"Hello" has that quote.
// The cue line needs exactly 2 hour digits, but the reader takes more. The content must have no leading BOM or white
// space and LF line ends, as stbox.Detect passes it.
func Detect(content []byte) bool {
	if afterCommandsRegexp.Match(content) {
		return true
	}
	loc := cueStartRegexp.FindIndex(content)
	if loc == nil {
		return false
	}
	return !bytes.HasPrefix(bytes.TrimLeft(content[loc[1]:], " \t"), []byte(`"`))
}

// frameLimit returns the rate rounded up. A frame number must be below it.
func frameLimit(r model.FrameRate) int {
	return int((r.Num + r.Den - 1) / r.Den)
}

// style is the state that the "$" commands set. It applies to every cue until a command changes it.
type style struct {
	bold, italic, underline bool
	// row is the vertical alignment: 0 bottom, 1 middle, 2 top. column is the horizontal one: 1 left, 2 center, 3 right.
	row, column int
}

func defaultStyle() style {
	return style{row: 0, column: 2}
}

// The command values of $HorzAlign and $VertAlign, indexed by column and by row.
var (
	horizontal = [...]string{1: "Left", 2: "Center", 3: "Right"} //nolint:gochecknoglobals // a fixed list
	vertical   = [...]string{"Bottom", "Center", "Top"}          //nolint:gochecknoglobals // a fixed list
)

var commandRegexp = regexp.MustCompile(`^\$([A-Za-z0-9]+)[ \t]*=[ \t]*(.*?)[ \t]*$`)

// apply changes s as the command of line says. $Bold, $Italic and $Underlined take True or False. $HorzAlign takes
// Left, Center or Right and $VertAlign takes Top, Center or Bottom. Another line or value leaves s as it is.
func (s *style) apply(line string) {
	m := commandRegexp.FindStringSubmatch(strings.Trim(line, markupcore.PHPTrimSet))
	if m == nil {
		return
	}
	name, value := markupcore.ASCIILower(m[1]), markupcore.ASCIILower(m[2])
	switch name {
	case "bold":
		setFlag(&s.bold, value)
	case "italic":
		setFlag(&s.italic, value)
	case "underlined":
		setFlag(&s.underline, value)
	case "horzalign":
		s.column = position(horizontal[:], value, s.column)
	case "vertalign":
		s.row = position(vertical[:], value, s.row)
	}
}

func setFlag(flag *bool, value string) {
	switch value {
	case "true":
		*flag = true
	case "false":
		*flag = false
	}
}

// position returns the index of the name in names that equals value in any case, else fallback.
func position(names []string, value string, fallback int) int {
	for i, n := range names {
		if n != "" && markupcore.ASCIILower(n) == value {
			return i
		}
	}
	return fallback
}

func (s *style) alignment() model.Alignment {
	a, _ := model.AlignmentAt(s.row, s.column)
	return a
}
