package webvtt

import (
	"fmt"
	"io"
	"sort"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Read reads WebVTT text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.WebVTT, err)
	}
	st := parsing.NewState(model.WebVTT, common)
	st.ReplaceNUL = true
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	s, err := read(st, string(content))
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func read(st *parsing.State, content string) (*model.Subtitle, error) {
	rd := vtt.NewReader(st, true)
	var (
		rolling vtt.Rolling
		cues    []model.Cue
		kept    []int
		out     []vtt.Kept
	)
	keep := func() {
		for _, k := range out {
			kept = append(kept, k.Index)
			cues = append(cues, k.Cue)
		}
		out = out[:0]
	}
	index := 0
	for b, err := range rd.Blocks(parsing.Lines(content)) {
		if err != nil {
			return nil, err
		}
		c, ok, err := rd.ParseBlock(b, index)
		if err != nil {
			return nil, err
		}
		index++
		if ok {
			out = rolling.Push(c, out)
			keep()
		}
	}
	out = rolling.Flush(out)
	keep()

	comments := rd.Comments()
	for i, c := range comments {
		comments[i].CuesBefore = sort.SearchInts(kept, c.CuesBefore)
	}
	s := &model.Subtitle{}
	parsing.AddParsed(s, cues, comments)
	if err := s.SetFormatData(string(model.WebVTT), rd.File()); err != nil {
		return nil, fmt.Errorf("%s file data: %w", model.WebVTT, err)
	}
	return s, nil
}
