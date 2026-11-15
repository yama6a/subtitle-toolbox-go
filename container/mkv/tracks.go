package mkv

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv/internal/ebml"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	trackTypeSubtitle     = 0x11
	defaultTimestampScale = 1_000_000
	defaultLanguage       = "eng"
	mkvmergeWritingApp    = "mkvmerge "
	noPosition            = -1
)

type encoding struct {
	scope, kind, algo uint64
	settings          []byte
}

type trackEntry struct {
	model.Track
	uid             string
	codecPrivate    []byte
	defaultDuration int64 // ns, noDuration without one
	encodings       []encoding
}

func (f *File) readTracks(tracks ebml.Header) error {
	return f.each(tracks, func(entry ebml.Header) error {
		if entry.ID != idTrackEntry {
			return nil
		}
		t, ok, err := f.readTrackEntry(entry)
		if err != nil || !ok {
			return err
		}
		if i := slices.IndexFunc(f.tracks, func(o trackEntry) bool { return o.Number == t.Number }); i >= 0 {
			f.tracks[i] = t
		} else {
			f.tracks = append(f.tracks, t)
		}
		return nil
	})
}

// entryReader reads the children of a TrackEntry.
type entryReader struct {
	f         *File
	t         trackEntry
	number    uint64
	kind      uint64
	hasNumber bool
	language  string
	bcp47     string
	hasLang   bool
	hasBCP47  bool
}

func (f *File) readTrackEntry(entry ebml.Header) (trackEntry, bool, error) {
	e := entryReader{f: f, t: trackEntry{defaultDuration: noDuration}}
	e.t.Container = model.Matroska
	e.t.Default = true
	if err := f.each(entry, e.child); err != nil {
		return trackEntry{}, false, err
	}
	if e.kind != trackTypeSubtitle || !e.hasNumber || e.number > math.MaxInt32 {
		return trackEntry{}, false, nil
	}
	e.t.Number = int(e.number)
	e.t.Format = codecFormat(e.t.CodecID)
	switch {
	case e.hasBCP47:
		e.t.Language = e.bcp47
	case e.hasLang:
		e.t.Language = e.language
	default:
		e.t.Language = defaultLanguage
	}
	return e.t, true, nil
}

func (e *entryReader) child(c ebml.Header) error {
	switch c.ID {
	case idTrackNumber:
		e.hasNumber = true
		return e.f.readUint(c, &e.number)
	case idTrackType:
		return e.f.readUint(c, &e.kind)
	case idTrackUID:
		var b []byte
		err := e.f.readBytes(c, &b)
		e.t.uid = strings.TrimLeft(string(b), "\x00")
		return err
	case idCodecID:
		return e.f.readString(c, &e.t.CodecID)
	case idCodecPrivate:
		return e.f.readBytes(c, &e.t.codecPrivate)
	case idLanguage:
		e.hasLang = true
		return e.f.readString(c, &e.language)
	case idLanguageBCP47:
		e.hasBCP47 = true
		return e.f.readString(c, &e.bcp47)
	case idName:
		return e.f.readString(c, &e.t.Name)
	}
	return e.flags(c)
}

func (e *entryReader) flags(c ebml.Header) error {
	var v uint64
	switch c.ID {
	case idFlagDefault:
		err := e.f.readUint(c, &v)
		e.t.Default = v != 0
		return err
	case idFlagForced:
		err := e.f.readUint(c, &v)
		e.t.Forced = v != 0
		return err
	case idDefaultDuration:
		err := e.f.readUint(c, &v)
		e.t.defaultDuration = int64(v)
		return err
	case idEncodings:
		var err error
		e.t.encodings, err = e.f.readEncodings(c)
		return err
	}
	return nil
}

// codecFormat returns the format that Extract reads from a codec, "" for a codec it cannot read.
func codecFormat(codecID string) model.Format {
	switch codecID {
	case "S_TEXT/UTF8":
		return model.SubRip
	case "S_TEXT/ASS", "S_TEXT/SSA":
		return model.ASS
	case "S_TEXT/WEBVTT", "D_WEBVTT/SUBTITLES", "D_WEBVTT/CAPTIONS", "D_WEBVTT/DESCRIPTIONS":
		return model.WebVTT
	case "S_HDMV/PGS":
		return model.PGS
	case "S_VOBSUB":
		return model.VobSub
	case codecDVB:
		return model.PGS
	default:
		return ""
	}
}

// readEncodings returns the content encodings in decoding order, the highest order first.
func (f *File) readEncodings(encodings ebml.Header) ([]encoding, error) {
	type ordered struct {
		order uint64
		encoding
	}
	var list []ordered
	err := f.each(encodings, func(e ebml.Header) error {
		if e.ID != idEncoding {
			return nil
		}
		o := ordered{encoding: encoding{scope: 1}}
		err := f.each(e, func(c ebml.Header) error { return f.readEncodingField(c, &o.order, &o.encoding) })
		list = append(list, o)
		return err
	})
	slices.SortStableFunc(list, func(a, b ordered) int { return cmp.Compare(b.order, a.order) })
	out := make([]encoding, len(list))
	for i, o := range list {
		out[i] = o.encoding
	}
	return out, err
}

func (f *File) readEncodingField(c ebml.Header, order *uint64, e *encoding) error {
	switch c.ID {
	case idEncodingOrder:
		return f.readUint(c, order)
	case idEncodingScope:
		return f.readUint(c, &e.scope)
	case idEncodingType:
		return f.readUint(c, &e.kind)
	case idCompression:
		return f.each(c, func(c ebml.Header) error {
			switch c.ID {
			case idCompAlgo:
				return f.readUint(c, &e.algo)
			case idCompSettings:
				return f.readBytes(c, &e.settings)
			}
			return nil
		})
	}
	return nil
}
