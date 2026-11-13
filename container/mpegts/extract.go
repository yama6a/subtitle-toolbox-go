package mpegts

import (
	"fmt"
	"maps"
	"slices"

	"github.com/asticode/go-astits"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/arib"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/teletext"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const ptsRange = 1 << 33

// pesSink gets the data of each PES packet of a track. The teletext and ARIB decoders are sinks.
type pesSink interface{ Add(data []byte, pts int64) }

// Extract reads the track that has the given number from the start of the file. Lenient mode has no effect. The warnings are those of the ARIB decoder.
func (f *File) Extract(number int, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	i := slices.IndexFunc(f.tracks, func(t model.Track) bool { return t.Number == number })
	if i < 0 {
		return nil, nil, &model.InvalidArgumentError{
			Option: "ReadOptions.Track",
			Msg:    fmt.Sprintf("The file has no subtitle track with the number %d.", number),
		}
	}
	track, s := f.tracks[i], f.streams[i]
	var (
		states   []textState
		warnings []model.ParseWarning
		zero     int64
		last     int64
		err      error
	)
	if track.CodecID == codecARIB {
		states, warnings, zero, last, err = f.readARIB(track, s)
	} else {
		states, zero, last, err = f.readTeletext(track, s)
	}
	if err != nil {
		return nil, nil, err
	}
	lastCue := parsing.NewState(model.SubRip, common).LastCueDuration()
	cues, err := textCues(number, states, zero, last, lastCue)
	if err != nil {
		return nil, nil, err
	}
	sub := &model.Subtitle{}
	sub.AddCues(cues...)
	sub.SetMetadata(model.MetadataLanguage, track.Language)
	return sub, warnings, nil
}

func (f *File) readTeletext(track model.Track, s stream) (states []textState, zero, last int64, err error) {
	dec := teletext.NewDecoder(s.page, track.Number%1000, teletext.LanguageGroup(track.Language))
	zero, last, err = f.readPES(s, dec)
	if err != nil {
		return nil, 0, 0, err
	}
	pages, err := dec.Finish()
	if err != nil {
		return nil, 0, 0, &model.ParseError{Msg: err.Error(), Err: err}
	}
	for _, p := range pages {
		states = append(states, textState{start: p.Start, lines: p.Lines})
	}
	return states, zero, last, nil
}

func (f *File) readARIB(track model.Track, s stream) (states []textState, warnings []model.ParseWarning, zero, last int64, err error) {
	dec := arib.NewDecoder(track.Language)
	zero, last, err = f.readPES(s, dec)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	for _, st := range dec.Finish() {
		states = append(states, textState{start: st.Start, lines: st.Lines, alignment: st.Alignment})
	}
	for _, w := range dec.Warnings() {
		ms := (max(0, w.Time-zero) + 45) / 90
		warnings = append(warnings, model.ParseWarning{
			Message: fmt.Sprintf("At %d.%03d s: %s", ms/1000, ms%1000, w.Message),
			Line:    model.NoLine,
			Block:   model.NoBlock,
			Action:  model.Repaired,
		})
	}
	return states, warnings, zero, last, nil
}

// readPES passes each PES of s.pid to sink. It returns the zero point, which is the smallest first PTS of the
// streams of the program, and the last PTS.
func (f *File) readPES(s stream, sink pesSink) (zero, last int64, err error) {
	a := newAligner(f.packetSize, map[uint16]bool{s.pid: true}, 0)
	a.others = map[uint16]bool{}
	for _, pid := range s.programPIDs {
		a.others[pid] = pid != s.pid
	}
	u := &unwrapper{first: map[uint16]int64{}}
	err = f.pass(a, func(d *astits.DemuxerData) bool {
		u.applyEvents(a)
		if d.PES != nil && d.PID == s.pid {
			pts, has := pesPTS(d.PES)
			if unwrapped, ok := u.apply(s.pid, pts, has); ok {
				sink.Add(d.PES.Data, unwrapped)
			}
		}
		return true
	})
	if err != nil {
		return 0, 0, err
	}
	u.applyEvents(a)
	zero, last = u.result()
	return zero, last, nil
}

// pesPTS returns the 33-bit PTS of a PES packet, when it has one.
func pesPTS(p *astits.PESData) (int64, bool) {
	if h := p.Header; h != nil && h.OptionalHeader != nil && h.OptionalHeader.PTS != nil {
		return h.OptionalHeader.PTS.Base, true
	}
	return 0, false
}

// unwrapper counts the 33-bit PTS across the wrap and keeps the first PTS of each stream.
type unwrapper struct {
	last     int64
	haveLast bool
	first    map[uint16]int64
}

// result returns the smallest first PTS and the last PTS. Without a PTS both are 0.
func (u *unwrapper) result() (zero, last int64) {
	if len(u.first) > 0 {
		zero = slices.Min(slices.Collect(maps.Values(u.first)))
	}
	if !u.haveLast {
		return zero, zero
	}
	return zero, u.last
}

// applyEvents applies the PTS events that the aligner found since the last call, in file order.
func (u *unwrapper) applyEvents(a *aligner) {
	for _, e := range a.events {
		u.apply(e.pid, e.pts, true)
	}
	a.events = a.events[:0]
}

// apply unwraps pts of the stream pid. A PES without a PTS takes the last PTS, and without one it is skipped.
func (u *unwrapper) apply(pid uint16, pts int64, has bool) (int64, bool) {
	switch {
	case has && u.haveLast:
		pts += roundHalfAway(u.last-pts, ptsRange) * ptsRange
	case !has && u.haveLast:
		pts = u.last
	case !has:
		return 0, false
	}
	u.last, u.haveLast = pts, true
	if _, ok := u.first[pid]; !ok {
		u.first[pid] = pts
	}
	return pts, true
}

// roundHalfAway returns x/y rounded half away from zero, for y above 0.
func roundHalfAway(x, y int64) int64 {
	if x < 0 {
		return -((-x + y/2) / y)
	}
	return (x + y/2) / y
}
