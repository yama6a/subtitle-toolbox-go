package parsing

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const (
	quoteMax  = 60
	quoteKeep = 57
)

var wordTimestamp = regexp.MustCompile(markupcore.WordTimestampPattern)

// FrameRateOption returns nil for a nil err. Else it returns the *InvalidArgumentError of FrameRate.Validate or
// FrameRateFromFloat with option set and subject in place of the leading "The frame rate" of the message.
func FrameRateOption(err error, option, subject string) error {
	var aerr *model.InvalidArgumentError
	if !errors.As(err, &aerr) {
		return err
	}
	return &model.InvalidArgumentError{Option: option, Msg: subject + strings.TrimPrefix(aerr.Msg, "The frame rate")}
}

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
