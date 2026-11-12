package mp4

import (
	"encoding/binary"
	"strconv"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// c608Cues decodes the CEA-608 pairs of field 1 as an SCC file with one code line per sample.
func (f *File) c608Cues(w *walker, opts scc.ReadOptions, lastCueDuration time.Duration) ([]model.Cue, error) {
	var lines []cea608.CodeLine
	for {
		s, ok, err := w.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		boxes, err := f.sampleBoxes(w, s)
		if err != nil {
			return nil, err
		}
		var words []uint16
		for _, b := range boxes {
			cdat, ok := b.(*mp4.CdatBox)
			if !ok {
				continue
			}
			for i := 0; i+1 < len(cdat.Data); i += 2 {
				words = append(words, binary.BigEndian.Uint16(cdat.Data[i:]))
			}
		}
		if len(words) > 0 {
			lines = append(lines, cea608.CodeLine{Start: model.FrameRate2997.Frames(nanos(s.start)), Words: words})
		}
	}
	return caption.CEA608Cues(lines, caption.CEA608Options{Channel: opts.Channel, RollUp: opts.RollUp}, lastCueDuration), nil
}

// c708Cues reads each ccdp box as a caption distribution packet, as an MCC file has one packet per line.
func (f *File) c708Cues(w *walker, opts mcc.ReadOptions, lastCueDuration time.Duration) ([]model.Cue, error) {
	var frames []cea708.Frame
	for {
		s, ok, err := w.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		boxes, err := f.sampleBoxes(w, s)
		if err != nil {
			return nil, err
		}
		for _, b := range boxes {
			ccdp, ok := b.(*mp4.UnknownBox)
			if !ok || ccdp.Type() != "ccdp" {
				continue
			}
			_, triplets, err := cea708.ReadCDP(ccdp.Payload())
			if err != nil {
				return nil, parseError("The sample of track " + strconv.Itoa(w.t.info.Number) + " at byte " +
					strconv.FormatUint(s.offset, 10) + " holds a ccdp box that is not valid. " + err.Error())
			}
			frames = append(frames, cea708.Frame{Time: nanos(s.start), Triplets: triplets})
		}
	}
	return caption.CaptionDataCues(frames, caption.CEA708Service(opts.Service, opts.CEA608Only),
		caption.CEA608Options{Channel: opts.Channel, RollUp: opts.RollUp}, lastCueDuration), nil
}

// sampleBoxes reads a whole sample and decodes the boxes in it.
func (f *File) sampleBoxes(w *walker, s sample) ([]mp4.Box, error) {
	if err := w.outside(s); err != nil {
		return nil, err
	}
	data, err := readAt(f.r, s.offset, uint64(s.size))
	if err != nil {
		return nil, err
	}
	return decodeBoxes(data), nil
}

// decodeBoxes returns the boxes of a sample. A box that mp4ff cannot decode ends the list.
func decodeBoxes(data []byte) []mp4.Box {
	var boxes []mp4.Box
	for pos := 0; pos < len(data); {
		box, err := mp4.DecodeBoxSR(uint64(pos), bits.NewFixedSliceReader(data[pos:]))
		if err != nil || box.Size() < boxHeaderSize {
			return boxes
		}
		boxes = append(boxes, box)
		pos += int(box.Size())
	}
	return boxes
}
