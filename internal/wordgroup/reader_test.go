package wordgroup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func newReader(lenient bool, opts wordgroup.Options) (*wordgroup.Reader, *parsing.State) {
	st := parsing.NewState("test", model.CommonReadOptions{Lenient: lenient})
	return wordgroup.NewReader(st, opts, "test"), st
}

func readWord(item orderedjson.Value, path string) (wordgroup.Word, error) {
	text, err := wordgroup.Text(item, "w", path)
	if err != nil {
		return wordgroup.Word{}, err
	}
	v, _ := item.Get("s")
	start, err := wordgroup.Seconds(v, path+".s", time.Second)
	if err != nil {
		return wordgroup.Word{}, err
	}
	return wordgroup.Word{Text: text, Start: start, Timed: true, End: start + ms(100), HasEnd: true}, nil
}

func readWords(r *wordgroup.Reader) func([]orderedjson.Value, string) ([]wordgroup.Word, error) {
	return func(items []orderedjson.Value, path string) ([]wordgroup.Word, error) {
		return r.WordList(items, path, readWord)
	}
}

func TestReaderCue(t *testing.T) {
	t.Parallel()
	src := parseJSON(t, `{"x": 1}`)
	opts := wordgroup.Options{WordTimestamps: true, SpeakerVoices: true}
	t.Run("empty text gives no cue", func(t *testing.T) {
		t.Parallel()
		r, st := newReader(false, opts)
		c, err := r.Cue(0, ms(1), " \t", nil, "", nil, 0, src)
		require.NoError(t, err)
		assert.Nil(t, c)
		assert.Empty(t, st.Warnings())
	})
	t.Run("words, speaker and data", func(t *testing.T) {
		t.Parallel()
		r, _ := newReader(false, opts)
		c, err := r.Cue(0, ms(900), " a & b ", []wordgroup.Word{word("a", 0, 100), word("b", 500, 600)}, " Ann ",
			model.NewData(model.Field{Key: "k", Value: int64(1)}), 0, src)
		require.NoError(t, err)
		assert.Equal(t, []string{"a &amp; b"}, c.Lines)
		assert.Equal(t, "Ann", c.Speaker)
		assert.Equal(t, []model.Word{{Text: "a", End: ms(100)}, {Text: "b", Start: ms(500), End: ms(600)}}, c.Words)
		d, ok := c.FormatData("test")
		require.True(t, ok)
		assert.Equal(t, []string{"k"}, d.Keys())
	})
	t.Run("zero options", func(t *testing.T) {
		t.Parallel()
		r, _ := newReader(false, wordgroup.Options{})
		c, err := r.Cue(0, ms(900), "a", []wordgroup.Word{word("a", 0, 100)}, "Ann", nil, 0, src)
		require.NoError(t, err)
		assert.Empty(t, c.Words)
		assert.Empty(t, c.Speaker)
	})
	t.Run("swapped end", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			lenient bool
			warns   []model.WarningAction
			cue     bool
			err     bool
		}{
			{false, nil, false, true},
			{true, []model.WarningAction{model.Repaired}, true, false},
		}
		for _, tt := range tests {
			r, st := newReader(tt.lenient, opts)
			c, err := r.Cue(ms(2000), ms(1000), "a", nil, "", nil, 3, src)
			assert.Equal(t, tt.err, err != nil)
			assert.Equal(t, tt.cue, c != nil)
			var got []model.WarningAction
			for _, w := range st.Warnings() {
				got = append(got, w.Action)
				assert.Equal(t, 3, w.Block)
				assert.Equal(t, []string{`{"x":1}`}, w.Lines)
			}
			assert.Equal(t, tt.warns, got)
		}
	})
	t.Run("swap over 30 s is skipped", func(t *testing.T) {
		t.Parallel()
		r, st := newReader(true, opts)
		c, err := r.Cue(ms(40000), ms(1000), "a", nil, "", nil, 0, src)
		require.NoError(t, err)
		assert.Nil(t, c)
		require.Len(t, st.Warnings(), 1)
		assert.Equal(t, model.Skipped, st.Warnings()[0].Action)
	})
}

func TestReaderLists(t *testing.T) {
	t.Parallel()
	opts := wordgroup.Options{WordTimestamps: true}
	words := parseJSON(t, `[{"w": "a", "s": 1}, {"w": 5, "s": 2}, {"w": "b", "s": 3}]`).Items()
	t.Run("lenient skips a bad word", func(t *testing.T) {
		t.Parallel()
		r, st := newReader(true, opts)
		got, err := r.WordList(words, "words", readWord)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "b", got[1].Text)
		assert.Equal(t, []orderedjson.Value{words[2]}, got[1].Source)
		require.Len(t, st.Warnings(), 1)
		w := st.Warnings()[0]
		assert.Equal(t, "The field words[1].w must be a string.", w.Message)
		assert.Equal(t, model.Skipped, w.Action)
		assert.Equal(t, 1, w.Block)
		assert.Equal(t, []string{`{"w":5,"s":2}`}, w.Lines)
	})
	t.Run("strict returns the error", func(t *testing.T) {
		t.Parallel()
		r, _ := newReader(false, opts)
		_, err := r.WordList(words, "words", readWord)
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, model.Format("test"), perr.Format)
	})
	utterances := parseJSON(t, `[
		{"start": 0, "end": 1, "text": "a b", "speaker": 7, "words": [{"w": "a", "s": 0}, {"w": "b", "s": 0.5}]},
		{"start": 0, "end": "x", "text": "bad"},
		{"start": 2, "end": 3, "text": "  "},
		{"start": 1.5, "end": 2, "text": "c"}]`).Items()
	t.Run("utterances", func(t *testing.T) {
		t.Parallel()
		r, st := newReader(true, wordgroup.Options{WordTimestamps: true, SpeakerVoices: true})
		cues, err := r.Utterances(utterances, "u", "text", time.Second, readWords(r))
		require.NoError(t, err)
		require.Len(t, cues, 2)
		assert.Equal(t, "7", cues[0].Speaker)
		assert.Len(t, cues[0].Words, 2)
		d, _ := cues[0].FormatData("test")
		assert.Equal(t, []string{"start", "end", "text", "speaker", "words"}, d.Keys())
		require.Len(t, st.Warnings(), 1)
		assert.Equal(t, "The field u[1].end must be a time.", st.Warnings()[0].Message)
		assert.Equal(t, 1, st.Warnings()[0].Block)
	})
	t.Run("a strict word error comes out of an utterance", func(t *testing.T) {
		t.Parallel()
		r, _ := newReader(false, opts)
		bad := parseJSON(t, `[{"start": 0, "end": 1, "text": "a", "words": [{"w": "a", "s": "x"}]}]`).Items()
		_, err := r.Utterances(bad, "u", "text", time.Second, readWords(r))
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.Equal(t, "The field u[0].words[0].s must be a time.", perr.Msg)
	})
}

func TestReaderCuesFromWordsData(t *testing.T) {
	t.Parallel()
	src := func(s string) []orderedjson.Value { return []orderedjson.Value{parseJSON(t, s)} }
	a := word("a", 0, 100)
	a.Source = src(`{"id": 1}`)
	b := word("b", 200, 300)
	b.Source = src(`{"id": 2.50}`)
	tests := []struct {
		name  string
		extra *model.Data
		keys  []string
		list  bool
	}{
		{"words key last", model.NewData(model.Field{Key: "channel", Value: int64(0)}), []string{"channel", "words"}, true},
		{"extra wins", model.NewData(model.Field{Key: "words", Value: "mine"}, model.Field{Key: "channel", Value: int64(0)}), []string{"words", "channel"}, false},
		{"no extra", nil, []string{"words"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, _ := newReader(false, wordgroup.Options{})
			cues, err := r.CuesFromWords([]wordgroup.Word{a, b}, "words", tt.extra)
			require.NoError(t, err)
			require.Len(t, cues, 1)
			d, _ := cues[0].FormatData("test")
			assert.Equal(t, tt.keys, d.Keys())
			v, _ := d.Get("words")
			if tt.list {
				assert.Len(t, v, 2)
			} else {
				assert.Equal(t, "mine", v)
			}
		})
	}
}

func TestReaderTranscript(t *testing.T) {
	t.Parallel()
	r, _ := newReader(false, wordgroup.Options{})
	late, early := &model.Cue{Start: ms(2000), End: ms(3000), Lines: []string{"b"}}, &model.Cue{Start: ms(1000), End: ms(2000), Lines: []string{"a"}}
	s, err := r.Transcript([]*model.Cue{late, early}, parseJSON(t, `"en-US"`), model.NewData(model.Field{Key: "k", Value: true}))
	require.NoError(t, err)
	lang, ok := s.Metadata(model.MetadataLanguage)
	assert.True(t, ok)
	assert.Equal(t, "en-US", lang)
	assert.Equal(t, "a", s.Cues()[0].Text())
	_, ok = s.FormatData("test")
	assert.True(t, ok)

	s, err = r.Transcript(nil, parseJSON(t, `""`), nil)
	require.NoError(t, err)
	_, ok = s.Metadata(model.MetadataLanguage)
	assert.False(t, ok)
}
