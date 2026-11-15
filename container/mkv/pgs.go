package mkv

import (
	"encoding/binary"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap/displayset"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
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
			var size int
			if len(data) >= pgsSegmentHeader {
				size = pgsSegmentHeader + int(binary.BigEndian.Uint16(data[1:]))
			}
			if size == 0 || size > len(data) {
				msg := fmt.Sprintf("The block of track %d at byte %d holds a PGS segment that is cut off.", t.Number, b.offset)
				if err := d.Skip(&model.ParseError{Format: model.PGS, Msg: msg}); err != nil {
					return nil, nil, err
				}
				cutOff = true
				break read
			}
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
