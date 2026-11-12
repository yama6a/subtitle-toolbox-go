package mp4

import (
	"encoding/hex"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Eyevinn/mp4ff/mp4"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	codecTx3g = "tx3g"
	codecC608 = "c608"
	codecC708 = "c708"

	trackEnabled       = 0x000001
	allSamplesForced   = 0x80000000
	undeterminedPacked = 0x7FFF
	emptyEdit          = -1
)

// track is a listed subtitle track with the data that Extract needs.
type track struct {
	info      model.Track
	encrypted bool
	timescale uint32
	delay     *big.Rat // seconds that the edit list adds to every sample time
	stbl      *mp4.StblBox
}

// newTrack returns the track of trak, or nil when it is no subtitle track.
func newTrack(trak *mp4.TrakBox, movieTimescale uint32) (*track, error) {
	tkhd := first[*mp4.TkhdBox](trak.Children)
	mdia := first[*mp4.MdiaBox](trak.Children)
	if tkhd == nil || mdia == nil {
		return nil, nil //nolint:nilnil // a track that is no subtitle track is skipped
	}
	hdlr := first[*mp4.HdlrBox](mdia.Children)
	mdhd := first[*mp4.MdhdBox](mdia.Children)
	if hdlr == nil || mdhd == nil || !slices.Contains([]string{"text", "sbtl", "subt", "clcp"}, hdlr.HandlerType) {
		return nil, nil //nolint:nilnil // a track that is no subtitle track is skipped
	}
	number := int(tkhd.TrackID)
	if mdhd.Timescale == 0 {
		return nil, parseError("Track " + strconv.Itoa(number) + " has a timescale of 0.")
	}

	info := model.Track{
		Container: model.MP4,
		Number:    number,
		Language:  language(mdia, mdhd),
		Name:      trackName(trak),
		Default:   tkhd.Flags&trackEnabled != 0,
	}
	t := &track{timescale: mdhd.Timescale, delay: editListDelay(trak, movieTimescale, mdhd.Timescale)}
	if minf := first[*mp4.MinfBox](mdia.Children); minf != nil {
		t.stbl = first[*mp4.StblBox](minf.Children)
	}
	t.readEntry(&info)
	t.info = info
	return t, nil
}

// readEntry sets the codec, the format and the forced flag from the first sample entry.
func (t *track) readEntry(info *model.Track) {
	if t.stbl == nil {
		return
	}
	if stsd := first[*mp4.StsdBox](t.stbl.Children); stsd != nil && len(stsd.Children) > 0 {
		entry := stsd.Children[0]
		info.CodecID = codecName(entry.Type())
		t.encrypted = isEncrypted(entry)
		if tx3g, ok := entry.(*mp4.TextSampleEntryBox); ok {
			info.Forced = tx3g.DisplayFlags&allSamplesForced != 0
		}
	}
	if !t.encrypted {
		info.Format = formatOf(info.CodecID)
	}
}

func formatOf(codec string) model.Format {
	switch codec {
	case codecTx3g:
		return model.SubRip
	case codecC608:
		return model.SCC
	case codecC708:
		return model.MCC
	default:
		return ""
	}
}

// codecName returns a printable type as it is, and other bytes as 0x and 8 hex digits.
func codecName(t string) string {
	if printable([]byte(t)) {
		return t
	}
	return "0x" + hex.EncodeToString([]byte(t))
}

func isEncrypted(entry mp4.Box) bool {
	switch entry.Type() {
	case "encv", "enca", "enct", "encs":
		return true
	}
	tx3g, ok := entry.(*mp4.TextSampleEntryBox)
	return ok && first[*mp4.SinfBox](tx3g.Children) != nil
}

// language returns the elng string, else the language of the mdhd box.
func language(mdia *mp4.MdiaBox, mdhd *mp4.MdhdBox) string {
	if elng := first[*mp4.ElngBox](mdia.Children); elng != nil {
		if l := cleanString(elng.Language); l != "" {
			return l
		}
	}
	p := mdhd.Language
	if p == 0 || p == undeterminedPacked {
		return "und"
	}
	return string([]byte{byte(p>>10&0x1F) + 0x60, byte(p>>5&0x1F) + 0x60, byte(p&0x1F) + 0x60})
}

// trackName returns the text of the first name box in the user data of the track.
func trackName(trak *mp4.TrakBox) string {
	udta := first[*mp4.UdtaBox](trak.Children)
	if udta == nil {
		return ""
	}
	for _, c := range udta.Children {
		switch b := c.(type) {
		case *mp4.NameBox:
			return cleanString(b.Name)
		case *mp4.UserDataTextBox:
			if c.Type() == "name" && len(b.Texts) > 0 {
				return cleanString(b.Texts[0].Text)
			}
		}
	}
	return ""
}

// cleanString cuts s at the first NUL byte and returns "" for text that is not valid UTF-8.
func cleanString(s string) string {
	s, _, _ = strings.Cut(s, "\x00")
	if !utf8.ValidString(s) {
		return ""
	}
	return s
}

// editListDelay returns the seconds by which the edit list moves the samples: the empty edits at the start,
// minus the media time of the first edit after them. Later edits do not change the times.
func editListDelay(trak *mp4.TrakBox, movieTimescale, mediaTimescale uint32) *big.Rat {
	delay := new(big.Rat)
	edts := first[*mp4.EdtsBox](trak.Children)
	if edts == nil {
		return delay
	}
	elst := first[*mp4.ElstBox](edts.Children)
	if elst == nil {
		return delay
	}
	var empty big.Int
	for _, e := range elst.Entries {
		if e.MediaTime != emptyEdit {
			if movieTimescale != 0 {
				delay.SetFrac(&empty, big.NewInt(int64(movieTimescale)))
			}
			return delay.Sub(delay, big.NewRat(e.MediaTime, int64(mediaTimescale)))
		}
		empty.Add(&empty, new(big.Int).SetUint64(e.SegmentDuration))
	}
	return delay
}
