package mkv

import (
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/bitmap/dvdsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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
			if err := st.Fail(err, model.NoLine, i, nil); err != nil {
				return nil, nil, err
			}
			continue
		}
		if u.Image == nil {
			continue
		}
		if err := s.AddCue(u.Cue(b.start, b.end)); err != nil {
			return nil, nil, err
		}
	}
	return s, st.Warnings(), nil
}
