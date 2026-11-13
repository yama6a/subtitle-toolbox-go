package chapters

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const space = `[ \t\n\v\f\r]*`

var (
	ogmTimestamp = regexp.MustCompile(`^CHAPTER[0-9]+` + space + `=` + space + `([0-9]+)` + space + `:` + space + `([0-9]+)` + space +
		`:` + space + `([0-9]+)` + space + `[.,]` + space + `([0-9]{1,9})`)
	ogmName = regexp.MustCompile(`^CHAPTER[0-9]+NAME` + space + `=(.*)$`)
)

// ReadOGM reads the simple chapter format of mkvmerge: a CHAPTERxx=hh:mm:ss.mmm line and a CHAPTERxxNAME= line for
// each chapter. It ignores the Lenient option for its own lines.
func ReadOGM(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.OGMChapters, err)
	}
	st := parsing.NewState(model.OGMChapters, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	var cues []model.Cue
	var start time.Duration
	pending := false
	for i, line := range physicalLines(string(content)) {
		line = strings.Trim(line, markupcore.PHPTrimSet)
		if line == "" {
			continue
		}
		if !pending {
			if start, err = ogmStart(line, i+1); err != nil {
				return nil, nil, err
			}
			pending = true
			continue
		}
		m := ogmName.FindStringSubmatch(line)
		if m == nil {
			return nil, nil, parseError(model.OGMChapters, i+1, `The line "`+line+`" is not a CHAPTERxxNAME= line.`)
		}
		cues = append(cues, escapedCue(start, m[1]))
		pending = false
	}
	s.AddCues(endChapters(cues, nil, opts.MediaDuration)...)
	return s, st.Warnings(), nil
}

// ogmStart reads a CHAPTERxx= line. The fraction digits are a decimal fraction, rounded to whole milliseconds.
func ogmStart(line string, number int) (time.Duration, error) {
	m := ogmTimestamp.FindStringSubmatch(line)
	if m == nil {
		return 0, parseError(model.OGMChapters, number, `The line "`+line+`" is not a CHAPTERxx= line.`)
	}
	hours, minutes, seconds := capped(m[1]), capped(m[2]), capped(m[3])
	if minutes > 59 || seconds > 59 {
		return 0, parseError(model.OGMChapters, number, `The line "`+line+`" has a minute or second above 59.`)
	}
	fraction, _ := strconv.ParseInt((m[4] + "00000000")[:9], 10, 64)
	ms := (hours*3600+minutes*60+seconds)*1000 + (fraction+500_000)/1_000_000
	if ms >= limitMillis {
		e := parsing.OverLimit(line, number)
		e.Format = model.OGMChapters
		return 0, e
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// WriteOGM writes the simple chapter format of mkvmerge. Ends are lost.
func WriteOGM(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.OGMChapters, common)
	if err != nil {
		return err
	}
	var b strings.Builder
	for i, c := range s.All() {
		fmt.Fprintf(&b, "CHAPTER%02d=%s\nCHAPTER%02dNAME=%s\n", i+1, markupcore.CoreTimestamp(c.Start), i+1,
			strings.Join(markupcore.PlainLines(c.Lines), " "))
	}
	return writing.Write(w, model.OGMChapters, []byte(b.String()), common, false)
}
