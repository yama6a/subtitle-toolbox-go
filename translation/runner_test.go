package translation_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
)

type call struct {
	texts          []string
	source, target string
}

// upperEngine upper-cases the ASCII letters of the text between tags and entities, as the PHP test engine does.
type upperEngine struct {
	drop  bool
	calls []call
}

var (
	placeholderPattern = regexp.MustCompile(`</?x\d+/?>`)
	upperPattern       = regexp.MustCompile(`<[^<>]*>|&[#\w]+;|[^<&]+`)
)

func (e *upperEngine) Translate(_ context.Context, texts []string, source, target string) ([]string, error) {
	e.calls = append(e.calls, call{texts: texts, source: source, target: target})
	out := make([]string, len(texts))
	for i, text := range texts {
		if e.drop {
			text = placeholderPattern.ReplaceAllString(text, "")
		}
		out[i] = upperPattern.ReplaceAllStringFunc(text, func(m string) string {
			if m[0] == '<' || m[0] == '&' {
				return m
			}
			return strings.Map(func(r rune) rune {
				if r >= 'a' && r <= 'z' {
					return r - 'a' + 'A'
				}
				return r
			}, m)
		})
	}
	return out, nil
}

// fixedEngine returns its first strings, one for each text.
type fixedEngine []string

func (e fixedEngine) Translate(_ context.Context, texts []string, _, _ string) ([]string, error) {
	return e[:min(len(texts), len(e))], nil
}

type funcEngine func(ctx context.Context, n int) ([]string, error)

func (f funcEngine) Translate(ctx context.Context, texts []string, _, _ string) ([]string, error) {
	return f(ctx, len(texts))
}

func readFile(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, name)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

func fromTexts(texts ...string) *model.Subtitle {
	s := &model.Subtitle{}
	for i, text := range texts {
		start := time.Duration(1+2*i) * time.Second
		c := model.Cue{Start: start, End: start + time.Second}
		c.SetText(text)
		s.AddCue(c)
	}
	return s
}

func lines(s *model.Subtitle) [][]string {
	out := make([][]string, 0, len(s.Cues()))
	for _, c := range s.Cues() {
		out = append(out, c.Lines)
	}
	return out
}

func cueWarnings(r translation.Report) []int {
	if len(r.Warnings) == 0 {
		return nil
	}
	out := make([]int, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		out = append(out, w.Cue)
	}
	return out
}

func TestTranslateOwnStation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		drop         bool
		wantLines    map[int][]string
		wantWarnings []int
	}{
		{
			name: "tags survive and sentences over 2 cues go out as one text",
			wantLines: map[int][]string{
				0: {"THE TRAIN TO BASEL LEAVES"},
				1: {"FROM PLATFORM 4 AT <b>10:15</b>."},
				2: {"\u266a \u266a"},
				3: {"<i>TICKETS &amp; SEAT", "RESERVATIONS</i> ARE SOLD HERE."},
				4: {"2024"},
				5: {"- IS THIS SEAT FREE?", "- YES, IT IS."},
				6: {"<i>THE NEXT STOP IS</i>"},
				7: {"<i>ZURICH MAIN STATION.</i>"},
			},
		},
		{
			name: "dropped placeholders strip the tags and warn",
			drop: true,
			wantLines: map[int][]string{
				1: {"FROM PLATFORM 4 AT 10:15."},
				3: {"TICKETS &amp; SEAT", "RESERVATIONS ARE SOLD HERE."},
				5: {"- IS THIS SEAT FREE?", "- YES, IT IS."},
			},
			wantWarnings: []int{0, 1, 3, 6, 7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, "translation/own_station.srt")
			require.NoError(t, s.AddComment("Station announcements", 3))
			engine := &upperEngine{drop: tt.drop}

			report, err := translation.Translate(t.Context(), s, engine, "en", "de", translation.Options{})

			require.NoError(t, err)
			assert.Equal(t, []call{{
				texts: []string{
					"The train to Basel leaves from platform 4 at <x1>10:15</x1>.",
					"<x1>Tickets &amp; seat\nreservations</x1> are sold here.",
					"- Is this seat free?\n- Yes, it is.",
					"<x1>The next stop is Zurich main station.</x1>",
				},
				source: "en", target: "de",
			}}, engine.calls)
			got := lines(s)
			for i, want := range tt.wantLines {
				assert.Equal(t, want, got[i], "cue %d", i)
			}
			assert.Equal(t, tt.wantWarnings, cueWarnings(report))
			if len(tt.wantWarnings) > 0 {
				assert.Equal(t, "The engine dropped or changed a placeholder tag. The cue has no tags.", report.Warnings[0].Message)
			}
			language, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, "de", language)
			cues := s.Cues()
			assert.Equal(t, time.Second, cues[0].Start)
			assert.Equal(t, 21*time.Second, cues[7].End)
			require.Len(t, s.Comments(), 1)
			assert.Equal(t, "Station announcements", s.Comments()[0].Text)
			assert.Equal(t, 3, s.Comments()[0].BeforeCue)
		})
	}
}

func TestTranslateSends(t *testing.T) {
	t.Parallel()

	image := &model.Image{PNG: []byte("PNG"), Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080}
	joining := []string{
		"one", "two", "three", "four", "five.", "six", "\u266a", "seven", "\u201cEight?\u201d", "nine", "\u6b21\u3002", "ten",
	}
	tests := []struct {
		name  string
		file  string
		cues  []string
		image bool
		opts  translation.Options
		want  [][]string
	}{
		{
			name:  "cues without text to translate are not sent",
			cues:  []string{"\u266a\u266b", "1984", "...", "<i>\u266a</i>", "Hello."},
			image: true,
			want:  [][]string{{"Hello."}},
		},
		{
			name: "joining stops at a sentence end, the limit and an untranslated cue",
			cues: joining,
			want: [][]string{{"one two three", "four five.", "six", "seven \u201cEight?\u201d", "nine \u6b21\u3002", "ten"}},
		},
		{
			name: "no joining",
			cues: []string{"one", "two", "three."},
			opts: translation.Options{NoJoinSentences: true},
			want: [][]string{{"one", "two", "three."}},
		},
		{
			name: "2 cues per sentence",
			cues: []string{"one", "two", "three."},
			opts: translation.Options{MaxCuesPerSentence: 2},
			want: [][]string{{"one two", "three."}},
		},
		{
			name: "batches of 32 characters, a longer text alone",
			cues: []string{"First sentence.", "Second sentence.", "Third one is", "split over cues.", strings.Repeat("Long. ", 10)},
			opts: translation.Options{MaxCharactersPerRequest: 32},
			want: [][]string{{"First sentence.", "Second sentence."}, {"Third one is split over cues."}, {strings.TrimSpace(strings.Repeat("Long. ", 10))}},
		},
		{
			name: "no cue, no call",
			want: [][]string{},
		},
		{
			name: "styled file",
			file: "srt/real/own_styled.srt",
			want: [][]string{{
				"<x1><x2>[train horn]</x2></x1> <x3>[rain]</x3> <x4>[train horn]</x4>",
				"<x1><x2>[oven door opens]</x2></x1> [whistle] [station speaker] <x3>Thebakeryopensat7:35.</x3>",
				"<x1>Italics over two lines that close on the second line</x1> <x2/>Italics that never close The next cue is not italic",
				"<x1>x</x1>^3 * <x2>x</x2> = 100",
			}},
		},
		{
			name: "escaping file",
			file: "srt/real/own_escaping.srt",
			want: [][]string{{
				"I &lt;3 bread &amp; jam <x1>Salt &amp; pepper</x1> on the <x2>left</x2> Platform 2 &gt; platform 1 &lt; platform 3",
				"The sign says &amp;amp; and &amp;lt;b&amp;gt; <x1>Rain &amp; wind &gt;&gt; 40 km/h</x1>",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.cues...)
			if tt.file != "" {
				s = readFile(t, tt.file)
			}
			if tt.image {
				s.AddCue(model.Cue{Start: 20 * time.Second, End: 21 * time.Second, Image: image})
			}
			engine := &upperEngine{}

			_, err := translation.Translate(t.Context(), s, engine, "en", "de", tt.opts)

			require.NoError(t, err)
			got := make([][]string, 0, len(engine.calls))
			for _, c := range engine.calls {
				got = append(got, c.texts)
			}
			assert.Equal(t, tt.want, got)
			language, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, "de", language)
		})
	}
}

func TestTranslateFills(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		file         string
		cues         []string
		answers      []string
		want         [][]string
		wantWarnings []int
	}{
		{name: "broken closing placeholder", cues: []string{"<i>Lauf!</i>"}, answers: []string{"<x1>Run! </ x1>"}, want: [][]string{{"Run!"}}, wantWarnings: []int{0}},
		{name: "swapped placeholders", cues: []string{"<i>Lauf!</i>"}, answers: []string{"Run!</x1><x1>"}, want: [][]string{{"Run!"}}, wantWarnings: []int{0}},
		{name: "extra placeholder", cues: []string{"<i>Lauf!</i>"}, answers: []string{"<x1>Run!</x1><x2/>"}, want: [][]string{{"Run!"}}, wantWarnings: []int{0}},
		{name: "doubled placeholder", cues: []string{"<i>Lauf!</i>"}, answers: []string{"<x1>Run!</x1></x1>"}, want: [][]string{{"Run!"}}, wantWarnings: []int{0}},
		{
			name:    "entities of the answer become core text",
			cues:    []string{"<b>Tom &amp; Jerry</b> sagen a &lt; b"},
			answers: []string{"<x1>Tom & Jerry</x1> &#39;say&#39; a < b &amp; c&nbsp;d"},
			want:    [][]string{{"<b>Tom &amp; Jerry</b> 'say' a &lt; b &amp; c\u00a0d"}},
		},
		{
			name:    "split at a word boundary in proportion to the characters",
			cues:    []string{"Regularly he takes part in events of", "the patient organization."},
			answers: []string{"Er nimmt regelm\u00e4ssig an Veranstaltungen der Patientenorganisation teil."},
			want:    [][]string{{"Er nimmt regelm\u00e4ssig an Veranstaltungen der"}, {"Patientenorganisation teil."}},
		},
		{
			name:    "text without spaces is cut between characters",
			cues:    []string{"<i>The train", "leaves now.</i>"},
			answers: []string{"<x1>\u5217\u8f66\u73b0\u5728\u51fa\u53d1\u3002</x1>"},
			want:    [][]string{{"<i>\u5217\u8f66\u73b0</i>"}, {"<i>\u5728\u51fa\u53d1\u3002</i>"}},
		},
		{
			name:         "a translation that is too short leaves a cue empty",
			cues:         []string{"Yes,", "of course."},
			answers:      []string{"Ja"},
			want:         [][]string{{"Ja"}, nil},
			wantWarnings: []int{1},
		},
		{
			name: "styled file",
			file: "srt/real/own_styled.srt",
			want: [][]string{
				{`<font color="#00ff00"><b>[TRAIN HORN]</b></font>`},
				{`<font color="#ff00ff">[RAIN]</font>`},
				{`<font color="#00ff00">[TRAIN HORN]</font>`},
				{"<b><u>[OVEN DOOR OPENS]</u></b>"},
				{"[WHISTLE]"},
				{"[STATION SPEAKER] <i>THEBAKERYOPENSAT7:35.</i>"},
				{"<i>ITALICS OVER TWO LINES THAT CLOSE ON THE SECOND LINE</i>"},
				{"<i>ITALICS THAT NEVER CLOSE"},
				{"THE NEXT CUE IS NOT ITALIC"},
				{"<i>X</i>^3 * <i>X</i> = 100"},
			},
		},
		{
			name: "escaping file",
			file: "srt/real/own_escaping.srt",
			want: [][]string{
				{"I &lt;3 BREAD &amp; JAM"},
				{"<i>SALT &amp; PEPPER</i> ON THE <b>LEFT</b>"},
				{"PLATFORM 2 &gt; PLATFORM 1 &lt; PLATFORM 3"},
				{"THE SIGN SAYS &amp;AMP; AND &amp;LT;B&amp;GT;"},
				{`<font color="#ffcc00">RAIN &amp; WIND &gt;&gt; 40 KM/H</font>`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.cues...)
			if tt.file != "" {
				s = readFile(t, tt.file)
			}
			var engine translation.Engine = &upperEngine{}
			if tt.answers != nil {
				engine = fixedEngine(tt.answers)
			}

			report, err := translation.Translate(t.Context(), s, engine, "de", "en", translation.Options{})

			require.NoError(t, err)
			got := lines(s)
			for i := range got {
				if len(got[i]) == 0 {
					got[i] = nil
				}
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantWarnings, cueWarnings(report))
		})
	}
}

func TestTranslateErrors(t *testing.T) {
	t.Parallel()

	errDown := errors.New("the engine is down")
	second := func() translation.Engine {
		n := 0
		return funcEngine(func(_ context.Context, count int) ([]string, error) {
			n++
			if n == 1 {
				return make([]string, count), nil
			}
			return nil, errDown
		})
	}
	tests := []struct {
		name       string
		engine     translation.Engine
		target     string
		opts       translation.Options
		cancelled  bool // the context ended before the call
		wantOption string
		wantMsg    string
		wantIs     error
		wantNot    error
	}{
		{
			name: "cues limit below 0", engine: &upperEngine{}, target: "de", opts: translation.Options{MaxCuesPerSentence: -1},
			wantOption: "translation.Options.MaxCuesPerSentence", wantMsg: "The maximum cues per sentence must be at least 1, got -1.",
		},
		{
			name: "characters limit below 0", engine: &upperEngine{}, target: "de", opts: translation.Options{MaxCharactersPerRequest: -5},
			wantOption: "translation.Options.MaxCharactersPerRequest", wantMsg: "The maximum characters per request must be at least 1, got -5.",
		},
		{name: "nil engine", target: "de", wantMsg: "The translation engine is missing."},
		{name: "empty target", engine: &upperEngine{}, target: " \t", wantMsg: "The target language is missing."},
		{
			name:   "wrong answer count",
			engine: fixedEngine{"Eins."},
			target: "de",
			wantIs: translation.ErrService,
		},
		{name: "engine error", engine: second(), target: "de", opts: translation.Options{MaxCharactersPerRequest: 4}, wantIs: errDown, wantNot: translation.ErrService},
		{name: "context ended before the call", engine: &upperEngine{}, target: "de", cancelled: true, wantIs: context.Canceled, wantNot: translation.ErrService},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts("one.", "two.")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancelled {
				cancel()
			}

			report, err := translation.Translate(ctx, s, tt.engine, "en", tt.target, tt.opts)

			require.Error(t, err)
			assert.Empty(t, report.Warnings)
			if tt.wantMsg != "" {
				var invalid *model.InvalidArgumentError
				require.ErrorAs(t, err, &invalid)
				assert.Equal(t, tt.wantOption, invalid.Option)
				assert.Equal(t, tt.wantMsg, invalid.Msg)
			}
			if tt.wantIs != nil {
				require.ErrorIs(t, err, tt.wantIs)
			}
			if tt.wantNot != nil {
				require.NotErrorIs(t, err, tt.wantNot)
			}
			var service *translation.ServiceError
			if errors.Is(tt.wantIs, translation.ErrService) {
				require.ErrorAs(t, err, &service)
				assert.Equal(t, 0, service.Status)
				assert.Equal(t, "translation: The translation engine must return one string per text, got 1 values for 2 texts.", err.Error())
			}
			assert.Equal(t, [][]string{{"one."}, {"two."}}, lines(s))
			_, ok := s.Metadata(model.MetadataLanguage)
			assert.False(t, ok)
		})
	}
}

func TestTranslateContextEndsInTheCall(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	engine := funcEngine(func(ctx context.Context, _ int) ([]string, error) {
		cancel()
		<-ctx.Done()
		return nil, errors.New("the engine is down")
	})
	s := fromTexts("one.")

	_, err := translation.Translate(ctx, s, engine, "en", "de", translation.Options{})

	require.ErrorIs(t, err, context.Canceled)
	var service *translation.ServiceError
	assert.NotErrorAs(t, err, &service)
	assert.NotContains(t, err.Error(), "the engine is down")
	assert.Equal(t, [][]string{{"one."}}, lines(s))
}

func BenchmarkTranslate(b *testing.B) {
	for _, cues := range []int{200, 2000} {
		texts := make([]string, cues)
		for i := range texts {
			texts[i] = "<i>The train to Basel leaves</i>"
			if i%2 == 1 {
				texts[i] = "from platform 4 at <b>10:15</b>."
			}
		}
		for _, limit := range []int{0, 100} {
			b.Run("cues="+strconv.Itoa(cues)+"/maxcharacters="+strconv.Itoa(limit), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					s := fromTexts(texts...)
					_, err := translation.Translate(b.Context(), s, &upperEngine{}, "en", "de", translation.Options{MaxCharactersPerRequest: limit})
					require.NoError(b, err)
				}
			})
		}
	}
}
