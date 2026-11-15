package mkv

import (
	"encoding/binary"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/bitmap/dvbsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// dvbSubtitle decodes the blocks of an S_DVBSUB track. CodecPrivate of 4 bytes or more holds the composition page and
// the ancillary page. Without them the decoder reads every page.
func (f *File) dvbSubtitle(private []byte, blocks []block, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	dec := dvbsub.NewAllPagesDecoder(common)
	if len(private) >= 4 {
		dec = dvbsub.NewDecoder(int(binary.BigEndian.Uint16(private)), int(binary.BigEndian.Uint16(private[2:])), common)
	}
	for _, b := range blocks {
		start, _ := f.nanos(b.start)
		var end int64
		if b.duration != noDuration {
			d, _ := f.nanos(b.duration)
			end = start + d
		}
		if err := dec.Add(b.data, time.Duration(start), time.Duration(end), b.duration != noDuration); err != nil {
			return nil, nil, err
		}
	}
	s := &model.Subtitle{}
	if err := s.AddCues(dec.Finish(parsing.LastCueDuration(common))...); err != nil {
		return nil, nil, err
	}
	return s, dec.Warnings(), nil
}
