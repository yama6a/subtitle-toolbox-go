package srt

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Read reads SubRip text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, content, ok, err := parsing.ReadText(r, model.SubRip, common, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := newBlockReader(st)
	var cues []model.Cue
	index := 0
	for b := range rd.Blocks(parsing.Lines(string(content))) {
		parsed, err := rd.ParseBlock(b, index)
		if err != nil {
			return nil, nil, err
		}
		cues = append(cues, parsed...)
		index++
	}
	if err := s.AddCues(cues...); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}
