package hls

import (
	"bytes"
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// DefaultSegmentDuration is the segment duration that Segment uses when SegmentOptions.SegmentDuration is 0.
const DefaultSegmentDuration = 6 * time.Second

// DefaultFileNamePattern is the file name pattern that Segment uses when SegmentOptions.FileNamePattern is empty.
const DefaultFileNamePattern = "sub%d.vtt"

var verbRegexp = regexp.MustCompile(`%\d*d`)

// SegmentOptions holds the settings of Segment. The zero value uses the defaults.
type SegmentOptions struct {
	// SegmentDuration is the duration of each segment, rounded to whole milliseconds. 0 means DefaultSegmentDuration.
	SegmentDuration time.Duration
	// Map is the map that each segment header carries. Nil means MPEGTS DefaultMPEGTS and LOCAL 0.
	// It is a pointer, because an MPEGTS of 0 is legal.
	Map *TimestampMap
	// FileNamePattern is the file name of segment n, with n counted from 0. After "%%" is removed, it must hold
	// exactly one verb that starts with % and ends with d, such as %d or %03d. The empty string means
	// DefaultFileNamePattern.
	FileNamePattern string
	// MediaDuration is the duration of the video, so that the playlist covers all of it, rounded to whole
	// milliseconds. 0 means the end of the last cue.
	MediaDuration time.Duration
}

// Validate returns an error that matches model.ErrInvalidArgument when a field of o is out of range.
func (o SegmentOptions) Validate() error {
	if o.SegmentDuration != 0 && o.SegmentDuration.Round(time.Millisecond) < time.Millisecond {
		return invalid("SegmentOptions.SegmentDuration",
			"The segment duration must be at least 0.001 s, got "+timecode.Seconds(o.SegmentDuration)+".")
	}
	if o.Map != nil {
		if err := o.Map.Validate(); err != nil {
			return err
		}
	}
	if p := strings.ReplaceAll(o.FileNamePattern, "%%", ""); o.FileNamePattern != "" &&
		(strings.Count(p, "%") != 1 || !verbRegexp.MatchString(p)) {
		return invalid("SegmentOptions.FileNamePattern",
			`The file name pattern must hold exactly one %d and no other placeholder, got "`+o.FileNamePattern+`".`)
	}
	if o.MediaDuration != 0 && o.MediaDuration.Round(time.Millisecond) < time.Millisecond {
		return invalid("SegmentOptions.MediaDuration",
			"The media duration must be at least 0.001 s, got "+timecode.Seconds(o.MediaDuration)+".")
	}
	return nil
}

// Rendition is the segments and the media playlist of one subtitle rendition.
type Rendition struct {
	// cues are copies of the cues with an identifier, shifted by the LOCAL time of the map.
	cues []model.Cue
	// starts and ends are the cue times in milliseconds before the shift. The window of a segment uses them.
	starts, ends []int64
	fileData     *model.Data
	pattern      string
	segmentMS    int64
	totalMS      int64
}

// Segment cuts s into WebVTT segments of the duration of opts. A segment holds every cue that overlaps it, with
// its full times. A cue without an identifier gets its 1-based index in s, so it has the same identifier in every
// segment. The header text, the STYLE blocks and the REGION blocks of s go into each segment, and its comments do
// not. Segment writes no segment until the caller asks for it.
func Segment(s *model.Subtitle, opts SegmentOptions) (*Rendition, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	m := TimestampMap{MPEGTS: DefaultMPEGTS}
	if opts.Map != nil {
		m = *opts.Map
	}
	r := &Rendition{
		pattern:   cmp.Or(opts.FileNamePattern, DefaultFileNamePattern),
		segmentMS: cmp.Or(opts.SegmentDuration, DefaultSegmentDuration).Round(time.Millisecond).Milliseconds(),
	}
	cues := s.Cues()
	r.totalMS = totalMilliseconds(opts.MediaDuration, cues)
	if r.totalMS == 0 {
		return nil, invalid("SegmentOptions.MediaDuration",
			"The subtitle has no cue that ends after 0 s. Set the mediaDuration option to segment it.")
	}

	if err := r.load(s, cues, m); err != nil {
		return nil, err
	}
	return r, nil
}

// totalMilliseconds returns the media duration, or the largest cue end when the media duration is 0.
func totalMilliseconds(media time.Duration, cues []model.Cue) int64 {
	if media != 0 {
		return media.Round(time.Millisecond).Milliseconds()
	}
	var total int64
	for _, c := range cues {
		total = max(total, c.End.Milliseconds())
	}
	return total
}

// load fills the cues, their times and the file data of r.
func (r *Rendition) load(s *model.Subtitle, cues []model.Cue, m TimestampMap) error {
	for i := range cues {
		if cues[i].ID == "" {
			cues[i].ID = strconv.Itoa(i + 1)
		}
	}
	var copied model.Subtitle
	copied.AddCues(cues...)
	cues = copied.Cues()
	r.starts, r.ends = make([]int64, len(cues)), make([]int64, len(cues))
	for i, c := range cues {
		r.starts[i], r.ends[i] = c.Start.Milliseconds(), c.End.Milliseconds()
	}
	if err := copied.Shift(m.Local); err != nil {
		return err
	}
	r.cues = copied.Cues()

	r.fileData, _ = s.FormatData(formatDataKey)
	if r.fileData == nil {
		r.fileData = model.NewData()
	}
	lines, _ := r.fileData.Strings("headerLines")
	headerLines := []any{m.Header()}
	for _, l := range lines {
		if !isMapLine(l) {
			headerLines = append(headerLines, l)
		}
	}
	r.fileData.Set("headerLines", headerLines)
	return nil
}

// Count returns the number of segments.
func (r *Rendition) Count() int {
	return int((r.totalMS + r.segmentMS - 1) / r.segmentMS)
}

// EachSegment writes each segment in playlist order and calls fn with its file name and content. It writes one
// segment at a time, so its memory does not grow with the media duration. Each call writes the segments again. fn
// must not change content. When fn returns an error, EachSegment stops and returns it.
func (r *Rendition) EachSegment(fn func(name string, content []byte) error) error {
	var empty []byte
	var active []int
	next := 0
	for i, start := 0, int64(0); start < r.totalMS; i, start = i+1, start+r.segmentMS {
		end := min(start+r.segmentMS, r.totalMS)
		for ; next < len(r.starts) && r.starts[next] < end; next++ {
			active = append(active, next)
		}
		active = slices.DeleteFunc(active, func(c int) bool { return r.leaves(c, start) })
		content := empty
		if len(active) > 0 || empty == nil {
			cues := make([]model.Cue, len(active))
			for j, c := range active {
				cues[j] = r.cues[c]
			}
			var err error
			if content, err = r.write(cues); err != nil {
				return err
			}
			if len(active) == 0 {
				empty = content
			}
		}
		if err := fn(fmt.Sprintf(r.pattern, i), content); err != nil {
			return err
		}
	}
	return nil
}

// leaves reports whether cue c is not in the segment that starts at start. A cue of zero length stays only in the
// segment where it starts.
func (r *Rendition) leaves(c int, start int64) bool {
	if r.ends[c] == r.starts[c] {
		return r.starts[c] < start
	}
	return r.ends[c] <= start
}

func (r *Rendition) write(cues []model.Cue) ([]byte, error) {
	var s model.Subtitle
	if err := s.SetFormatData(formatDataKey, r.fileData); err != nil {
		return nil, err
	}
	s.AddCues(cues...)
	var buf bytes.Buffer
	if _, err := webvtt.Write(&buf, &s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SegmentDuration is the file name and the duration of one segment.
type SegmentDuration struct {
	// Name is the file name of the segment.
	Name string
	// Duration is the length of the segment in whole milliseconds.
	Duration time.Duration
}

// Durations returns the file name and the duration of each segment in playlist order.
func (r *Rendition) Durations() []SegmentDuration {
	out := make([]SegmentDuration, 0, r.Count())
	for i, start := 0, int64(0); start < r.totalMS; i, start = i+1, start+r.segmentMS {
		out = append(out, SegmentDuration{
			Name:     fmt.Sprintf(r.pattern, i),
			Duration: time.Duration(min(start+r.segmentMS, r.totalMS)-start) * time.Millisecond,
		})
	}
	return out
}

// Playlist returns the VOD media playlist, as RFC 8216 section 4.3 defines it. The target duration is the largest
// segment duration in seconds, rounded to the nearest integer, so each EXTINF value rounded to the nearest integer
// does not exceed it (section 4.3.3.1). The decimal EXTINF values need protocol version 3 (section 7).
func (r *Rendition) Playlist() []byte {
	durations := r.Durations()
	target := int64(1)
	for _, d := range durations {
		target = max(target, (d.Duration.Milliseconds()+500)/1000)
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:" + strconv.FormatInt(target, 10) +
		"\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	for _, d := range durations {
		ms := d.Duration.Milliseconds()
		fmt.Fprintf(&b, "#EXTINF:%d.%03d,\n%s\n", ms/1000, ms%1000, d.Name)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return []byte(b.String())
}
