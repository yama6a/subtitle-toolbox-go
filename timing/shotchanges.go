package timing

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

var (
	ptsTime = regexp.MustCompile(`\bpts_time:\s*(\d+(?:\.\d+)?)`)
	seconds = regexp.MustCompile(`^(\d+)(?:\.(\d+))?$`)
	clock   = regexp.MustCompile(`^\d+:[0-5]\d:[0-5]\d(?:\.\d+)?$`)
)

// ShotChangesFromFFmpegLog returns the pts_time values of an FFmpeg select and showinfo log, sorted, without duplicates.
// Each time rounds to whole milliseconds. It returns a *model.ParseError for a time of 100000 hours or more.
func ShotChangesFromFFmpegLog(log []byte) ([]time.Duration, error) {
	var times []time.Duration
	for _, m := range ptsTime.FindAllSubmatchIndex(log, -1) {
		text := string(log[m[2]:m[3]])
		d, ok := parseSeconds(text)
		if !ok {
			return nil, badShotChange(strings.Count(string(log[:m[0]]), "\n")+1, text)
		}
		times = append(times, d)
	}
	return sorted(times), nil
}

// ShotChangesFromText returns the times of a text with one time per line, sorted, without duplicates.
// A line is seconds or h:mm:ss with an optional fraction. Each time rounds to whole milliseconds.
// It returns a *model.ParseError for a line that is neither, and for a time of 100000 hours or more.
func ShotChangesFromText(text []byte) ([]time.Duration, error) {
	var times []time.Duration
	content := strings.TrimPrefix(string(text), "\xef\xbb\xbf")
	for n, line := range parsing.Lines(content) {
		line = strings.Trim(line, markupcore.PHPTrimSet)
		if line == "" {
			continue
		}
		var d time.Duration
		ok := false
		switch {
		case seconds.MatchString(line):
			d, ok = parseSeconds(line)
		case clock.MatchString(line):
			d, ok = timecode.ClockOrLimit(line)
			ok = ok && d < timecode.MaxTime
		}
		if !ok {
			return nil, badShotChange(n+1, line)
		}
		times = append(times, d)
	}
	return sorted(times), nil
}

// parseSeconds reads digits with an optional fraction as whole milliseconds. ok is false for 100000 hours or more.
func parseSeconds(s string) (d time.Duration, ok bool) {
	m := seconds.FindStringSubmatch(s)
	whole := strings.TrimLeft(m[1], "0")
	if len(whole) > 9 {
		return 0, false
	}
	var millis int64
	for _, c := range whole + (m[2] + "000")[:3] {
		millis = millis*10 + int64(c-'0')
	}
	if len(m[2]) > 3 && m[2][3] >= '5' {
		millis++
	}
	d = time.Duration(millis) * time.Millisecond
	return d, d < timecode.MaxTime
}

func sorted(times []time.Duration) []time.Duration {
	slices.Sort(times)
	return slices.Compact(times)
}

func badShotChange(line int, text string) error {
	return &model.ParseError{
		Line: line,
		Msg:  `The shot change time "` + text + `" is not valid. Use seconds or hh:mm:ss.mmm.`,
	}
}
