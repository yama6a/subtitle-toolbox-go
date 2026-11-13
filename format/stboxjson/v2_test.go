package stboxjson_test

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func confidence(f float64) *float64 { return &f }

func beachCue() model.Cue {
	return model.Cue{
		Start: time.Second, End: 5 * time.Second, Lines: []string{"<i>The beach</i> was empty."}, Speaker: "Anna",
		Words: []model.Word{
			{Text: "The", Start: time.Second, End: 1200 * time.Millisecond},
			{Text: "beach", Start: 1200 * time.Millisecond, End: 2 * time.Second, Confidence: confidence(0.93)},
		},
	}
}

func TestWriteVersions(t *testing.T) {
	t.Parallel()

	withData := beachCue()
	require.NoError(t, withData.SetFormatData("vtt", model.NewData(model.Field{Key: "voiceClass", Value: "loud"})))
	forced := beachCue()
	forced.Forced = true
	forced.ID = "a"
	plain := model.Cue{End: time.Second, Lines: []string{"Rain"}}
	tests := []struct {
		name    string
		cue     model.Cue
		version int
		want    string
	}{
		{"default writes version 2 with the key order", forced, 0, `{"version":2,"metadata":{},"comments":[],"formatData":{},"cues":[` +
			`{"start":1.0,"end":5.0,"lines":["<i>The beach</i> was empty."],"identifier":"a","alignment":null,"forced":true,` +
			`"speaker":"Anna","words":[{"text":"The","start":1.0,"end":1.2},{"text":"beach","start":1.2,"end":2.0,"confidence":0.93}],"formatData":{}}]}`},
		{"version 2 keeps the voice form in the format data", withData, 2, `{"version":2,"metadata":{},"comments":[],"formatData":{},"cues":[` +
			`{"start":1.0,"end":5.0,"lines":["<i>The beach</i> was empty."],"identifier":null,"alignment":null,` +
			`"speaker":"Anna","words":[{"text":"The","start":1.0,"end":1.2},{"text":"beach","start":1.2,"end":2.0,"confidence":0.93}],` +
			`"formatData":{"vtt":{"voiceClass":"loud"}}}]}`},
		{"version 1 writes the tags back", beachCue(), 1, `{"version":1,"metadata":{},"comments":[],"formatData":{},"cues":[` +
			`{"start":1.0,"end":5.0,"lines":["<v Anna><00:00:01.000><i>The <00:00:01.200>beach</i> was empty."],"identifier":null,"alignment":null,"formatData":{}}]}`},
		{"version 2 leaves out an empty speaker and words", plain, 2, `{"version":2,"metadata":{},"comments":[],"formatData":{},"cues":[` +
			`{"start":0.0,"end":1.0,"lines":["Rain"],"identifier":null,"alignment":null,"formatData":{}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCue(tt.cue)
			assert.Equal(t, tt.want, writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{Version: tt.version}))
		})
	}
}

func TestWriteVersionErrors(t *testing.T) {
	t.Parallel()

	nan := beachCue()
	nan.Words[1].Confidence = confidence(math.NaN())
	badUTF8 := beachCue()
	badUTF8.Words[0].Text = "\xff"
	tests := []struct {
		name string
		cue  model.Cue
		opts stboxjson.WriteOptions
		want string
	}{
		{"version 3", beachCue(), stboxjson.WriteOptions{Version: 3}, "The library JSON version must be 0, 1 or 2, got 3."},
		{"NaN confidence", nan, stboxjson.WriteOptions{}, infOrNaN},
		{"bad UTF-8 in a word", badUTF8, stboxjson.WriteOptions{}, "Malformed UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCue(tt.cue)
			_, err := stboxjson.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

const youtubeLine = "the<00:00:01.000><c> train</c><c.red><00:00:02.000><c> leaves</c></c>"

// A WebVTT cue whose tags the placement rule would write elsewhere keeps its tagged lines in the vtt cue data.
func TestVersion2KeepsTheTaggedLinesOfWebVTT(t *testing.T) {
	t.Parallel()

	c := model.Cue{End: 5 * time.Second, Lines: []string{youtubeLine}}
	require.NoError(t, c.SetFormatData("vtt", model.NewData(model.Field{Key: "align", Value: "start"})))
	vtt.ExtractWords(&c)
	s := &model.Subtitle{}
	s.AddCue(c)
	out := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
	assert.Contains(t, out, `"formatData":{"vtt":{"align":"start","taggedLines":["`+youtubeLine+`"]}}`)
	assert.NotContains(t, writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{Version: 1}), "taggedLines")

	got, ok := mustRead(t, out, false).Cue(0)
	require.True(t, ok)
	assert.Equal(t, []string{youtubeLine}, got.TaggedLines())

	edited, _ := s.Cue(0)
	edited.Words[1].Start += time.Second
	s2 := &model.Subtitle{}
	s2.AddCue(edited)
	assert.NotContains(t, writeString(t, s2, model.CommonWriteOptions{}, stboxjson.WriteOptions{}), "taggedLines")
}

// A stored layout that no longer matches the cue is dropped with a warning, also in strict mode.
func TestReadDropsATaggedLinesKeyThatDoesNotMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, lines, speaker string
	}{
		{"another word start", `<00:00:01.500>a <00:00:02.000>b`, ""},
		{"another speaker", `<v Ben><00:00:01.000>a <00:00:02.000>b`, "Anna"},
		{"another text", `<00:00:01.000>a <00:00:02.000>c`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			speaker := ""
			if tt.speaker != "" {
				speaker = `"speaker": "` + tt.speaker + `", `
			}
			doc := `{"version": 2, "cues": [{"start": 1, "end": 5, "lines": ["a b"], ` + speaker +
				`"words": [{"text": "a", "start": 1, "end": 2}, {"text": "b", "start": 2, "end": 3}], ` +
				`"formatData": {"vtt": {"taggedLines": ["` + tt.lines + `"]}}}]}`
			for _, lenient := range []bool{false, true} {
				s, warnings, err := read(t, doc, lenient)
				require.NoError(t, err)
				require.Len(t, warnings, 1)
				assert.Equal(t, model.Repaired, warnings[0].Action)
				assert.Contains(t, warnings[0].Message, "The field cues[0].formatData.vtt.taggedLines does not match")
				c, _ := s.Cue(0)
				assert.Empty(t, c.FormatDataKeys())
			}
		})
	}
}

func TestVersion2RoundTrip(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	c := beachCue()
	require.NoError(t, c.SetFormatData("vtt", model.NewData(model.Field{Key: "voiceClass", Value: "loud"}, model.Field{Key: "voiceEnd", Value: true})))
	s.AddCue(c)
	out := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{PrettyPrint: true})

	got := mustRead(t, out, false)
	again, ok := got.Cue(0)
	require.True(t, ok)
	assert.Equal(t, c.Lines, again.Lines)
	assert.Equal(t, c.Speaker, again.Speaker)
	assert.Equal(t, c.Words, again.Words)
	assert.Equal(t, c.FormatDataKeys(), again.FormatDataKeys())
	assert.Equal(t, out, writeString(t, got, model.CommonWriteOptions{}, stboxjson.WriteOptions{PrettyPrint: true}))
}

func TestReadVersions(t *testing.T) {
	t.Parallel()

	v1 := mustRead(t, `{"version": 1, "cues": [{"start": 1, "end": 5, "lines": ["<v Anna><00:00:01.000>The <00:00:01.200>beach"]}]}`, false)
	c, _ := v1.Cue(0)
	assert.Equal(t, "Anna", c.Speaker)
	assert.Equal(t, []model.Word{{Text: "The", Start: ms(1000), End: ms(1200)}, {Text: "beach", Start: ms(1200), End: ms(5000)}}, c.Words)

	v2 := mustRead(t, `{"version": 2, "cues": [{"start": 1, "end": 5, "lines": ["<v Ben><00:00:01.000>The beach"], "speaker": null, "words": null}]}`, false)
	c, _ = v2.Cue(0)
	assert.Equal(t, []string{"<v Ben><00:00:01.000>The beach"}, c.Lines)
	assert.Empty(t, c.Speaker)
	assert.Empty(t, c.Words)
}

// A time literal with more than a million fraction digits reads as its rounded value and does not crash.
func TestReadTimeWithAMillionFractionDigits(t *testing.T) {
	t.Parallel()

	digits := strings.Repeat("0", 1_000_001)
	tests := []struct {
		name, doc string
	}{
		{"cue times version 1", `{"version": 1, "cues": [{"start": 1.` + digits + `1, "end": 2.` + digits + `, "lines": ["a"]}]}`},
		{"cue times version 2", `{"version": 2, "cues": [{"start": 1.` + digits + `1, "end": 2.` + digits + `, "lines": ["a"]}]}`},
		{"word times", `{"version": 2, "cues": [{"start": 1, "end": 2, "lines": ["a"], "words": [{"text": "a", "start": 1.` + digits + `1, "end": 2.` + digits + `}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, tt.doc, false)
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, time.Second, c.Start)
			assert.Equal(t, 2*time.Second, c.End)
			if len(c.Words) > 0 {
				assert.Equal(t, model.Word{Text: "a", Start: time.Second, End: 2 * time.Second}, c.Words[0])
			}
		})
	}
}

func TestReadVersion2LenientSkipsABadCue(t *testing.T) {
	t.Parallel()

	content := `{"version": 2, "cues": [` + oneCue + `, {"start": 1, "end": 2, "lines": ["a"], "words": [{"text": "b", "start": 1, "end": 2}]}]}`
	s, warnings, err := read(t, content, true)
	require.NoError(t, err)
	assert.Equal(t, 1, s.Len())
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Skipped, warnings[0].Action)
	assert.Equal(t, "The field cues[1].words[0] is not in the text of the cue after the word before it.", warnings[0].Message)
}

// BenchmarkReadVersion2 reads a document of about 1 MB with the reader checks of every word. A cue with 12,000 words
// shows that the order rule searches the text once, not once per word.
func BenchmarkReadVersion2(b *testing.B) {
	for _, tt := range []struct{ cues, words int }{{700, 20}, {1, 12000}} {
		b.Run("cues="+strconv.Itoa(tt.cues)+"/words="+strconv.Itoa(tt.words), func(b *testing.B) {
			var doc strings.Builder
			doc.WriteString(`{"version": 2, "cues": [`)
			for i := range tt.cues {
				if i > 0 {
					doc.WriteString(",")
				}
				var words, text strings.Builder
				for j := range tt.words {
					if j > 0 {
						words.WriteString(",")
						text.WriteString(" ")
					}
					word := "w" + strconv.Itoa(j)
					text.WriteString(word)
					words.WriteString(`{"text":"` + word + `","start":` + strconv.Itoa(i) + `.0,"end":` + strconv.Itoa(i) + `.5,"confidence":0.5}`)
				}
				doc.WriteString(`{"start":` + strconv.Itoa(i) + `.0,"end":` + strconv.Itoa(i) + `.9,"lines":["` + text.String() + `"],"speaker":"Anna","words":[` + words.String() + `]}`)
			}
			doc.WriteString("]}")
			content := doc.String()
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := stboxjson.Read(strings.NewReader(content), model.CommonReadOptions{}, stboxjson.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
