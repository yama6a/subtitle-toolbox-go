package wordgroup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		json string
		unit time.Duration
		want time.Duration
		err  string
	}{
		{"number", `1.5`, time.Second, ms(1500), ""},
		{"numeric string", `"2.0"`, time.Second, ms(2000), ""},
		{"unit", `1500`, time.Millisecond, ms(1500), ""},
		{"negative", `-1`, time.Second, 0, "The field f must be a time."},
		{"not numeric", `"abc"`, time.Second, 0, "The field f must be a time."},
		{"not finite", `1e400`, time.Second, 0, "The field f must be a time."},
		{"null", `null`, time.Second, 0, "The field f must be a time."},
		{"bool", `true`, time.Second, 0, "The field f must be a time."},
		{"100000 hours", `360000000`, time.Second, 0, "The field f is not below 100000 hours."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := wordgroup.Seconds(parseJSON(t, tt.json), "f", tt.unit)
			if tt.err != "" {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.err, perr.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	for _, js := range []string{`{"t": 5}`, `{}`, `[1]`, `"t"`} {
		_, err := wordgroup.Text(parseJSON(t, js), "t", "p[0]")
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr, js)
		assert.Equal(t, "The field p[0].t must be a string.", perr.Msg)
	}
	got, err := wordgroup.Text(parseJSON(t, `{"t": "x"}`), "t", "p")
	require.NoError(t, err)
	assert.Equal(t, "x", got)
}

func TestSpeaker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		json string
		name string
		ok   bool
	}{
		{`"A"`, "A", true},
		{`5`, "5", true},
		{`5.0`, "", false},
		{`true`, "", false},
		{`null`, "", false},
	}
	for _, tt := range tests {
		name, ok := wordgroup.Speaker(parseJSON(t, tt.json))
		assert.Equal(t, tt.name, name, tt.json)
		assert.Equal(t, tt.ok, ok, tt.json)
	}
}

func TestBetween(t *testing.T) {
	t.Parallel()
	words := []wordgroup.Word{word("a", 100, 700), word("b", 700, 1100), word("c", 1100, 1500), word("d", 1900, 2500)}
	tests := []struct {
		name       string
		from       int // the index to start at
		start, end int
		want       []string
		next       int
	}{
		{"float middle 0.1 and 0.7 on boundary 0.4", 0, 0, 400, []string{"a"}, 1},
		{"middle on the end boundary", 0, 500, 900, []string{"b"}, 2},
		{"skips words before the start", 0, 1000, 1300, []string{"c"}, 3},
		{"nothing", 2, 1600, 1700, nil, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			i := tt.from
			var got []string
			for _, w := range wordgroup.Between(words, &i, ms(tt.start), ms(tt.end)) {
				got = append(got, w.Text)
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.next, i)
		})
	}
}

func TestWithResults(t *testing.T) {
	t.Parallel()
	data := parseJSON(t, `{"a": 1, "results": {"items": [], "x": 2}, "b": 3}`)
	results, _ := data.Get("results")
	assert.JSONEq(t, `{"a":1,"b":3,"results":{"x":2}}`, orderedjson.Block(wordgroup.WithResults(data, results, "items")))
	assert.JSONEq(t, `{"a":1,"b":3}`, orderedjson.Block(wordgroup.WithResults(data, results, "items", "x")))
}

func TestCueWords(t *testing.T) {
	t.Parallel()
	untimed := word("then", 0, 0)
	untimed.Timed = false
	noEnd := func(text string, start int) wordgroup.Word {
		return wordgroup.Word{Text: text, Start: ms(start), Timed: true}
	}
	half := 0.5
	tests := []struct {
		name  string
		text  string
		words []wordgroup.Word
		want  []model.Word
	}{
		{"tea and toast", "Tea, then toast <now> & later.", []wordgroup.Word{
			noEnd("Tea,", 1000), noEnd("coffee", 1500), untimed, noEnd("toast", 2250), noEnd("<now>", 3000), noEnd("&", 3500),
		}, []model.Word{
			{Text: "Tea,", Start: ms(1000), End: ms(2250)},
			{Text: "toast", Start: ms(2250), End: ms(3000)},
			{Text: "<now>", Start: ms(3000), End: ms(3500)},
			{Text: "&", Start: ms(3500), End: ms(4000)},
		}},
		{
			"end before start", "a b",
			[]wordgroup.Word{word("a", 500, 100), noEnd("b", 900)},
			[]model.Word{{Text: "a", Start: ms(500), End: ms(500)}, {Text: "b", Start: ms(900), End: ms(4000)}},
		},
		{
			"start that goes back", "a b c",
			[]wordgroup.Word{word("a", 100, 200), word("b", 50, 80), word("c", 300, 400)},
			[]model.Word{{Text: "a", Start: ms(100), End: ms(200)}, {Text: "c", Start: ms(300), End: ms(400)}},
		},
		{
			"confidence is copied", "a",
			[]wordgroup.Word{{Text: "a", Start: 0, Timed: true, HasEnd: true, End: ms(10), Confidence: &half}},
			[]model.Word{{Text: "a", Start: 0, End: ms(10), Confidence: &half}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := wordgroup.CueWords(tt.text, tt.words, ms(4000))
			assert.Equal(t, tt.want, got)
			for _, w := range got {
				if w.Confidence != nil {
					assert.NotSame(t, &half, w.Confidence)
				}
			}
		})
	}
}
