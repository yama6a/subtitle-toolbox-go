package stats_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/stats"
)

func TestMarshalJSON(t *testing.T) {
	t.Parallel()

	got, err := json.Marshal(stats.Of(load(t, "statistics/own_bakery.srt")))

	require.NoError(t, err)
	assert.Equal(t, `{"cueCount":4,"wordCount":26,"characterCount":123,"totalDisplayTime":9,"span":11,`+
		`"charactersPerSecond":{"min":10.857142857142858,"average":14.064285714285713,"max":18.4},`+
		`"wordsPerMinute":{"min":137.14285714285714,"average":176.78571428571428,"max":240},`+
		`"charactersPerLine":{"min":15,"average":20.5,"max":26},`+
		`"gaps":{"min":-0.5,"average":0.666666667,"max":2},`+
		`"mostUsedWords":[{"word":"the","count":4},{"word":"bread","count":3},{"word":"bakery","count":2},`+
		`{"word":"at","count":2},{"word":"opens","count":1},{"word":"six","count":1},{"word":"fresh","count":1},`+
		`{"word":"warm","count":1},{"word":"is","count":1},{"word":"gone","count":1}]}`, string(got))
}

func TestMarshalJSONWithoutCues(t *testing.T) {
	t.Parallel()

	got, err := json.Marshal(stats.Statistics{})

	require.NoError(t, err)
	assert.Equal(t, `{"cueCount":0,"wordCount":0,"characterCount":0,"totalDisplayTime":0,"span":null,`+
		`"charactersPerSecond":null,"wordsPerMinute":null,"charactersPerLine":null,"gaps":null,"mostUsedWords":[]}`, string(got))
}

func TestMarshalJSONDurations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   stats.Statistics
		want string
	}{
		{"fraction", stats.Statistics{TotalDisplayTime: 4900 * time.Millisecond, Span: ptr(time.Second)}, `"totalDisplayTime":4.9,"span":1,`},
		{"negative", stats.Statistics{Span: ptr(-1500 * time.Millisecond)}, `"span":-1.5,`},
		{"zero", stats.Statistics{Span: ptr(time.Duration(0))}, `"span":0,`},
		{
			"gaps",
			stats.Statistics{Gaps: &stats.DurationRange{Min: -500 * time.Millisecond, Average: 250 * time.Millisecond, Max: time.Second}},
			`"gaps":{"min":-0.5,"average":0.25,"max":1},`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tt.in)

			require.NoError(t, err)
			assert.Contains(t, string(got), tt.want)
		})
	}
}

func TestMarshalJSONWritesTenWords(t *testing.T) {
	t.Parallel()

	all := make([]stats.WordCount, 12)
	for i := range all {
		all[i] = stats.WordCount{Word: fmt.Sprintf("w%d", i), Count: 12 - i}
	}

	got, err := json.Marshal(stats.Statistics{MostUsedWords: all})

	require.NoError(t, err)
	var decoded struct {
		MostUsedWords []stats.WordCount `json:"mostUsedWords"`
	}
	require.NoError(t, json.Unmarshal(got, &decoded))
	assert.Len(t, decoded.MostUsedWords, stats.MostUsedWordsInJSON)
	assert.Equal(t, 10, stats.MostUsedWordsInJSON)
	assert.Equal(t, "w9", decoded.MostUsedWords[9].Word)
}

func TestMarshalJSONKeepsHTMLCharacters(t *testing.T) {
	t.Parallel()

	got, err := stats.Of(subtitle(cue(0, 1, "R&B <3"))).MarshalJSON()

	require.NoError(t, err)
	assert.Contains(t, string(got), `{"word":"r&b","count":1}`)
}
