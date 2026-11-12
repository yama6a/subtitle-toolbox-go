package sprucestl

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// defaultHeader holds the commands that DVD Studio Pro can need to import a file, and an empty line.
//
//nolint:gochecknoglobals // fixed header lines
var defaultHeader = []string{
	"$FontName = Arial",
	"$FontSize = 42",
	"$HorzAlign = Center",
	"$VertAlign = Bottom",
	"$XOffset = 0",
	"$YOffset = 0",
	"$Bold = False",
	"$Italic = False",
	"$Underlined = False",
	"$TextContrast = 15",
	"$Outline1Contrast = 15",
	"$Outline2Contrast = 15",
	"$BackgroundContrast = 0",
	"$ForceDisplay = False",
	"$FadeIn = 0",
	"$FadeOut = 0",
	"$TapeOffset = False",
	"",
}

// maxFrameLimit is the largest frame rate, rounded up, whose frame numbers have 2 digits.
const maxFrameLimit = 100

const (
	defaultSeparator = " , "
	defaultLineBreak = "|"
)

var (
	separatorRegexp = regexp.MustCompile(`^[ \t]*,[ \t]*$`)
	breakRegexp     = regexp.MustCompile(`^[ \t]*\|[ \t]*$`)
	tagRegexp       = regexp.MustCompile(`(?i)^<(/?)([biu])>$`)
	caretRegexp     = regexp.MustCompile(`\^([BIU])`)
	lineEnds        = strings.NewReplacer("\r", " ", "\n", " ") //nolint:gochecknoglobals // a fixed replacer
)

// Write writes s as Spruce STL. A stored header, command or trailer line with CR or LF writes those characters as
// spaces. A | in the text becomes U+00A6 BROKEN BAR, and a ^ before B, I or U gets U+2060 WORD JOINER after it, so
// neither reads back as a line break or a toggle.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	file, _ := s.FormatData(formatDataKey)
	rate, err := outputRate(opts, file)
	if err != nil {
		return WriteReport{}, err
	}
	if frameLimit(rate) > maxFrameLimit {
		return WriteReport{}, &model.UnwritableError{
			Format: model.SpruceSTL, CueIndex: -1,
			Reason: writeFrameRateOption + ": The frame number has 2 digits, so the Spruce STL frame rate must not be above " +
				strconv.Itoa(maxFrameLimit) + " fps, got " + timecode.Number(rate.Float64()) + ".",
		}
	}
	separator := storedOr(file, "separator", separatorRegexp, defaultSeparator)
	lineBreak := storedOr(file, "lineBreak", breakRegexp, defaultLineBreak)
	header, ok := file.Strings("header")
	if !ok {
		header = defaultHeader
	}

	st := defaultStyle()
	var body []byte
	for _, line := range header {
		body = appendStored(body, line, &st, true)
	}
	comments := make([][]string, s.Len()+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, s.Len())
		comments[at] = append(comments[at], c.Text)
	}
	for i, c := range s.All() {
		body = appendComments(body, comments[i])
		data, _ := c.FormatData(formatDataKey)
		commands, _ := data.Strings("commands")
		for _, command := range commands {
			body = appendStored(body, command, &st, false)
		}
		body = appendAlignment(body, c.Position.Alignment, &st)
		body = append(body, timeCode(c.Start, rate)+separator+timeCode(c.End, rate)+separator+writtenText(c, st, lineBreak)+"\n"...)
	}
	body = appendComments(body, comments[s.Len()])
	trailer, _ := file.Strings("trailer")
	for _, line := range trailer {
		body = appendStored(body, line, &st, false)
	}
	return WriteReport{}, writing.Write(w, model.SpruceSTL, body, common, false)
}

// storedOr returns the stored string of key when it matches shape, else fallback.
func storedOr(file *model.Data, key string, shape *regexp.Regexp, fallback string) string {
	if v, _ := file.String(key); shape.MatchString(v) {
		return v
	}
	return fallback
}

// outputRate returns the rate of the option, else the stored rate, else 25 fps.
func outputRate(opts WriteOptions, file *model.Data) (model.FrameRate, error) {
	if !opts.FrameRate.IsZero() {
		return opts.FrameRate, nil
	}
	stored, ok := file.Number("frameRate")
	if !ok {
		return model.FrameRate{Num: 25, Den: 1}, nil
	}
	rate, err := model.FrameRateFromFloat(stored)
	return rate, parsing.FrameRateOption(err, writeFrameRateOption, "The Spruce STL frame rate")
}

// appendStored writes a stored header, command or trailer line, with CR and LF as spaces, and applies it to st. It
// skips a line that the reader would not read as that kind of line: only a header line may be blank or a // comment,
// and every other line must be a $ command.
func appendStored(body []byte, line string, st *style, isHeader bool) []byte {
	line = lineEnds.Replace(line)
	trimmed := strings.Trim(line, markupcore.PHPTrimSet)
	isCommand := strings.HasPrefix(trimmed, "$")
	isHeaderOnly := trimmed == "" || strings.HasPrefix(trimmed, "//")
	if isCommand || isHeader && isHeaderOnly {
		return appendCommand(body, line, st)
	}
	return body
}

// appendCommand writes a header, command or trailer line and applies it to st.
func appendCommand(body []byte, line string, st *style) []byte {
	st.apply(line)
	return append(append(body, line...), '\n')
}

func appendComments(body []byte, comments []string) []byte {
	for _, text := range comments {
		for line := range strings.SplitSeq(markupcore.NormalizeLineEnds(text), "\n") {
			body = append(append(body, "//"...), line+"\n"...)
		}
	}
	return body
}

// appendAlignment writes the $HorzAlign and $VertAlign commands that move the cue from the position of st to a.
func appendAlignment(body []byte, a model.Alignment, st *style) []byte {
	if a.Column() != st.column {
		body = appendCommand(body, "$HorzAlign = "+horizontal[a.Column()], st)
	}
	if a.Row() != st.row {
		body = appendCommand(body, "$VertAlign = "+vertical[a.Row()], st)
	}
	return body
}

// timeCode returns d as hh:mm:ss:ff. It stops at the largest time code below 100000 hours.
func timeCode(d time.Duration, rate model.FrameRate) string {
	limit := frameLimit(rate)
	d = max(d.Round(time.Millisecond), 0)
	whole := int64(d / time.Second)
	frame := int(rate.Frames(d % time.Second))
	if frame >= limit {
		whole++
		frame = 0
	}
	if whole >= timecode.MaxHours*3600 {
		whole, frame = timecode.MaxHours*3600-1, limit-1
	}
	return fmt.Sprintf("%02d:%02d:%02d:%02d", whole/3600, whole/60%60, whole%60, frame)
}

// writtenText returns the lines of c joined with lineBreak. A toggle goes before each text that needs another bold,
// italics or underline state than the one before. The first state is the one of the commands.
func writtenText(c model.Cue, st style, lineBreak string) string {
	split := make([]string, 0, len(c.Lines))
	for _, line := range c.Lines {
		split = append(split, markupcore.SplitLineBreaks(markupcore.RubyAsText(line))...)
	}
	current := [3]bool{st.bold, st.italic, st.underline}
	var lines []string
	for _, line := range markupcore.CarryOpenTags(split) {
		if text := strings.Trim(textLine(line, &current), markupcore.PHPTrimSet); text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, lineBreak)
}

// textLine writes the text of a line with toggles. current is the style so far, as bold, italics and underline.
func textLine(line string, current *[3]bool) string {
	var open [3]int
	var b strings.Builder
	for i, token := range markupcore.SplitTags(line) {
		if i%2 == 1 {
			if m := tagRegexp.FindStringSubmatch(token); m != nil {
				flag := strings.Index("biu", markupcore.ASCIILower(m[2]))
				open[flag] = max(0, open[flag]+1-2*len(m[1]))
			}
			continue
		}
		token = markupcore.DecodeEntities(token)
		if strings.Trim(token, markupcore.PHPTrimSet) == "" {
			b.WriteString(token)
			continue
		}
		for flag, toggle := range [...]string{"^B", "^I", "^U"} {
			if current[flag] != (open[flag] > 0) {
				current[flag] = !current[flag]
				b.WriteString(toggle)
			}
		}
		b.WriteString(escape(token, strings.HasSuffix(b.String(), "^")))
	}
	return b.String()
}

// escape returns text so that the reader reads it back as text: | becomes U+00A6 and ^ before B, I or U gets a word
// joiner. afterCaret is true when the text before it ends in a ^ that is no toggle.
func escape(text string, afterCaret bool) string {
	text = strings.ReplaceAll(text, "|", "\u00a6")
	text = caretRegexp.ReplaceAllString(text, "^"+writing.WordJoiner+"$1")
	if afterCaret && strings.IndexAny(text, "BIU") == 0 {
		text = writing.WordJoiner + text
	}
	return text
}
