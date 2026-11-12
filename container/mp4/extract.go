package mp4

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// ExtractOptions holds the format options that caption tracks use.
type ExtractOptions struct {
	// SCC holds the channel and the roll-up mode of c608 tracks.
	SCC scc.ReadOptions
	// MCC holds the service, CEA608Only, the channel and the roll-up mode of c708 tracks.
	MCC mcc.ReadOptions
}

// Extract reads all samples of the subtitle track with the track ID number.
// It supports the codecs tx3g, c608 and c708. A damaged file returns a *model.ParseError.
// A number that is no subtitle track returns a *model.InvalidArgumentError.
// The subtitle has the track language as its language metadata. The warnings are always nil.
func (f *File) Extract(number int, common model.CommonReadOptions, opts ExtractOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t := f.find(number)
	if t == nil {
		return nil, nil, &model.InvalidArgumentError{
			Option: "ReadOptions.Track",
			Msg:    "The file has no subtitle track with the number " + strconv.Itoa(number) + ".",
		}
	}
	if err := t.checkExtractable(opts); err != nil {
		return nil, nil, err
	}
	w, err := newWalker(t, f.size)
	if err != nil {
		return nil, nil, err
	}

	lastCueDuration := parsing.NewState(t.info.Format, common).LastCueDuration()
	var cues []model.Cue
	switch t.info.CodecID {
	case codecC608:
		cues, err = f.c608Cues(w, opts.SCC, lastCueDuration)
	case codecC708:
		cues, err = f.c708Cues(w, opts.MCC, lastCueDuration)
	default:
		cues, err = f.tx3gCues(w)
	}
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	s.SetMetadata(model.MetadataLanguage, t.info.Language)
	return s, nil, nil
}

// checkExtractable fails for an encrypted track, a codec without a reader, and bad format options.
func (t *track) checkExtractable(opts ExtractOptions) error {
	n := strconv.Itoa(t.info.Number)
	if t.encrypted {
		return parseError("Track " + n + " has the codec " + t.info.CodecID + " and is encrypted.")
	}
	if t.info.Format == "" {
		return parseError("Track " + n + " has the codec " + t.info.CodecID + ". The reader extracts only " +
			codecTx3g + ", " + codecC608 + ", " + codecC708 + ".")
	}
	switch t.info.CodecID {
	case codecC608:
		return opts.SCC.Validate()
	case codecC708:
		return opts.MCC.Validate()
	}
	return nil
}

func (f *File) find(number int) *track {
	for _, t := range f.tracks {
		if t.info.Number == number {
			return t
		}
	}
	return nil
}
