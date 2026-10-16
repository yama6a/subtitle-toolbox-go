package stats

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MostUsedWordsInJSON is the number of words that MarshalJSON writes.
const MostUsedWordsInJSON = 10

type jsonStatistics struct {
	CueCount            int         `json:"cueCount"`
	WordCount           int         `json:"wordCount"`
	CharacterCount      int         `json:"characterCount"`
	TotalDisplayTime    float64     `json:"totalDisplayTime"`
	Span                *float64    `json:"span"`
	CharactersPerSecond *Range      `json:"charactersPerSecond"`
	WordsPerMinute      *Range      `json:"wordsPerMinute"`
	CharactersPerLine   *Range      `json:"charactersPerLine"`
	Gaps                *Range      `json:"gaps"`
	MostUsedWords       []WordCount `json:"mostUsedWords"`
}

// MarshalJSON writes the shape of PHP SubtitleStatistics::toArray(): durations as seconds, a nil value as null
// and the first MostUsedWordsInJSON words.
func (st Statistics) MarshalJSON() ([]byte, error) {
	out := jsonStatistics{
		CueCount:            st.CueCount,
		WordCount:           st.WordCount,
		CharacterCount:      st.CharacterCount,
		TotalDisplayTime:    st.TotalDisplayTime.Seconds(),
		CharactersPerSecond: st.CharactersPerSecond,
		WordsPerMinute:      st.WordsPerMinute,
		CharactersPerLine:   st.CharactersPerLine,
		MostUsedWords:       make([]WordCount, min(len(st.MostUsedWords), MostUsedWordsInJSON)),
	}
	copy(out.MostUsedWords, st.MostUsedWords)
	if st.Span != nil {
		out.Span = ptr(st.Span.Seconds())
	}
	if g := st.Gaps; g != nil {
		out.Gaps = &Range{Min: g.Min.Seconds(), Average: g.Average.Seconds(), Max: g.Max.Seconds()}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("encode the statistics: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func ptr[T any](v T) *T { return &v }
