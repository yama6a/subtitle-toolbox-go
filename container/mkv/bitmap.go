package mkv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/displayset"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/dvdsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	pgsSegmentHeader = 3
	ptsTicks         = 9
	ptsNanoseconds   = 100_000
)

// readPGS passes the segments of each block to the PGS decoder with the 90 kHz time of the block. Block durations do
// not count. A segment that the block end cuts off fails a strict read. A lenient read warns and ends the track.
func (f *File) readPGS(t *trackEntry, blocks []block, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	d := displayset.NewDecoder(common)
	cutOff := false
read:
	for _, b := range blocks {
		ns, _ := f.nanos(b.start)
		pts := ns * ptsTicks / ptsNanoseconds
		for data := b.data; len(data) > 0; {
			if len(data) < pgsSegmentHeader || len(data)-pgsSegmentHeader < int(binary.BigEndian.Uint16(data[1:])) {
				msg := fmt.Sprintf("The block of track %d at byte %d holds a PGS segment that is cut off.", t.Number, b.offset)
				if err := d.Skip(&model.ParseError{Format: model.PGS, Msg: msg}); err != nil {
					return nil, nil, err
				}
				cutOff = true
				break read
			}
			size := pgsSegmentHeader + int(binary.BigEndian.Uint16(data[1:]))
			if err := d.Add(pts, data[0], data[pgsSegmentHeader:size]); err != nil {
				return nil, nil, err
			}
			data = data[size:]
		}
	}
	cues, err := d.Finish(cutOff)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	parsing.AddParsed(s, cues, nil)
	return s, d.Warnings(), nil
}

// readVobSub decodes each block as one subpicture unit with the colours of the .idx header in CodecPrivate.
// A broken unit fails a strict read. A lenient read skips it with a warning that has the block index.
func (f *File) readVobSub(t *trackEntry, private []byte, blocks []block, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	ix, err := dvdsub.ParseHeader(private)
	if err != nil {
		return nil, nil, err
	}
	timed := f.timedBlocks(blocks, t, common)
	st := parsing.NewState(model.VobSub, common)
	s := &model.Subtitle{}
	for i, b := range timed {
		u, err := dvdsub.DecodeUnit([]byte(b.data), ix)
		if err != nil {
			perr, _ := errors.AsType[*model.ParseError](err)
			if err := st.Fail(perr, model.NoLine, i, nil); err != nil {
				return nil, nil, err
			}
			continue
		}
		if u.Image == nil {
			continue
		}
		start := min(time.Duration(b.startMs)*time.Millisecond+u.StartDelay, timecode.MaxValidTime)
		end := time.Duration(b.endMs) * time.Millisecond
		if u.HasStop {
			end = time.Duration(b.startMs)*time.Millisecond + u.StopDelay
		}
		s.AddCue(model.Cue{Start: start, End: min(max(end, start), timecode.MaxValidTime), Forced: u.Forced, Image: u.Image})
	}
	return s, st.Warnings(), nil
}
