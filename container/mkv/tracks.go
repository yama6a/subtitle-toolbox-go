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
	for entry, err := range f.r.Children(tracks) {
		if err != nil {
			return err
		}
		if entry.ID != idTrackEntry {
			continue
		}
		t, ok, err := f.readTrackEntry(entry)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if i := slices.IndexFunc(f.tracks, func(o trackEntry) bool { return o.Number == t.Number }); i >= 0 {
			f.tracks[i] = t
		} else {
			f.tracks = append(f.tracks, t)
		}
	}
	return nil
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
	for c, err := range f.r.Children(entry) {
		if err != nil {
			return trackEntry{}, false, err
		}
		if err := e.child(c); err != nil {
			return trackEntry{}, false, err
		}
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

func (e *entryReader) child(c ebml.Header) (err error) {
	r := e.f.r
	switch c.ID {
	case idTrackNumber:
		e.hasNumber = true
		e.number, err = r.Unsigned(c)
	case idTrackType:
		e.kind, err = r.Unsigned(c)
	case idTrackUID:
		var b []byte
		b, err = r.Bytes(c)
		e.t.uid = strings.TrimLeft(string(b), "\x00")
	case idCodecID:
		e.t.CodecID, err = r.String(c)
	case idCodecPrivate:
		e.t.codecPrivate, err = r.Bytes(c)
	case idLanguage:
		e.hasLang = true
		e.language, err = r.String(c)
	case idLanguageBCP47:
		e.hasBCP47 = true
		e.bcp47, err = r.String(c)
	case idName:
		e.t.Name, err = r.String(c)
	default:
		err = e.flags(c)
	}
	return err
}

func (e *entryReader) flags(c ebml.Header) (err error) {
	var v uint64
	switch c.ID {
	case idFlagDefault:
		v, err = e.f.r.Unsigned(c)
		e.t.Default = v != 0
	case idFlagForced:
		v, err = e.f.r.Unsigned(c)
		e.t.Forced = v != 0
	case idDefaultDuration:
		v, err = e.f.r.Unsigned(c)
		e.t.defaultDuration = int64(v)
	case idEncodings:
		e.t.encodings, err = e.f.readEncodings(c)
	}
	return err
}

// readableCodecs lists the codecs that Extract reads, in the order of the error message, with the format of each.
//
//nolint:gochecknoglobals // lookup table of the codecs
var readableCodecs = []struct {
	id     string
	format model.Format
}{
	{"S_TEXT/UTF8", model.SubRip},
	{"S_TEXT/ASS", model.ASS},
	{"S_TEXT/SSA", model.ASS},
	{"S_TEXT/WEBVTT", model.WebVTT},
	{"S_HDMV/PGS", model.PGS},
	{"S_VOBSUB", model.VobSub},
	{codecDVB, model.PGS},
	{"D_WEBVTT/SUBTITLES", model.WebVTT},
	{"D_WEBVTT/CAPTIONS", model.WebVTT},
	{"D_WEBVTT/DESCRIPTIONS", model.WebVTT},
}

// codecFormat returns the format that Extract reads from a codec, "" for a codec it cannot read.
func codecFormat(codecID string) model.Format {
	for _, c := range readableCodecs {
		if c.id == codecID {
			return c.format
		}
	}
	return ""
}

func readableCodecIDs() string {
	ids := make([]string, len(readableCodecs))
	for i, c := range readableCodecs {
		ids[i] = c.id
	}
	return strings.Join(ids, ", ")
}

// readEncodings returns the content encodings in decoding order, the highest order first.
func (f *File) readEncodings(encodings ebml.Header) ([]encoding, error) {
	type ordered struct {
		order uint64
		encoding
	}
	var list []ordered
	for e, err := range f.r.Children(encodings) {
		if err != nil {
			return nil, err
		}
		if e.ID != idEncoding {
			continue
		}
		o := ordered{encoding: encoding{scope: 1}}
		for c, err := range f.r.Children(e) {
			if err != nil {
				return nil, err
			}
			if err := f.readEncodingField(c, &o.order, &o.encoding); err != nil {
				return nil, err
			}
		}
		list = append(list, o)
	}
	slices.SortStableFunc(list, func(a, b ordered) int { return cmp.Compare(b.order, a.order) })
	out := make([]encoding, len(list))
	for i, o := range list {
		out[i] = o.encoding
	}
	return out, nil
}

func (f *File) readEncodingField(c ebml.Header, order *uint64, e *encoding) (err error) {
	switch c.ID {
	case idEncodingOrder:
		*order, err = f.r.Unsigned(c)
	case idEncodingScope:
		e.scope, err = f.r.Unsigned(c)
	case idEncodingType:
		e.kind, err = f.r.Unsigned(c)
	case idCompression:
		err = f.readCompression(c, e)
	}
	return err
}

func (f *File) readCompression(compression ebml.Header, e *encoding) error {
	for c, err := range f.r.Children(compression) {
		if err != nil {
			return err
		}
		switch c.ID {
		case idCompAlgo:
			if e.algo, err = f.r.Unsigned(c); err != nil {
				return err
			}
		case idCompSettings:
			if e.settings, err = f.r.Bytes(c); err != nil {
				return err
			}
		}
	}
	return nil
}
