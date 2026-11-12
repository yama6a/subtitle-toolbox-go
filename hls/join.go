package hls

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// JoinOptions holds the settings of Join. The zero value uses the defaults.
type JoinOptions struct {
	// StreamStart is the MPEGTS value that the joined times count from. Nil takes the MPEGTS of the first segment.
	StreamStart *int64
}

func (o JoinOptions) validate() error {
	if s := o.StreamStart; s != nil && (*s < 0 || *s >= MPEGTSWrap) {
		return invalid("JoinOptions.StreamStart", "The stream start must be a 33-bit timestamp from 0 to "+
			strconv.FormatInt(MPEGTSWrap-1, 10)+", got "+strconv.FormatInt(*s, 10)+".")
	}
	return nil
}

// start returns the stream start: the option, else the MPEGTS of the map of the first segment.
func (o JoinOptions) start(first TimestampMap) int64 {
	if o.StreamStart != nil {
		return *o.StreamStart
	}
	return first.MPEGTS
}

type cueKey struct {
	start, end time.Duration
	text       string
}

// Join reads the WebVTT segments in playlist order and returns one subtitle with cue times counted from the stream
// start. A segment without an X-TIMESTAMP-MAP header counts as MPEGTS 0 and LOCAL 0, as RFC 8216 section 3.5 says.
// Join keeps the header text, the STYLE blocks and the REGION blocks of the first segment. It drops a cue with the
// same start, end and text as an earlier cue, and then joins the cues that RemoveDuplicateCues(0) joins. So a cue
// that Segment split at a segment boundary is whole again.
func Join(segments [][]byte, opts JoinOptions) (*model.Subtitle, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	joined := &model.Subtitle{}
	var start int64
	var cues []model.Cue
	seen := map[cueKey]bool{}
	for i, content := range segments {
		segment, m, err := readSegment(content)
		if err != nil {
			return nil, fmt.Errorf("read segment %d: %w", i, err)
		}
		if i == 0 {
			start = opts.start(m)
			if err := joined.SetFormatData(formatDataKey, withoutMap(segment)); err != nil {
				return nil, err
			}
		}
		if err := segment.Shift(m.Offset(start)); err != nil {
			return nil, err
		}
		for _, c := range segment.All() {
			if key := (cueKey{c.Start, c.End, c.Text()}); !seen[key] {
				seen[key] = true
				cues = append(cues, c)
			}
		}
	}
	joined.AddCues(cues...)
	if err := joined.RemoveDuplicateCues(0); err != nil {
		return nil, err
	}
	return joined, nil
}

// readSegment reads a segment in strict mode and returns it with its map, which is MPEGTS 0 and LOCAL 0 without a header.
func readSegment(content []byte) (*model.Subtitle, TimestampMap, error) {
	segment, _, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
	if err != nil {
		return nil, TimestampMap{}, err
	}
	m, _, err := TimestampMapOf(segment)
	if err == nil {
		err = m.Validate()
	}
	return segment, m, err
}

// withoutMap returns the WebVTT file data of s without the map lines. It has no headerLines when none are left.
func withoutMap(s *model.Subtitle) *model.Data {
	data, _ := s.FormatData(formatDataKey)
	if data == nil {
		return nil
	}
	lines, _ := data.Strings("headerLines")
	var kept []any
	for _, l := range lines {
		if !isMapLine(l) {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		data.Delete("headerLines")
	} else {
		data.Set("headerLines", kept)
	}
	return data
}
