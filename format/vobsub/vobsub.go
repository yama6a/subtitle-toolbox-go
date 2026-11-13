// Package vobsub reads VobSub, the DVD bitmap subtitles of a text index (.idx) and an MPEG-2 program stream (.sub).
//
// The index holds the screen size, the colours and, for each track, the time and the file position of each subtitle.
// The program stream holds the subpicture units. Each unit becomes one image cue. The format has no writer.
package vobsub

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/dvdsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// ReadOptions holds the settings of the VobSub reader.
type ReadOptions struct {
	// IDX is the content of the .idx file as UTF-8. It is a string, so stbox.ReadOptions stays comparable. stbox.Load and stbox.LoadFS fill it from the file next to the .sub file.
	IDX string
	// Index is the index: value of the track to read. Nil reads the first track that matches Language.
	// A pointer, because 0 is a valid index.
	Index *int
	// Language is the id: value of the track to read, compared without case. The empty string matches every track.
	Language string
}

// Validate checks Index and Language. The error is a *model.InvalidArgumentError.
func (o ReadOptions) Validate() error {
	if o.Index != nil && *o.Index < 0 {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.VobSub.Index",
			Msg:    "The track index must be 0 or more, got " + strconv.Itoa(*o.Index) + ".",
		}
	}
	if o.Language != "" && strings.Trim(o.Language, markupcore.PHPTrimSet) == "" {
		return &model.InvalidArgumentError{Option: "ReadOptions.VobSub.Language", Msg: "The language must not be empty."}
	}
	return nil
}

// Read reads the .sub program stream r with the .idx content of opts. The cues are image cues at the bottom
// center. The language metadata is the id value of the track, also when it is empty.
// A broken subpicture unit fails a strict read. A lenient read skips it with a warning that has the position of
// its timestamp line in the track as Block. An .idx error fails both.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	if opts.IDX == "" {
		return nil, nil, &model.InvalidArgumentError{
			Option: "ReadOptions.VobSub.IDX",
			Msg:    "VobSub needs the .idx content. Set ReadOptions.VobSub.IDX.",
		}
	}
	sub, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.VobSub, err)
	}
	ix, err := dvdsub.ParseIndex([]byte(opts.IDX))
	if err != nil {
		return nil, nil, err
	}
	track, err := selectTrack(ix, opts)
	if err != nil {
		return nil, nil, err
	}

	st := parsing.NewState(model.VobSub, common)
	units, err := readUnits(st, sub, track, ix)
	if err != nil {
		return nil, nil, err
	}
	return subtitleOf(st, track, units), st.Warnings(), nil
}

// subtitleOf returns the image cues of units. A unit without an image adds no cue but still ends the cue before it.
// A cue never ends before it starts, which a stop delay below the start delay or a later unit with an earlier start would cause.
func subtitleOf(st *parsing.State, track dvdsub.Track, units []entryUnit) *model.Subtitle {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, track.ID)
	for i, eu := range units {
		if eu.unit.Image == nil {
			continue
		}
		end := clamp(eu.time + eu.unit.StopDelay)
		if !eu.unit.HasStop {
			end = st.LastCueEnd(eu.start)
			if i+1 < len(units) {
				end = min(end, units[i+1].start)
			}
		}
		s.AddCue(model.Cue{Start: eu.start, End: max(end, eu.start), Forced: eu.unit.Forced, Image: eu.unit.Image})
	}
	return s
}

// readUnits decodes the unit of each entry of track. A broken unit fails a strict read, and a lenient read skips it
// with a warning.
func readUnits(st *parsing.State, sub []byte, track dvdsub.Track, ix dvdsub.Index) ([]entryUnit, error) {
	units := make([]entryUnit, 0, len(track.Entries))
	for _, e := range track.Entries {
		u, err := readEntry(sub, e, track.Index, ix)
		if err != nil {
			perr, _ := errors.AsType[*model.ParseError](err)
			if err := st.Fail(perr, model.NoLine, e.Line, nil); err != nil {
				return nil, err
			}
			continue
		}
		units = append(units, entryUnit{start: clamp(e.Time + u.StartDelay), unit: u, time: e.Time})
	}
	return units, nil
}

type entryUnit struct {
	start time.Duration
	// time is the time of the index entry, from which the stop delay counts.
	time time.Duration
	unit dvdsub.Unit
}

// clamp keeps a time below the limit of 100000 hours, which a start delay of up to 745 s can pass.
func clamp(d time.Duration) time.Duration {
	return min(d, timecode.MaxValidTime)
}

func selectTrack(ix dvdsub.Index, opts ReadOptions) (dvdsub.Track, error) {
	for _, t := range ix.Tracks {
		if (opts.Index == nil || t.Index == *opts.Index) && (opts.Language == "" || strings.EqualFold(t.ID, opts.Language)) {
			return t, nil
		}
	}
	var wanted []string
	if opts.Index != nil {
		wanted = append(wanted, "index "+strconv.Itoa(*opts.Index))
	}
	if opts.Language != "" {
		wanted = append(wanted, `language "`+opts.Language+`"`)
	}
	if len(wanted) == 0 {
		return dvdsub.Track{}, &model.ParseError{Format: model.VobSub, Msg: `The .idx content has no "id:" line.`}
	}
	return dvdsub.Track{}, &model.ParseError{Format: model.VobSub, Msg: "The .idx content has no track with " + strings.Join(wanted, " and ") + "."}
}

// readEntry reads and decodes the unit at the file position of e. Every error is a *model.ParseError.
func readEntry(sub []byte, e dvdsub.Entry, track int, ix dvdsub.Index) (dvdsub.Unit, error) {
	if e.FilePos >= int64(len(sub)) {
		return dvdsub.Unit{}, &model.ParseError{Format: model.VobSub, Msg: fmt.Sprintf(
			"The filepos %09x of timestamp %.3f is outside the .sub content of %d bytes.", e.FilePos, e.Time.Seconds(), len(sub))}
	}
	data, err := dvdsub.ReadUnit(sub, e.FilePos, track)
	if err != nil {
		return dvdsub.Unit{}, err
	}
	return dvdsub.DecodeUnit(data, ix)
}
