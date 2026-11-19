package mpegts

import (
	"cmp"

	"github.com/asticode/go-astits"

	"github.com/yama6a/subtitle-toolbox-go/v5/container/mpegts/internal/teletext"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

const (
	subtitlePage        = 0x02
	hearingImpairedPage = 0x05
	maxPage             = 99
)

// addTeletext lists the teletext subtitle pages of the elementary streams of one program.
func (f *File) addTeletext(streams []*astits.PMTElementaryStream) {
	pids := elementaryPIDs(streams)
	for _, es := range streams {
		if es.StreamType != astits.StreamTypePrivateData {
			continue
		}
		for _, desc := range es.ElementaryStreamDescriptors {
			switch {
			case desc.Teletext != nil:
				f.addPages(es.ElementaryPID, pids, desc.Length, desc.Teletext)
			case desc.VBITeletext != nil:
				f.addPages(es.ElementaryPID, pids, desc.Length, desc.VBITeletext)
			}
		}
	}
}

// addPages adds the pages of a teletext descriptor. go-astits reads a short last entry from the bytes after it,
// so only the first length/5 items count. It turns the page byte into a decimal number, so a page byte with a low
// nibble above 9 reads as another page.
func (f *File) addPages(pid uint16, programPIDs []uint16, length uint8, d *astits.DescriptorTeletext) {
	for _, item := range d.Items[:min(len(d.Items), int(length)/5)] {
		if (item.Type != subtitlePage && item.Type != hearingImpairedPage) || item.Page > maxPage {
			continue
		}
		magazine := int(item.Magazine)
		number := int(item.Page) + 100*cmp.Or(magazine, 8)
		if f.hasNumber(number) {
			number += int(pid) * 1000
			if f.hasNumber(number) {
				continue
			}
		}
		track := model.Track{
			Container: model.MPEGTS,
			Number:    number,
			CodecID:   codecTeletext,
			Format:    model.SubRip,
			Language:  language(item.Language),
		}
		if item.Type == hearingImpairedPage {
			track.Name = "hearing impaired"
		}
		f.tracks = append(f.tracks, track)
		f.streams = append(f.streams, stream{
			pid:         pid,
			page:        magazine<<8 | int(item.Page/10)<<4 | int(item.Page%10),
			programPIDs: programPIDs,
		})
	}
}

func (f *File) teletextCues(track model.Track, s stream, number int, common model.CommonReadOptions) ([]model.Cue, error) {
	dec := teletext.NewDecoder(s.page, number%1000, teletext.LanguageGroup(track.Language))
	zero, last, err := f.readPES(s, dec)
	if err != nil {
		return nil, err
	}
	pages, err := dec.Finish()
	if err != nil {
		return nil, &model.ParseError{Msg: err.Error(), Err: err}
	}
	states := make([]textState, len(pages))
	for i, p := range pages {
		states[i] = textState{start: p.Start, lines: p.Lines}
	}
	lastCue := parsing.LastCueDuration(common)
	return textCues(number, states, zero, last, lastCue)
}
