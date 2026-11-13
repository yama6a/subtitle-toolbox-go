package mpegts

import (
	"slices"
	"time"

	"github.com/asticode/go-astits"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/dvbsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	subtitlingEntrySize = 8
	hearingImpairedFrom = 0x20
	hearingImpairedTo   = 0x25
)

// addDVB lists the DVB subtitle services of the elementary streams of one program. The entries of all subtitling
// descriptors of a stream join into one list. The first entry has the PID as its number, a later entry or a clash has
// PID * 1000 + index. When that number is taken too, the entry is skipped.
func (f *File) addDVB(streams []*astits.PMTElementaryStream) {
	pids := make([]uint16, len(streams))
	for i, es := range streams {
		pids[i] = es.ElementaryPID
	}
	for _, es := range streams {
		if es.StreamType != astits.StreamTypePrivateData {
			continue
		}
		for i, item := range subtitlingItems(es) {
			number := int(es.ElementaryPID)*1000 + i
			if i == 0 && !f.hasNumber(int(es.ElementaryPID)) {
				number = int(es.ElementaryPID)
			}
			if f.hasNumber(number) {
				continue
			}
			track := model.Track{Container: model.MPEGTS, Number: number, CodecID: codecDVB, Format: model.PGS, Language: language(item.Language)}
			if item.Type >= hearingImpairedFrom && item.Type <= hearingImpairedTo {
				track.Name = "hearing impaired"
			}
			f.tracks = append(f.tracks, track)
			f.streams = append(f.streams, stream{
				pid: es.ElementaryPID, page: int(item.CompositionPageID), ancillary: int(item.AncillaryPageID), programPIDs: pids,
			})
		}
	}
}

func subtitlingItems(es *astits.PMTElementaryStream) []*astits.DescriptorSubtitlingItem {
	var items []*astits.DescriptorSubtitlingItem
	for _, desc := range es.ElementaryStreamDescriptors {
		if desc.Subtitling != nil {
			items = append(items, desc.Subtitling.Items[:min(len(desc.Subtitling.Items), int(desc.Length)/subtitlingEntrySize)]...)
		}
	}
	return items
}

// pesPacket is the data of one PES packet and its unwrapped PTS.
type pesPacket struct {
	data []byte
	pts  int64
}

// packetBuffer keeps the PES packets of a stream until the zero point is known.
type packetBuffer []pesPacket

func (b *packetBuffer) Add(data []byte, pts int64) {
	*b = append(*b, pesPacket{slices.Clone(data), pts})
}

// ticksToDuration returns ticks of the 90 kHz clock as a duration. The caller keeps ticks below the time limit.
// A negative time stays negative, and the decoder turns it into 0 when it builds the cues.
func ticksToDuration(ticks int64) time.Duration {
	return time.Duration(ticks * 100_000 / 9)
}

func (f *File) dvbCues(s stream, number int, common model.CommonReadOptions) ([]model.Cue, []model.ParseWarning, error) {
	var packets packetBuffer
	zero, _, err := f.readPES(s, &packets)
	if err != nil {
		return nil, nil, err
	}
	dec := dvbsub.NewDecoder(s.page, s.ancillary, false, common)
	for _, p := range packets {
		if _, err := ticksToTime(number, p.pts-zero); err != nil {
			return nil, nil, err
		}
		if err := dec.Add(p.data, ticksToDuration(p.pts-zero), 0, false); err != nil {
			return nil, nil, err
		}
	}
	return dec.Finish(parsing.NewState(model.PGS, common).LastCueDuration()), dec.Warnings(), nil
}
