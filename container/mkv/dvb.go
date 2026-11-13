package mkv

import (
	"encoding/binary"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/dvbsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// dvbSubtitle decodes the blocks of an S_DVBSUB track. CodecPrivate of 4 bytes or more holds the composition page and
// the ancillary page. Without them the decoder reads every page.
func (f *File) dvbSubtitle(t *trackEntry, private []byte, blocks []block, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	dec := dvbsub.NewDecoder(0, 0, true, common)
	if len(private) >= 4 {
		dec = dvbsub.NewDecoder(int(binary.BigEndian.Uint16(private)), int(binary.BigEndian.Uint16(private[2:])), false, common)
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
	s.AddCues(dec.Finish(parsing.NewState(t.Format, common).LastCueDuration())...)
	s.SetMetadata(model.MetadataLanguage, t.Language)
	if t.Forced {
		return s, dec.Warnings(), markForced(s)
	}
	return s, dec.Warnings(), nil
}
