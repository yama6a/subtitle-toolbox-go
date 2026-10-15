package stboxjson_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func writeString(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts stboxjson.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, stboxjson.Write(&out, s, common, opts))
	return out.String()
}

func setFormatData(t *testing.T, s *model.Subtitle, key string, fields ...model.Field) {
	t.Helper()

	require.NoError(t, s.SetFormatData(key, model.NewData(fields...)))
}

func cueWithData(t *testing.T, c model.Cue, key string, fields ...model.Field) model.Cue {
	t.Helper()

	require.NoError(t, c.SetFormatData(key, model.NewData(fields...)))
	return c
}

// Ports testWritesCompactJsonByDefault, testWritesEmptyMapsAsObjectsAndNumericKeysAsObjectKeys and
// testWithoutFormatDataOption of tests/Formatters/JsonFormatterTest.php of subtitle-toolbox-php.
func TestWriteDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build func(t *testing.T) *model.Subtitle
		opts  stboxjson.WriteOptions
		want  string
	}{
		{
			name: "compact by default",
			build: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := &model.Subtitle{}
				s.SetMetadata(model.MetadataTitle, "Big Buck Bunny")
				s.SetMetadata(model.MetadataLanguage, "en")
				s.AddCue(model.Cue{Start: ms(1500), End: ms(4000), Lines: []string{"Hello", "<i>world</i>"}, ID: "intro", Alignment: model.TopCenter})
				require.NoError(t, s.AddComment("Translated by Jane Doe", 0))
				return s
			},
			want: `{"version":1,"metadata":{"language":"en","title":"Big Buck Bunny"},` +
				`"comments":[{"text":"Translated by Jane Doe","beforeCueIndex":0}],"formatData":{},` +
				`"cues":[{"start":1.5,"end":4.0,"lines":["Hello","<i>world</i>"],"identifier":"intro","alignment":8,"formatData":{}}]}`,
		},
		{
			name: "numeric keys as object keys",
			build: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := &model.Subtitle{}
				s.SetMetadata("0", "zero")
				setFormatData(t, s, "sub", model.Field{Key: "0", Value: "a"}, model.Field{Key: "1", Value: "b"})
				return s
			},
			want: `{"version":1,"metadata":{"0":"zero"},"comments":[],"formatData":{"sub":{"0":"a","1":"b"}},"cues":[]}`,
		},
		{
			name: "without format data",
			build: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := &model.Subtitle{}
				setFormatData(t, s, "ass", model.Field{Key: "title", Value: "Bakery"})
				s.AddCue(cueWithData(t, model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Rain"}}, "vtt",
					model.Field{Key: "settings", Value: "line:0"}))
				return s
			},
			opts: stboxjson.WriteOptions{OmitFormatData: true},
			want: `{"version":1,"metadata":{},"comments":[],"cues":[{"start":1.0,"end":2.0,"lines":["Rain"],"identifier":null,"alignment":null}]}`,
		},
		{
			name: "sorted metadata and format data keys",
			build: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := &model.Subtitle{}
				s.SetMetadata("b", "2")
				s.SetMetadata("a", "1")
				setFormatData(t, s, "vtt", model.Field{Key: "z", Value: "last"}, model.Field{Key: "a", Value: "first"})
				setFormatData(t, s, "ass", model.Field{Key: "x", Value: int64(1)})
				c := cueWithData(t, model.Cue{}, "vtt", model.Field{Key: "line", Value: "0"})
				s.AddCue(cueWithData(t, c, "ass", model.Field{Key: "text", Value: "x"}))
				return s
			},
			want: `{"version":1,"metadata":{"a":"1","b":"2"},"comments":[],"formatData":{"ass":{"x":1},"vtt":{"z":"last","a":"first"}},` +
				`"cues":[{"start":0.0,"end":0.0,"lines":[],"identifier":null,"alignment":null,"formatData":{"ass":{"text":"x"},"vtt":{"line":"0"}}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, writeString(t, tt.build(t), model.CommonWriteOptions{}, tt.opts))
		})
	}
}

// Ports testPrettyPrintEndsWithANewLine of tests/Formatters/JsonFormatterTest.php of subtitle-toolbox-php.
func TestPrettyPrintEndsWithANewLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "{\r\n    \"version\": 1,\r\n    \"metadata\": {},\r\n    \"comments\": [],\r\n    \"formatData\": {},\r\n    \"cues\": []\r\n}\r\n",
		writeString(t, &model.Subtitle{}, model.CommonWriteOptions{LineEnding: model.CRLF}, stboxjson.WriteOptions{PrettyPrint: true}))
}

// Ports testWritesImageCuesWithBase64Png and testKeepsValidUtf8FormatDataAsString of
// tests/Formatters/JsonFormatterTest.php of subtitle-toolbox-php.
func TestWritesBinaryFormatDataAsBase64(t *testing.T) {
	t.Parallel()

	png := "\x89PNG\r\n\x1a\n"
	s := &model.Subtitle{}
	setFormatData(t, s, "ass", model.Field{Key: "title", Value: "B\u00e4ckerei"}, model.Field{Key: "png", Value: "png"},
		model.Field{Key: "deep", Value: []any{model.NewData(model.Field{Key: "bytes", Value: "\xff\xfe"})}})
	s.AddCue(cueWithData(t, model.Cue{Start: time.Second, End: 2 * time.Second}, model.ImageKey,
		model.Field{Key: "png", Value: png}, model.Field{Key: "screenWidth", Value: int64(720)}))

	got := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
	assert.Contains(t, got, `"ass":{"title":"B`+"\u00e4"+`ckerei","png":"png","deep":[{"bytes":{"base64":"//4="}}]}`)
	assert.Contains(t, got, `"image":{"png":{"base64":"`+base64.StdEncoding.EncodeToString([]byte(png))+`"},"screenWidth":720}`)

	again := mustRead(t, got, false)
	assert.Equal(t, s, again)
}

func TestWriteCueFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  model.Cue
		want string
	}{
		{"bottom center as null", model.Cue{Start: 0, End: ms(1)}, `{"start":0.0,"end":0.001,"lines":[],"identifier":null,"alignment":null,"formatData":{}}`},
		{"bottom left", model.Cue{Alignment: model.BottomLeft}, `{"start":0.0,"end":0.0,"lines":[],"identifier":null,"alignment":1,"formatData":{}}`},
		{"forced", model.Cue{Forced: true, ID: "7"}, `{"start":0.0,"end":0.0,"lines":[],"identifier":"7","alignment":null,"forced":true,"formatData":{}}`},
		{"negative times", model.Cue{Start: ms(-2500), End: ms(-1)}, `{"start":-2.5,"end":-0.001,"lines":[],"identifier":null,"alignment":null,"formatData":{}}`},
		{
			"last millisecond",
			model.Cue{Start: ms(359_999_999_999), End: ms(359_999_999_999)},
			`{"start":359999999.999,"end":359999999.999,"lines":[],"identifier":null,"alignment":null,"formatData":{}}`,
		},
		{
			"escapes",
			model.Cue{Lines: []string{"a/b <c> & \"d\" \\ \u2028 \x01 \u00e9"}},
			`{"start":0.0,"end":0.0,"lines":["a/b <c> & \"d\" \\ \u2028 \u0001 ` + "\u00e9" + `"],"identifier":null,"alignment":null,"formatData":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCue(tt.cue)
			got := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
			assert.Equal(t, `{"version":1,"metadata":{},"comments":[],"formatData":{},"cues":[`+tt.want+`]}`, got)
		})
	}
}

func TestWriteFormatDataNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value any
		want  string
	}{
		{int64(4), "4"},
		{4.0, "4.0"},
		{math.Copysign(0, -1), "-0.0"},
		{0.1, "0.1"},
		{1e20, "100000000000000000000.0"},
		{1e21, "1e+21"},
		{1e-6, "0.000001"},
		{1e-7, "1e-7"},
		{-1.5e-300, "-1.5e-300"},
		{int64(math.MaxInt64), "9223372036854775807"},
		{nil, "null"},
		{true, "true"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			setFormatData(t, s, "x", model.Field{Key: "v", Value: tt.value})
			got := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
			assert.Equal(t, `{"version":1,"metadata":{},"comments":[],"formatData":{"x":{"v":`+tt.want+`}},"cues":[]}`, got)
			assert.Equal(t, s, mustRead(t, got, false))
		})
	}
}

func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Lines: []string{"<b>Bold</b>"}})
	compact := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
	assert.Equal(t, compact, writeString(t, s, model.CommonWriteOptions{StripTags: true}, stboxjson.WriteOptions{}))
	assert.Equal(t, "\ufeff"+compact, writeString(t, s, model.CommonWriteOptions{BOM: model.BOMAdd}, stboxjson.WriteOptions{}))
	assert.Contains(t, compact, "<b>Bold</b>")
	assert.NotContains(t, compact, "\n")
}

// Ports testTextThatIsNotUtf8ThrowsTheLibraryException and testAnInfiniteValueThrowsWithoutTheEncodingAdvice of
// tests/Formatters/JsonOutputTest.php of subtitle-toolbox-php.
func TestWriteErrors(t *testing.T) {
	t.Parallel()

	const (
		malformed = "JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. Read a file in another encoding with its source encoding."
		infOrNaN  = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
	)
	twoCues := func(second model.Cue) *model.Subtitle {
		s := &model.Subtitle{}
		s.AddCues(model.Cue{Lines: []string{"Fine"}}, second)
		return s
	}
	withData := func(key string, value any) *model.Subtitle {
		s := &model.Subtitle{}
		setFormatData(t, s, "x", model.Field{Key: key, Value: value})
		return s
	}
	tests := []struct {
		name     string
		s        *model.Subtitle
		reason   string
		cueIndex int
	}{
		{"line", twoCues(model.Cue{Start: time.Second, Lines: []string{"Caf\xe9"}}), malformed, 1},
		{"identifier", twoCues(model.Cue{Start: time.Second, ID: "\xff"}), malformed, 1},
		{"metadata value", func() *model.Subtitle { s := &model.Subtitle{}; s.SetMetadata("title", "\xff"); return s }(), malformed, -1},
		{"metadata key", func() *model.Subtitle { s := &model.Subtitle{}; s.SetMetadata("\xff", "x"); return s }(), malformed, -1},
		{"comment", func() *model.Subtitle { s := &model.Subtitle{}; require.NoError(t, s.AddComment("\xff", 0)); return s }(), malformed, -1},
		{"format data key", withData("\xff", "x"), malformed, -1},
		{"format data name", func() *model.Subtitle {
			s := &model.Subtitle{}
			setFormatData(t, s, "\xff", model.Field{Key: "a", Value: "x"})
			return s
		}(), malformed, -1},
		{"infinite file data", withData("value", math.Inf(1)), infOrNaN, -1},
		{"NaN in a list", withData("value", []any{math.NaN()}), infOrNaN, -1},
		{"infinite cue data", twoCues(cueWithData(t, model.Cue{Start: time.Second}, "custom", model.Field{Key: "value", Value: math.Inf(-1)})), infOrNaN, 1},
		{"line before infinite file data", func() *model.Subtitle {
			s := withData("value", math.Inf(1))
			s.AddCue(model.Cue{Lines: []string{"\xff"}})
			return s
		}(), malformed, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			err := stboxjson.Write(&out, tt.s, model.CommonWriteOptions{}, stboxjson.WriteOptions{PrettyPrint: true})
			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			require.ErrorIs(t, err, model.ErrUnwritable)
			assert.Equal(t, model.JSON, uerr.Format)
			assert.Equal(t, tt.reason, uerr.Reason)
			assert.Equal(t, tt.cueIndex, uerr.CueIndex)
			assert.Zero(t, out.Len())
		})
	}
}

type failingWriter struct{}

var errDiskFull = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := stboxjson.Write(failingWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
	require.ErrorIs(t, err, errDiskFull)
}

func TestWritePrettyNests(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	setFormatData(t, s, "x", model.Field{Key: "list", Value: []any{int64(1), []any{}, model.NewData()}})
	got := writeString(t, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{PrettyPrint: true})
	assert.Equal(t, strings.Join([]string{
		"{",
		`    "version": 1,`,
		`    "metadata": {},`,
		`    "comments": [],`,
		`    "formatData": {`,
		`        "x": {`,
		`            "list": [`,
		`                1,`,
		`                [],`,
		`                {}`,
		`            ]`,
		`        }`,
		`    },`,
		`    "cues": []`,
		"}",
		"",
	}, "\n"), got)
}
