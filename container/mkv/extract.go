package mkv

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

const codecDVB = "S_DVBSUB"

// timedBlock is a block with its times rounded to whole milliseconds.
type timedBlock struct {
	start, end time.Duration
	data       string
	additional string
}

// ExtractOptions holds the settings of Extract. Matroska has none yet.
type ExtractOptions struct{}

// Extract reads all blocks of the subtitle track with the TrackNumber number.
func (f *File) Extract(number int, common model.CommonReadOptions, _ ExtractOptions) (*model.Subtitle, []model.ParseWarning, error) {
	s, warnings, err := f.extract(number, common)
	return s, warnings, wrap(err)
}

func (f *File) extract(number int, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t, ok := f.track(number)
	if !ok {
		return nil, nil, &model.InvalidArgumentError{Option: "ReadOptions.Track", Msg: fmt.Sprintf("the file has no subtitle track with the number %d", number)}
	}
	if t.Format == "" {
		return nil, nil, damaged("track %d has the codec %s. The reader extracts only %s", number, t.CodecID, readableCodecIDs())
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
	s, warnings, err := f.decodeTrack(t, private, blocks, common)
	if err != nil {
		return nil, nil, err
	}
	s.SetMetadata(model.MetadataLanguage, t.Language)
	if t.Forced {
		err = markForced(s)
	}
	return s, warnings, err
}

// decodeTrack reads the blocks of a track with the reader of its codec.
func (f *File) decodeTrack(t *trackEntry, private []byte, blocks []block, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	switch t.CodecID {
	case "S_HDMV/PGS":
		return f.readPGS(t, blocks, common)
	case "S_VOBSUB":
		return f.readVobSub(t, private, blocks, common)
	case codecDVB:
		return f.dvbSubtitle(private, blocks, common)
	}
	return parseRebuilt(t, string(private), f.timedBlocks(blocks, t, common), common)
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
		out[i] = timedBlock{start: roundToMillisecond(startNs), end: f.end(blocks, i, t, common), data: string(b.data), additional: string(b.additional)}
	}
	return out
}

func (f *File) end(blocks []block, i int, t *trackEntry, common model.CommonReadOptions) time.Duration {
	b := blocks[i]
	startNs, _ := f.nanos(b.start)
	switch {
	case b.duration != noDuration:
		d, _ := f.nanos(b.duration)
		return roundToMillisecond(startNs + d)
	case t.defaultDuration != noDuration:
		return roundToMillisecond(startNs + t.defaultDuration)
	case i+1 < len(blocks):
		next, _ := f.nanos(blocks[i+1].start)
		return roundToMillisecond(next)
	}
	return parsing.EndAfter(roundToMillisecond(startNs), parsing.LastCueDuration(common)).Truncate(time.Millisecond)
}

// roundToMillisecond rounds ns half up. Duration.Round would round a negative time away from zero.
func roundToMillisecond(ns int64) time.Duration {
	return time.Duration((ns+500_000)/1_000_000) * time.Millisecond
}
