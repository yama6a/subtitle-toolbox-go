package mkv

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	unreadCodecMessage = "Track %d has the codec %s. The reader extracts only S_TEXT/UTF8, S_TEXT/ASS, S_TEXT/SSA, " +
		"S_TEXT/WEBVTT, S_HDMV/PGS, S_VOBSUB, D_WEBVTT/SUBTITLES, D_WEBVTT/CAPTIONS, D_WEBVTT/DESCRIPTIONS."
	laterCodecMessage = "Track %d has the codec %s. The reader does not extract this codec yet. A later release adds it."
)

// timedBlock is a block with its times in whole milliseconds.
type timedBlock struct {
	startMs, endMs int64
	data           string
	additional     string
}

// Extract reads all blocks of the subtitle track with the TrackNumber number.
func (f *File) Extract(number int, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	s, warnings, err := f.extract(number, common)
	return s, warnings, wrap(err)
}

func (f *File) extract(number int, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t, ok := f.track(number)
	if !ok {
		return nil, nil, &model.InvalidArgumentError{Msg: fmt.Sprintf("The file has no subtitle track with the number %d.", number)}
	}
	if t.Format == "" {
		return nil, nil, damaged(unreadMessage(t.CodecID), number, t.CodecID)
	}
	privateBudget := f.inflateCap
	private, err := f.decode(t, t.codecPrivate, scopeCodecPriv, &privateBudget)
	if err != nil {
		return nil, nil, err
	}
	blocks, err := f.readBlocks(t)
	if err != nil {
		return nil, nil, err
	}
	var s *model.Subtitle
	var warnings []model.ParseWarning
	switch t.Format { //nolint:exhaustive // the other formats are text
	case model.PGS:
		s, warnings, err = f.readPGS(t, blocks, common)
	case model.VobSub:
		s, warnings, err = f.readVobSub(t, private, blocks, common)
	default:
		s, warnings, err = parseRebuilt(t, string(private), f.timedBlocks(blocks, t, common), common)
	}
	if err != nil {
		return nil, nil, err
	}
	s.SetMetadata(model.MetadataLanguage, t.Language)
	if t.Forced {
		err = markForced(s)
	}
	return s, warnings, err
}

// parseRebuilt rebuilds the file of the track from its blocks and reads it with the parser of its format.
func parseRebuilt(t *trackEntry, private string, blocks []timedBlock, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if t.Format == model.SubRip {
		return srt.Read(strings.NewReader(subRipFile(blocks)), common, srt.ReadOptions{})
	}
	if t.Format == model.ASS {
		return ass.Read(strings.NewReader(assFile(t.CodecID == "S_TEXT/SSA", private, blocks)), common, ass.ReadOptions{})
	}
	return webvtt.Read(strings.NewReader(webVTTFile(strings.HasPrefix(t.CodecID, "D_WEBVTT/"), private, blocks)), common, webvtt.ReadOptions{})
}

func markForced(s *model.Subtitle) error {
	for i, c := range s.Cues() {
		c.Forced = true
		if err := s.SetCue(i, c); err != nil {
			return err
		}
	}
	return nil
}

// timedBlocks converts the block times to milliseconds and sets the end of each block. A block without BlockDuration
// and DefaultDuration ends at the start of the next block. The last such block ends after LastCueDuration.
func (f *File) timedBlocks(blocks []block, t *trackEntry, common model.CommonReadOptions) []timedBlock {
	out := make([]timedBlock, len(blocks))
	for i, b := range blocks {
		startNs, _ := f.nanos(b.start)
		start := milliseconds(startNs)
		out[i] = timedBlock{startMs: start, endMs: f.endMs(blocks, i, t, common), data: string(b.data), additional: string(b.additional)}
	}
	return out
}

func (f *File) endMs(blocks []block, i int, t *trackEntry, common model.CommonReadOptions) int64 {
	b := blocks[i]
	startNs, _ := f.nanos(b.start)
	switch {
	case b.duration != noDuration:
		d, _ := f.nanos(b.duration)
		return milliseconds(startNs + d)
	case t.defaultDuration != noDuration:
		return milliseconds(startNs + t.defaultDuration)
	case i+1 < len(blocks):
		next, _ := f.nanos(blocks[i+1].start)
		return milliseconds(next)
	}
	start := time.Duration(milliseconds(startNs)) * time.Millisecond
	return int64(parsing.EndAfter(start, parsing.NewState(t.Format, common).LastCueDuration()) / time.Millisecond)
}

func milliseconds(ns int64) int64 {
	return (ns + 500_000) / 1_000_000
}

// unreadMessage returns the message for a codec without a reader. The DVB reader comes in a later release.
func unreadMessage(codecID string) string {
	if codecID == "S_DVBSUB" {
		return laterCodecMessage
	}
	return unreadCodecMessage
}
