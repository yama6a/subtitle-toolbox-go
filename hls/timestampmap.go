// Package hls cuts subtitles into HLS WebVTT segments and joins segments into one subtitle.
//
// HLS (HTTP Live Streaming) delivers a subtitle track as WebVTT segment files that an .m3u8 media playlist lists.
// Each segment starts with an X-TIMESTAMP-MAP header, which maps a WebVTT cue time to the 90 kHz MPEG-2 timestamp
// of the video. See RFC 8216 section 3.5.
package hls

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const (
	// ClockRate is the number of ticks per second of the MPEG-2 clock.
	ClockRate = 90000
	// MPEGTSWrap is the size of the 33-bit timestamp range.
	MPEGTSWrap = 1 << 33
	// DefaultMPEGTS is the MPEGTS of the default map, 10 s.
	DefaultMPEGTS = 900000
	// HeaderName is the name of the WebVTT header that holds the map.
	HeaderName = "X-TIMESTAMP-MAP"
)

// formatDataKey is the key of the WebVTT file data in a subtitle.
const formatDataKey = string(model.WebVTT)

var localRegexp = regexp.MustCompile(`^(?:(\d{2,}):)?([0-5]\d):([0-5]\d)\.(\d{3})$`)

// TimestampMap says that the WebVTT cue time Local plays at the MPEG-2 timestamp MPEGTS.
type TimestampMap struct {
	// MPEGTS is a 33-bit timestamp of the 90 kHz MPEG-2 clock, from 0 to MPEGTSWrap - 1.
	MPEGTS int64
	// Local is a WebVTT cue time. It must not be negative.
	Local time.Duration
}

// ParseTimestampMap reads a header line such as X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000, with the
// attributes in either order. It does not check the range of MPEGTS. Validate does.
func ParseTimestampMap(line string) (TimestampMap, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), HeaderName+"=")
	if !ok {
		return TimestampMap{}, parseError(`The line "` + line + `" does not start with ` + HeaderName + "=.")
	}
	attributes := map[string]string{}
	for attribute := range strings.SplitSeq(body, ",") {
		name, value, _ := strings.Cut(strings.TrimSpace(attribute), ":")
		attributes[asciiUpper(name)] = value
	}
	ticks, okTicks := parseTicks(attributes["MPEGTS"])
	local, okLocal := timecode.Clock(attributes["LOCAL"])
	if !okTicks || !okLocal || !localRegexp.MatchString(attributes["LOCAL"]) {
		return TimestampMap{}, parseError("The " + HeaderName + ` header "` + line + `" needs a LOCAL cue time and an integer MPEGTS value.`)
	}
	return TimestampMap{MPEGTS: ticks, Local: local}, nil
}

// parseTicks reads digits. A number above the int64 range becomes the largest int64, which Validate rejects.
func parseTicks(s string) (int64, bool) {
	if !timecode.IsDigits(s) {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return math.MaxInt64, true
	}
	return n, true
}

func asciiUpper(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, s)
}

func parseError(msg string) error {
	return &model.ParseError{Format: model.WebVTT, Msg: msg}
}

// TimestampMapOf returns the map of the first X-TIMESTAMP-MAP line in the WebVTT header lines of s, or false when
// s has none.
func TimestampMapOf(s *model.Subtitle) (TimestampMap, bool, error) {
	data, _ := s.FormatData(formatDataKey)
	lines, _ := data.Strings("headerLines")
	for _, line := range lines {
		if isMapLine(line) {
			m, err := ParseTimestampMap(line)
			return m, err == nil, err
		}
	}
	return TimestampMap{}, false, nil
}

func isMapLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), HeaderName+"=")
}

// Validate returns an error that matches model.ErrInvalidArgument when MPEGTS is not a 33-bit timestamp or Local is
// negative.
func (m TimestampMap) Validate() error {
	if m.MPEGTS < 0 || m.MPEGTS >= MPEGTSWrap {
		return invalid("TimestampMap.MPEGTS", "The MPEGTS value must be a 33-bit timestamp from 0 to "+
			strconv.FormatInt(MPEGTSWrap-1, 10)+", got "+strconv.FormatInt(m.MPEGTS, 10)+".")
	}
	if m.Local < 0 {
		return invalid("TimestampMap.Local", "The LOCAL time must not be negative, got "+timecode.Seconds(m.Local)+".")
	}
	return nil
}

// Header returns the header line, for example X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000.
func (m TimestampMap) Header() string {
	return HeaderName + "=LOCAL:" + markupcore.CoreTimestamp(m.Local) + ",MPEGTS:" + strconv.FormatInt(m.MPEGTS, 10)
}

// Offset returns the time to add to a cue time to get its time after the stream start, which is an MPEGTS value.
// A difference of 2^32 ticks or more counts as a wrapped 33-bit timestamp, as RFC 8216 section 3.5 asks.
func (m TimestampMap) Offset(streamStart int64) time.Duration {
	ticks := m.MPEGTS - streamStart
	switch {
	case ticks >= MPEGTSWrap/2:
		ticks -= MPEGTSWrap
	case ticks < -MPEGTSWrap/2:
		ticks += MPEGTSWrap
	}
	// One tick lasts 100000/9 ns. A tick count on a half millisecond is a multiple of 9, so it converts exactly.
	return time.Duration(ticks*100_000/9) - m.Local
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "hls." + field, Msg: msg}
}
