package mkv

import (
	"errors"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap/dvdsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

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
