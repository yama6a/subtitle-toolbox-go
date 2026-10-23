package srt_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const bom = "\ufeff"

// Ports testSubtitleIsFormattedCorrectly, testUnsupportedXmlTagsAreStrippedAway and
// testAllXmlTagsAreStrippedAwayIfOptionIsSet of tests/Formatters/SubRipFormatterTest.php of subtitle-toolbox-php.
func TestWriteFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, want string
		stripTags      bool
	}{
		{name: "valid file", in: "srt/valid.srt", want: "srt/valid.srt"},
		{name: "tags that SubRip does not show go", in: "srt/strip_xml.srt", want: "srt/valid.srt"},
		{name: "all tags go", in: "srt/strip_xml.srt", want: "srt/all_tags_stripped.srt", stripTags: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, string(testfiles.Read(t, tt.in)), false)
			assert.Equal(t, string(testfiles.Read(t, tt.want)), write(t, s, model.CommonWriteOptions{StripTags: tt.stripTags}))
		})
	}
}

// Ports the API tests of tests/Formatters/SubRipFormatterTest.php of subtitle-toolbox-php.
func TestWriteCues(t *testing.T) {
	t.Parallel()

	cue := func(start, end time.Duration, lines ...string) model.Cue {
		return model.Cue{Start: start, End: end, Lines: lines}
	}
	aligned := func(c model.Cue, a model.Alignment) model.Cue {
		c.Alignment = a
		return c
	}
	withCoordinates := model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"Text"}}
	require.NoError(t, withCoordinates.SetFormatData("srt", model.NewData(model.Field{Key: "coordinates", Value: model.NewData(
		model.Field{Key: "x1", Value: int64(10)}, model.Field{Key: "x2", Value: int64(20)},
		model.Field{Key: "y1", Value: int64(30)}, model.Field{Key: "y2", Value: int64(40)},
	)})))
	require.NoError(t, withCoordinates.SetFormatData("vtt", model.NewData(model.Field{Key: "align", Value: "start"})))

	tests := []struct {
		name      string
		cues      []model.Cue
		stripTags bool
		want      string
	}{
		{
			name: "the strike tag stays and other tags go",
			cues: []model.Cue{cue(time.Second, 2*time.Second, "<s>struck</s> <c.loud>plain</c>")},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\n<s>struck</s> plain\n",
		},
		{
			name:      "every tag goes",
			cues:      []model.Cue{cue(time.Second, 2*time.Second, "<s>struck</s> <c.loud>plain</c>")},
			stripTags: true,
			want:      bom + "1\n00:00:01,000 --> 00:00:02,000\nstruck plain\n",
		},
		{
			name: "bottom center writes no alignment tag",
			cues: []model.Cue{cue(time.Second, 2*time.Second, "Bottom center"), cue(3*time.Second, 4*time.Second, "Default")},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nBottom center\n\n2\n00:00:03,000 --> 00:00:04,000\nDefault\n",
		},
		{
			name: "a line that becomes empty drops out",
			cues: []model.Cue{cue(time.Second, 2*time.Second, "Before", "<c.loud></c>", "After")},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nBefore\nAfter\n",
		},
		{
			name: "entities are decoded",
			cues: []model.Cue{cue(time.Second, 2*time.Second, "I &lt;3 bread &amp; jam", "<i>Salt &amp; pepper</i> 2 &gt; 1")},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nI <3 bread & jam\n<i>Salt & pepper</i> 2 > 1\n",
		},
		{
			name:      "entities are decoded without tags",
			cues:      []model.Cue{cue(time.Second, 2*time.Second, "I &lt;3 bread &amp; jam", "<i>Salt &amp; pepper</i> 2 &gt; 1")},
			stripTags: true,
			want:      bom + "1\n00:00:01,000 --> 00:00:02,000\nI <3 bread & jam\nSalt & pepper 2 > 1\n",
		},
		{
			name: "a negative start writes as 0",
			cues: []model.Cue{cue(-500*time.Millisecond, time.Second, "Early")},
			want: bom + "1\n00:00:00,000 --> 00:00:01,000\nEarly\n",
		},
		{
			name: "a cue without text and with an alignment",
			cues: []model.Cue{aligned(cue(time.Second, 2*time.Second), model.TopCenter), cue(3*time.Second, 4*time.Second)},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\n{\\an8}\n\n2\n00:00:03,000 --> 00:00:04,000\n",
		},
		{
			name: "the alignment tag goes before the first line",
			cues: []model.Cue{aligned(cue(time.Second, 2*time.Second, "<i>The train</i> leaves", "soon"), model.MiddleRight)},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\n{\\an6}<i>The train</i> leaves\nsoon\n",
		},
		{
			name: "coordinates and no WebVTT settings",
			cues: []model.Cue{withCoordinates},
			want: bom + "1\n00:00:01,000 --> 00:00:04,000 X1:10 X2:20 Y1:30 Y2:40\nText\n",
		},
		{
			name: "more than 99 hours and fewer than 1000",
			cues: []model.Cue{cue(100*time.Hour, 999*time.Hour+time.Millisecond, "Late")},
			want: bom + "1\n100:00:00,000 --> 999:00:00,001\nLate\n",
		},
		{
			name: "ruby as text, color classes as font tags, voices and timestamps go",
			cues: []model.Cue{cue(time.Second, 2*time.Second, "<ruby>base<rt>kan</rt></ruby> <c.yellow>Hi</c> <v Joe>you<00:00:01.500> there</v>")},
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nbase (kan) <font color=\"#ffff00\">Hi</font> you there\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(t, tt.cues...), model.CommonWriteOptions{StripTags: tt.stripTags}))
		})
	}
}

func TestWriteOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(t, model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"One", "Two"}})
	tests := []struct {
		name   string
		common model.CommonWriteOptions
		want   string
	}{
		{"default", model.CommonWriteOptions{}, bom + "1\n00:00:01,000 --> 00:00:02,000\nOne\nTwo\n"},
		{"CR LF", model.CommonWriteOptions{LineEnding: model.CRLF}, bom + "1\r\n00:00:01,000 --> 00:00:02,000\r\nOne\r\nTwo\r\n"},
		{"no BOM", model.CommonWriteOptions{BOM: model.BOMRemove}, "1\n00:00:01,000 --> 00:00:02,000\nOne\nTwo\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, s, tt.common))
		})
	}
}

func TestWriteEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, bom, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
	assert.Empty(t, write(t, &model.Subtitle{}, model.CommonWriteOptions{BOM: model.BOMRemove}))
}

// Ports testTimestampsAreWrittenInTheStandardForm, testCoordinatesAreWrittenBack, testAlignmentIsWrittenAtTheStartOfTheFirstLine,
// testTextWithLessThanAndAmpersandSurvivesARoundTrip, testLineThatIsNotUtf8KeepsItsBytes and testFormatterWritesTheArrowWithSpaces
// of tests/Formatters/SubRipFormatterTest.php and tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		lenient   bool
		stripTags bool
		want      string
	}{
		{
			name: "times in the standard form",
			in:   "1\n0:00:01.5 --> 0:00:02.25\nText\n",
			want: bom + "1\n00:00:01,500 --> 00:00:02,250\nText\n",
		},
		{
			name: "coordinates",
			in:   bom + "1\n00:00:01,000 --> 00:00:04,000 X1:100 X2:600 Y1:40 Y2:80\nText\n",
			want: bom + "1\n00:00:01,000 --> 00:00:04,000 X1:100 X2:600 Y1:40 Y2:80\nText\n",
		},
		{
			name: "the alignment tag moves to the start",
			in:   "1\n00:00:01,000 --> 00:00:04,000\n<i>The train</i> {\\an8}leaves\nsoon\n",
			want: bom + "1\n00:00:01,000 --> 00:00:04,000\n{\\an8}<i>The train</i> leaves\nsoon\n",
		},
		{
			name:      "the alignment tag without other tags",
			in:        "1\n00:00:01,000 --> 00:00:04,000\n<i>The train</i> {\\an8}leaves\nsoon\n",
			stripTags: true,
			want:      bom + "1\n00:00:01,000 --> 00:00:04,000\n{\\an8}The train leaves\nsoon\n",
		},
		{
			name: "less than and ampersand",
			in:   bom + "1\n00:00:01,000 --> 00:00:02,000\nI <3 bread & jam\n",
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nI <3 bread & jam\n",
		},
		{
			name: "entities in the file",
			in:   "1\n00:00:01,000 --> 00:00:02,000\nAT&amp T &lt;b&gt;\n",
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\nAT&amp T &lt;b&gt;\n",
		},
		{
			name: "invalid UTF-8 keeps its bytes",
			in:   bom + "1\n00:00:01,000 --> 00:00:02,000\ncaf\xe9 & <i>cr\xe8me</i>\n",
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\ncaf\xe9 & <i>cr\xe8me</i>\n",
		},
		{
			name:    "the arrow with spaces",
			in:      "1\n00:00:01,000 ---> 00:00:02,000\nText\n",
			lenient: true,
			want:    bom + "1\n00:00:01,000 --> 00:00:02,000\nText\n",
		},
		{
			name: "SSA toggles as core tags",
			in:   "1\n00:00:01,000 --> 00:00:02,000\n{\\an9\\b1}Bold {\\pos(1,2)}text\n",
			want: bom + "1\n00:00:01,000 --> 00:00:02,000\n{\\an9}<b>Bold {\\pos(1,2)}text</b>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.in, tt.lenient)
			assert.Equal(t, tt.want, write(t, s, model.CommonWriteOptions{StripTags: tt.stripTags}))
		})
	}
}

type failingWriter struct{}

var errDiskFull = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

func TestWriteError(t *testing.T) {
	t.Parallel()

	err := srt.Write(failingWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.ErrorIs(t, err, errDiskFull)
}

func TestWriteThenReadKeepsTheCues(t *testing.T) {
	t.Parallel()

	s := subtitle(t,
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<b>One</b> &amp; two"}, Alignment: model.TopLeft},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second},
	)
	var out bytes.Buffer
	require.NoError(t, srt.Write(&out, s, model.CommonWriteOptions{}, srt.WriteOptions{}))
	back, _ := mustRead(t, out.String(), false)
	assert.Equal(t, cueRows(s), cueRows(back))
	assert.Equal(t, model.TopLeft, firstCue(t, back).Alignment)
}

func TestWriteStaysAtTheLargestTimeThatTheReaderReads(t *testing.T) {
	t.Parallel()

	const hour = time.Hour
	tests := []struct {
		name       string
		start, end time.Duration
		want       string
	}{
		{"999:59:59.999 stays", 0, 1000*hour - time.Millisecond, "00:00:00,000 --> 999:59:59,999\n"},
		{"1000:00:00.000 clamps", 0, 1000 * hour, "00:00:00,000 --> 999:59:59,999\n"},
		{"99999:59:59.999 clamps", 0, 100000*hour - time.Millisecond, "00:00:00,000 --> 999:59:59,999\n"},
		{"a start and an end that clamp", 5000 * hour, 5001 * hour, "999:59:59,999 --> 999:59:59,999\n"},
		{"998:00:00.500 stays", 0, 998*hour + 500*time.Millisecond, "00:00:00,000 --> 998:00:00,500\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := write(t, subtitle(t, model.Cue{Start: tt.start, End: tt.end, Lines: []string{"x"}}), model.CommonWriteOptions{})
			assert.Contains(t, got, tt.want)
			_, _, err := read(t, got, false)
			require.NoError(t, err)
		})
	}
}
