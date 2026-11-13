package chapters

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// capDigits stops digit counting above the 100000 hours limit, so long digit runs cannot overflow.
const capDigits = 1 << 40

// timeFields holds the digit runs of a time such as 1:02:48.
type timeFields struct {
	hours, minutes, seconds string
	hasHours                bool
}

// ReadYouTube reads the lines of r that start or end with a time such as 2:48 or 1:02:48, for example a video
// description. It ignores other lines and the Lenient option.
func ReadYouTube(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.YouTubeChapters, err)
	}
	st := parsing.NewState(model.YouTubeChapters, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	var cues []model.Cue
	for _, line := range parsing.Lines(string(content)) {
		line = strings.Trim(line, markupcore.PHPTrimSet)
		f, title, ok := leadingTime(line)
		if !ok {
			f, title, ok = trailingTime(line)
		}
		if !ok {
			continue
		}
		start, ok := f.duration()
		if !ok {
			continue
		}
		c := model.Cue{Start: start}
		if title = trimSeparators(title); title != "" {
			c.Lines = []string{markupcore.EscapeText(title)}
		}
		cues = append(cues, c)
	}
	s.AddCues(endChapters(cues, nil, opts.MediaDuration)...)
	return s, st.Warnings(), nil
}

// WriteYouTube writes one line per chapter: the start as m:ss or h:mm:ss, a space and the title. Ends are lost.
func WriteYouTube(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.YouTubeChapters, common)
	if err != nil {
		return err
	}
	var body []byte
	for _, c := range s.All() {
		secs := max(c.Start, 0) / time.Second
		var line string
		if secs >= 3600 {
			line = fmt.Sprintf("%d:%02d:%02d", secs/3600, secs/60%60, secs%60)
		} else {
			line = fmt.Sprintf("%d:%02d", secs/60, secs%60)
		}
		line += " " + strings.Join(markupcore.PlainLines(c.Lines), " ")
		body = append(body, strings.TrimRight(line, markupcore.PHPTrimSet)...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.YouTubeChapters, body, common, false)
}

func (f timeFields) duration() (time.Duration, bool) {
	seconds, minutes, hours := capped(f.seconds), capped(f.minutes), capped(f.hours)
	if seconds > 59 || (f.hasHours && minutes > 59) {
		return 0, false
	}
	total := hours*3600 + minutes*60 + seconds
	if total >= timecode.MaxHours*3600 {
		return 0, false
	}
	return time.Duration(total) * time.Second, true
}

func capped(digits string) int64 {
	var n int64
	for i := range len(digits) {
		n = min(n*10+int64(digits[i]-'0'), capDigits)
	}
	return n
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isOpener(b byte) bool { return b == '(' || b == '[' }

func isCloser(b byte) bool { return b == ')' || b == ']' }

// separatorAt returns the byte length of the separator that starts s, 0 for none. A separator is a PCRE white
// space byte, "|", ":", "-", U+2013 or U+2014.
func separatorAt(s string) int {
	if s == "" {
		return 0
	}
	switch s[0] {
	case ' ', '\t', '\n', '\v', '\f', '\r', '|', ':', '-':
		return 1
	}
	if strings.HasPrefix(s, "–") || strings.HasPrefix(s, "—") {
		return 3
	}
	return 0
}

// separatorBefore reports whether a separator ends at index p of s.
func separatorBefore(s string, p int) bool {
	return p >= 1 && separatorAt(s[p-1:]) == 1 || p >= 3 && separatorAt(s[p-3:]) == 3
}

func trimSeparators(s string) string {
	for n := separatorAt(s); n > 0; n = separatorAt(s) {
		s = s[n:]
	}
	for {
		switch {
		case len(s) > 0 && separatorAt(s[len(s)-1:]) == 1:
			s = s[:len(s)-1]
		case len(s) >= 3 && separatorAt(s[len(s)-3:]) == 3:
			s = s[:len(s)-3]
		default:
			return s
		}
	}
}

func digitsEnd(s string, i int) int {
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

func digitsStart(s string, i int) int {
	for i > 0 && isDigit(s[i-1]) {
		i--
	}
	return i
}

// leadingTime reads a time at the start of line. The time must end the line or be followed by a separator.
// It tries the form with hours first, as a backtracking pattern would.
func leadingTime(line string) (timeFields, string, bool) {
	p := 0
	if len(line) > 0 && isOpener(line[0]) {
		p = 1
	}
	a := digitsEnd(line, p)
	if a == p || a >= len(line) || line[a] != ':' {
		return timeFields{}, "", false
	}
	if f, rest, ok := leadingWithHours(line, p, a); ok {
		return f, rest, true
	}
	seconds, rest, ok := leadingSeconds(line, a+1)
	return timeFields{minutes: line[p:a], seconds: seconds}, rest, ok
}

// leadingWithHours reads the form with hours, where line[p:a] holds the hours and line[a] is the colon after them.
func leadingWithHours(line string, p, a int) (timeFields, string, bool) {
	b := digitsEnd(line, a+1)
	if b == a+1 || b >= len(line) || line[b] != ':' {
		return timeFields{}, "", false
	}
	seconds, rest, ok := leadingSeconds(line, b+1)
	return timeFields{hours: line[p:a], minutes: line[a+1 : b], seconds: seconds, hasHours: true}, rest, ok
}

// leadingSeconds reads 2 digits at i and an optional closing bracket. It returns the rest of the line when that
// is empty or starts with a separator. A closing bracket is no separator, so the form without it never wins.
func leadingSeconds(line string, i int) (seconds, rest string, ok bool) {
	if i+2 > len(line) || !isDigit(line[i]) || !isDigit(line[i+1]) {
		return "", "", false
	}
	q := i + 2
	if q < len(line) && isCloser(line[q]) {
		q++
	}
	if q < len(line) && separatorAt(line[q:]) == 0 {
		return "", "", false
	}
	return line[i : i+2], line[q:], true
}

// trailingTime reads a time at the end of line. The time starts at the line start or after a separator. It
// returns the shortest title, which is the line before the leftmost start that fits.
func trailingTime(line string) (timeFields, string, bool) {
	e := len(line)
	if e > 0 && isCloser(line[e-1]) {
		e--
	}
	if e < 3 || !isDigit(line[e-2]) || !isDigit(line[e-1]) || line[e-3] != ':' {
		return timeFields{}, "", false
	}
	seconds := line[e-2 : e]
	bEnd := e - 3
	bStart := digitsStart(line, bEnd)
	if bStart == bEnd {
		return timeFields{}, "", false
	}
	if f, title, ok := trailingWithHours(line, bStart, bEnd); ok {
		f.seconds = seconds
		return f, title, true
	}
	if p, ok := trailingStart(line, bStart); ok {
		return timeFields{minutes: line[bStart:bEnd], seconds: seconds}, line[:p], true
	}
	return timeFields{}, "", false
}

// trailingStart returns the start of a time whose digits start at i, with its opening bracket when it has one.
func trailingStart(line string, i int) (int, bool) {
	if i > 0 && isOpener(line[i-1]) && (i == 1 || separatorBefore(line, i-1)) {
		return i - 1, true
	}
	if i == 0 || separatorBefore(line, i) {
		return i, true
	}
	return 0, false
}

// trailingWithHours reads the form with hours, where the minutes are line[bStart:bEnd]. The caller sets the seconds.
func trailingWithHours(line string, bStart, bEnd int) (timeFields, string, bool) {
	if bStart == 0 || line[bStart-1] != ':' {
		return timeFields{}, "", false
	}
	aEnd := bStart - 1
	aStart := digitsStart(line, aEnd)
	if aStart == aEnd {
		return timeFields{}, "", false
	}
	p, ok := trailingStart(line, aStart)
	return timeFields{hours: line[aStart:aEnd], minutes: line[bStart:bEnd], hasHours: true}, line[:p], ok
}
