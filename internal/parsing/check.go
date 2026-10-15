package parsing

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	quoteMax  = 60
	quoteKeep = 57
)

var wordTimestamp = regexp.MustCompile(markupcore.WordTimestampPattern)

// BoundedTime returns d, or an error on line when d is 100000 hours or more. The error has no format,
// so the caller passes it to Fail. A caller passes timecode.MaxTime for a time that overflows time.Duration.
func BoundedTime(d time.Duration, text string, line int) (time.Duration, error) {
	if d >= timecode.MaxTime {
		return 0, &model.ParseError{Line: line, Msg: `The time "` + text + `" is not below ` + strconv.Itoa(timecode.MaxHours) + " hours."}
	}
	return d, nil
}

// BoundedField returns d, or an error when the time d of the JSON field path is 100000 hours or more.
func BoundedField(d time.Duration, path string) (time.Duration, error) {
	if d >= timecode.MaxTime {
		return 0, &model.ParseError{Msg: "The field " + path + " is not below " + strconv.Itoa(timecode.MaxHours) + " hours."}
	}
	return d, nil
}

// CheckWordTimestamps returns the BoundedTime error of the first word timestamp in lines that is 100000 hours or more.
func CheckWordTimestamps(lines []string, line int) error {
	for _, l := range lines {
		if !strings.Contains(l, "<") {
			continue
		}
		for _, tag := range wordTimestamp.FindAllString(l, -1) {
			d, ok := markupcore.WordTimestamp(tag)
			if !ok {
				d = timecode.MaxTime
			}
			if _, err := BoundedTime(d, tag[1:len(tag)-1], line); err != nil {
				return err
			}
		}
	}
	return nil
}

// Quote returns text in double quotes for a message. A text of more than 60 runes keeps its first 57 and "...".
func Quote(text string) string {
	if utf8.RuneCountInString(text) <= quoteMax {
		return `"` + text + `"`
	}
	cut := 0
	for range quoteKeep {
		_, size := utf8.DecodeRuneInString(text[cut:])
		cut += size
	}
	return `"` + text[:cut] + `..."`
}
