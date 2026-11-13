package karaoke_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var (
	beach       = cue(0, 1.6, []string{"The beach was quiet."}, word{"The", 0}, word{"beach", 0.24}, word{"was", 0.71}, word{"quiet.", 0.95})
	oneToFive   = cue(0, 5, []string{"One two three four five"}, word{"One", 0}, word{"two", 1}, word{"three", 2}, word{"four", 3}, word{"five", 4})
	yellow      = `font color="#ffff00"`
	yellowOpen  = `<font color="#ffff00">`
	yellowClose = "</font>"
)

func TestApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  model.Cue
		opts karaoke.Options
		want []out
	}{
		{"all words", beach, karaoke.Options{}, []out{
			rows(0, 0.24, "<u>The</u> beach was quiet."), rows(0.24, 0.71, "The <u>beach</u> was quiet."),
			rows(0.71, 0.95, "The beach <u>was</u> quiet."), rows(0.95, 1.6, "The beach was <u>quiet.</u>"),
		}},
		{"cumulative", beach, karaoke.Options{Style: yellow, Mode: karaoke.ModeCumulative}, []out{
			rows(0, 0.24, yellowOpen+"The"+yellowClose+" beach was quiet."), rows(0.24, 0.71, yellowOpen+"The beach"+yellowClose+" was quiet."),
			rows(0.71, 0.95, yellowOpen+"The beach was"+yellowClose+" quiet."), rows(0.95, 1.6, yellowOpen+"The beach was quiet."+yellowClose),
		}},
		{"window 3", oneToFive, karaoke.Options{MaxWordsPerCue: 3}, []out{
			rows(0, 1, "<u>One</u> two three"), rows(1, 2, "One <u>two</u> three"), rows(2, 3, "two <u>three</u> four"),
			rows(3, 4, "three <u>four</u> five"), rows(4, 5, "three four <u>five</u>"),
		}},
		{"window 2", oneToFive, karaoke.Options{MaxWordsPerCue: 2}, []out{
			rows(0, 1, "<u>One</u> two"), rows(1, 2, "<u>two</u> three"), rows(2, 3, "<u>three</u> four"),
			rows(3, 4, "<u>four</u> five"), rows(4, 5, "four <u>five</u>"),
		}},
		{"window 1", oneToFive, karaoke.Options{MaxWordsPerCue: 1}, []out{
			rows(0, 1, "<u>One</u>"), rows(1, 2, "<u>two</u>"), rows(2, 3, "<u>three</u>"), rows(3, 4, "<u>four</u>"), rows(4, 5, "<u>five</u>"),
		}},
		{
			"window over 2 lines and a tag", cue(2, 4, []string{"Hi, <i>the train</i>", "is late."}, word{"the", 2.5}, word{"train", 3}, word{"is late.", 3.5}),
			karaoke.Options{MaxWordsPerCue: 1},
			[]out{
				rows(2, 2.5, "Hi, <i>the</i>"), rows(2.5, 3, "Hi, <i><u>the</u></i>"), rows(3, 3.5, "<i><u>train</u></i>"), rows(3.5, 4, "<u>is late.</u>"),
			},
		},
		{
			"cumulative around tags", cue(0, 3, []string{"Hi <b>there</b> you"}, word{"Hi", 0}, word{"there", 1}, word{"you", 2}),
			karaoke.Options{Mode: karaoke.ModeCumulative},
			[]out{
				rows(0, 1, "<u>Hi</u> <b>there</b> you"), rows(1, 2, "<u>Hi</u> <b><u>there</u></b> you"), rows(2, 3, "<u>Hi</u> <b><u>there</u></b> <u>you</u>"),
			},
		},
		{"lead cue", cue(1, 3, []string{"So the end"}, word{"the", 2}, word{"end", 2.5}), karaoke.Options{}, []out{
			rows(1, 2, "So the end"), rows(2, 2.5, "So <u>the</u> end"), rows(2.5, 3, "So the <u>end</u>"),
		}},
		{"words outside the cue", cue(1, 3, []string{"Early same time late"}, word{"Early", 0.5}, word{"same", 2}, word{"time", 2}, word{"late", 4}), karaoke.Options{}, []out{
			rows(1, 2, "<u>Early</u> same time late"), rows(2, 3, "Early same <u>time</u> late"),
		}},
		{"all word cues empty", cue(2, 2, []string{"Too short"}, word{"Too", 2}, word{"short", 2}), karaoke.Options{}, []out{rows(2, 2, "Too short")}},
		{"angle brackets in text", cue(0, 1, []string{"a < b > c"}, word{"a", 0}, word{"< b > c", 0.5}), karaoke.Options{}, []out{
			rows(0, 0.5, "<u>a</u> < b > c"), rows(0.5, 1, "a <u>< b > c</u>"),
		}},
		{"no words", model.Cue{Start: 0, End: seconds(1), Lines: []string{"<i>Plain</i> text", "second line"}}, karaoke.Options{}, []out{rows(0, 1, "<i>Plain</i> text", "second line")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(tt.cue)
			_, err := karaoke.Apply(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, describe(s))
		})
	}
}

func TestApplyReport(t *testing.T) {
	t.Parallel()

	s := subtitle(beach)
	report, err := karaoke.Apply(s, karaoke.Options{})
	require.NoError(t, err)
	assert.Equal(t, karaoke.Report{CuesBefore: 1, CuesAfter: 4}, report)
}

func TestApplyCueData(t *testing.T) {
	t.Parallel()

	in := cue(0, 1.6, []string{"The beach was quiet."}, word{"The", 0}, word{"beach", 0.24}, word{"was", 0.71}, word{"quiet.", 0.95})
	in.ID, in.Forced, in.Speaker = "intro", true, "Ann"
	in.Position.Alignment = model.TopCenter
	require.NoError(t, in.SetFormatData("ass", model.NewData(model.Field{Key: "fields", Value: model.NewData(model.Field{Key: "Style", Value: "Karaoke"})})))
	wantData, _ := in.FormatData("ass")
	plain := model.Cue{Start: seconds(5), End: seconds(6), ID: "p", Lines: []string{"<i>Plain</i> text"}}
	plain.Position.Alignment = model.TopCenter

	s := subtitle(in, plain)
	_, err := karaoke.Apply(s, karaoke.Options{})
	require.NoError(t, err)
	cues := s.Cues()
	require.Len(t, cues, 5)
	for i, c := range cues[:4] {
		wantID := ""
		if i == 0 {
			wantID = "intro"
		}
		assert.Equal(t, wantID, c.ID)
		assert.Equal(t, "Ann", c.Speaker)
		assert.True(t, c.Forced)
		assert.Equal(t, model.TopCenter, c.Position.Alignment)
		assert.Nil(t, c.Words)
		data, ok := c.FormatData("ass")
		require.True(t, ok)
		assert.Equal(t, wantData, data)
	}
	assert.Equal(t, plain, cues[4])
}

func TestApplyComments(t *testing.T) {
	t.Parallel()

	s := subtitle(beach, cue(2, 3, []string{"Next line"}, word{"Next", 2}, word{"line", 2.5}))
	require.NoError(t, s.AddComment("before", 1))
	require.NoError(t, s.AddComment("end", 2))
	_, err := karaoke.Apply(s, karaoke.Options{})
	require.NoError(t, err)
	at := make([]int, 0, 2)
	for _, c := range s.Comments() {
		at = append(at, c.BeforeCue)
	}
	assert.Equal(t, []int{4, 6}, at)
}

func TestApplyClone(t *testing.T) {
	t.Parallel()

	s := subtitle(beach)
	_, err := karaoke.Apply(s.Clone(), karaoke.Options{})
	require.NoError(t, err)
	assert.Equal(t, 1, s.Len())
	first, _ := s.Cue(0)
	assert.Len(t, first.Words, 4)
}

func TestApplyTooLarge(t *testing.T) {
	t.Parallel()

	words := make([]word, 0, 100)
	text := ""
	for i := range 100 {
		words = append(words, word{strings.Repeat("a", 10000), float64(i)})
		text += words[i].text + " "
	}
	s := subtitle(cue(0, 200, []string{text[:len(text)-1]}, words...))
	_, err := karaoke.Apply(s, karaoke.Options{})
	var invalid *model.InvalidArgumentError
	require.ErrorAs(t, err, &invalid)
	assert.Contains(t, invalid.Msg, "cue 1")
	assert.Contains(t, invalid.Msg, "resegment.SplitLong")
	assert.Equal(t, 1, s.Len())
	first, _ := s.Cue(0)
	assert.Len(t, first.Words, len(words))

	_, err = karaoke.Apply(s, karaoke.Options{MaxWordsPerCue: 1})
	require.NoError(t, err)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		opts karaoke.Options
		opt  string
		msg  string
	}{
		{karaoke.Options{Style: "em"}, "Style", `The style em must be one of the core markup tags b, i, u, s or font, for example u or font color="#ffff00".`},
		{karaoke.Options{Style: "v"}, "Style", `The style v must be one of the core markup tags b, i, u, s or font, for example u or font color="#ffff00".`},
		{karaoke.Options{Style: `font color="a>b"`}, "Style", `The style font color="a>b" must not hold < or >. Pass the tag without its brackets, for example u.`},
		{karaoke.Options{MaxWordsPerCue: -2}, "MaxWordsPerCue", "The maximum of -2 words per cue must be 1 or more."},
	}
	for _, tt := range tests {
		t.Run(tt.opt+tt.msg, func(t *testing.T) {
			t.Parallel()

			s := subtitle(beach)
			_, err := karaoke.Apply(s, tt.opts)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, "karaoke.Options."+tt.opt, invalid.Option)
			assert.Equal(t, tt.msg, invalid.Msg)
			assert.Equal(t, 1, s.Len())
		})
	}
	require.NoError(t, karaoke.Options{Style: " B "}.Validate())
}

func TestWindow(t *testing.T) {
	t.Parallel()

	for n := 1; n <= 6; n++ {
		for limit := 1; limit <= n+1; limit++ {
			t.Run(strconv.Itoa(n)+"words limit "+strconv.Itoa(limit), func(t *testing.T) {
				t.Parallel()

				words := make([]word, 0, n)
				text := ""
				for k := range n {
					w := string(rune('a' + k))
					words = append(words, word{w, float64(k + 1)})
					text += w + " "
				}
				s := subtitle(cue(0, float64(n+1), []string{text[:len(text)-1]}, words...))
				_, err := karaoke.Apply(s, karaoke.Options{MaxWordsPerCue: limit})
				require.NoError(t, err)
				cues := s.Cues()
				require.Len(t, cues, n+1)
				for a, c := range cues[1:] {
					first := max(0, min(a-(limit-1)/2, n-limit))
					last := first + limit - 1
					if limit >= n {
						first, last = 0, n-1
					}
					want := ""
					for k := first; k <= last; k++ {
						if k > first {
							want += " "
						}
						if k == a {
							want += "<u>" + string(rune('a'+k)) + "</u>"
						} else {
							want += string(rune('a' + k))
						}
					}
					assert.Equal(t, []string{want}, c.Lines)
				}
			})
		}
	}
}

func TestGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input, want string
		opts        karaoke.Options
	}{
		{"karaoke/hebrew.lrc", "karaoke/hebrew_word.srt", karaoke.Options{}},
		{"lrc/real/handwritten-enhanced.lrc", "karaoke/lrc_cumulative.srt", karaoke.Options{Style: yellow, Mode: karaoke.ModeCumulative}},
		{"ass/real/own_aegisub.ass", "karaoke/aegisub_window.srt", karaoke.Options{MaxWordsPerCue: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, tt.input)
			_, err := karaoke.Apply(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, describe(readFile(t, tt.want)), describe(s))
			for _, c := range s.All() {
				assert.Nil(t, c.Words)
			}
		})
	}
}

func TestModeString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "word", karaoke.ModeWord.String())
	assert.Equal(t, "cumulative", karaoke.ModeCumulative.String())
	assert.Equal(t, "Mode(7)", karaoke.Mode(7).String())
}
