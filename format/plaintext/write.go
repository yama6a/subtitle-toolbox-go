package plaintext

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/paragraphs"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var spacesRegexp = regexp.MustCompile(`[ \t]+`)

// Write writes the text of s as plain text paragraphs. A cue without text is skipped. Markup and entities are
// resolved as in markupcore.PlainText, so the output always holds the words only. StripTags changes nothing.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	var items []paragraphs.Item[string]
	for _, c := range s.All() {
		lines := textLines(c.Lines)
		if len(lines) == 0 {
			continue
		}
		items = append(items, paragraphs.Item[string]{Start: c.Start, End: c.End, Value: strings.Join(lines, lineSeparator(opts.KeepLineBreaks))})
	}
	blocks := make([]string, 0, len(items))
	for _, p := range paragraphs.ByGap(items, opts.gap(), nil) {
		var b strings.Builder
		if opts.WithTimes {
			seconds := max(p.Start, 0) / time.Second
			fmt.Fprintf(&b, "[%02d:%02d:%02d] ", seconds/3600, seconds/60%60, seconds%60)
		}
		b.WriteString(strings.Join(p.Values, lineSeparator(opts.OneCuePerLine)))
		b.WriteByte('\n')
		blocks = append(blocks, b.String())
	}
	return writing.Write(w, model.PlainText, []byte(strings.Join(blocks, "\n")), common, false)
}

func lineSeparator(newLine bool) string {
	if newLine {
		return "\n"
	}
	return " "
}

func textLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		line = spacesRegexp.ReplaceAllLiteralString(markupcore.PlainText(markupcore.RubyAsText(line)), " ")
		if line = strings.Trim(line, markupcore.PHPTrimSet); line != "" {
			out = append(out, line)
		}
	}
	return out
}
