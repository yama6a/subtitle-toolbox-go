package mpegts

import (
	"regexp"
	"slices"

	"github.com/asticode/go-astits"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/arib"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

var languageCode = regexp.MustCompile(`^[a-z]{3}$`) //nolint:gochecknoglobals // compiled once

// addCaptionStreams adds the ARIB caption streams of one program. A PID that two programs share keeps its last
// program and its first place.
func addCaptionStreams(captions []stream, streams []*astits.PMTElementaryStream) []stream {
	pids := make([]uint16, len(streams))
	for i, es := range streams {
		pids[i] = es.ElementaryPID
	}
	for _, es := range streams {
		if es.StreamType != astits.StreamTypePrivateData || !isCaption(es.ElementaryStreamDescriptors) {
			continue
		}
		s := stream{pid: es.ElementaryPID, programPIDs: pids}
		if i := slices.IndexFunc(captions, func(c stream) bool { return c.pid == s.pid }); i >= 0 {
			captions[i] = s
		} else {
			captions = append(captions, s)
		}
	}
	return captions
}

// isCaption reports whether the descriptors name a caption stream of profile A. The first component tag and the
// first 2 bytes of the data component descriptors count.
func isCaption(descriptors []*astits.Descriptor) bool {
	tag, haveTag := 0, false
	var component []byte
	for _, d := range descriptors {
		switch {
		case d.StreamIdentifier != nil && !haveTag:
			tag, haveTag = int(d.StreamIdentifier.ComponentTag), true
		case d.Tag == dataComponentTag:
			component = append(component, d.UserDefined...)
		}
	}
	return tag >= firstCaptionTag && tag <= lastCaptionTag && len(component) >= 2 &&
		int(component[0])<<8|int(component[1]) == captionDataComponent
}

// addARIB lists the caption streams as tracks. The language comes from the first caption management data of the
// PID within the first 32 MiB.
func (f *File) addARIB(captions []stream) error {
	if len(captions) == 0 {
		return nil
	}
	languages, err := f.captionLanguages(captions)
	if err != nil {
		return err
	}
	for _, s := range captions {
		number := int(s.pid)
		if f.hasNumber(number) {
			number *= 1000
		}
		f.tracks = append(f.tracks, model.Track{
			Container: model.MPEGTS,
			Number:    number,
			CodecID:   codecARIB,
			Format:    model.SubRip,
			Language:  languages[s.pid],
		})
		f.streams = append(f.streams, s)
	}
	return nil
}

// captionLanguages returns the language of each caption PID, "und" when none is found.
func (f *File) captionLanguages(captions []stream) (map[uint16]string, error) {
	wanted := map[uint16]bool{}
	languages := map[uint16]string{}
	for _, s := range captions {
		wanted[s.pid] = true
		languages[s.pid] = "und"
	}
	remaining := len(wanted)
	a := newAligner(f.packetSize, wanted, tableScanBytes)
	err := f.pass(a, func(d *astits.DemuxerData) bool {
		if d.PES == nil || !wanted[d.PID] {
			return true
		}
		if code := arib.Language(d.PES.Data); languageCode.MatchString(code) {
			languages[d.PID] = code
			delete(wanted, d.PID)
			remaining--
		}
		return remaining > 0
	})
	return languages, err
}
