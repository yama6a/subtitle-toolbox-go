package lrc_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testCentisecondsRoundUpIntoTheNextSecond, testWordTimestampsAreWrittenAsLrcWordTimestamps,
// testEntitiesFromOtherFormatsAreDecoded and testSubtitleWithoutMetadataGetsNoIdTags of
// tests/Formatters/LyricsFormatterTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cues []model.Cue
		want string
	}{
		{"centiseconds round up into the next second", []model.Cue{{Start: 59996 * ms, End: 61 * time.Second, Lines: []string{"Text"}}}, "[01:00.00] Text\n"},
		{"half a centisecond rounds up", []model.Cue{{Start: 1005 * ms, Lines: []string{"Text"}}}, "[00:01.01] Text\n"},
		{"hours fold into the minutes", []model.Cue{{Start: 3661905 * ms, Lines: []string{"Text"}}}, "[61:01.91] Text\n"},
		{"a negative time writes as 0", []model.Cue{{Start: -2 * time.Second, Lines: []string{"Text"}}}, "[00:00.00] Text\n"},
		{
			"word timestamps",
			[]model.Cue{{Start: 21100 * ms, End: 25 * time.Second, Lines: []string{"<i><00:00:21.100>Bread</i> <00:00:21.600>is <01:01:01.905>warm"}}},
			"[00:21.10] <00:21.10>Bread <00:21.60>is <61:01.91>warm\n",
		},
		{
			"entities from other formats are decoded",
			[]model.Cue{
				{Start: time.Second, End: 2 * time.Second, Lines: []string{"I &lt;3 bread &amp; jam"}},
				{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"<b><00:00:03.000>Fish</b> &amp; <00:00:03.500>chips"}},
			},
			"[00:01.00] I <3 bread & jam\n[00:03.00] <00:03.00>Fish & <00:03.50>chips\n",
		},
		{"lines join with a space", []model.Cue{{Start: time.Second, Lines: []string{"Slow", "<i>river</i>"}}}, "[00:01.00] Slow river\n"},
		{"ruby as text", []model.Cue{{Lines: []string{"<ruby>Tokyo<rt>to</rt></ruby>"}}}, "[00:00.00] Tokyo (to)\n"},
		{"a cue without text", []model.Cue{{Start: time.Second}}, "[00:01.00] \n"},
		{
			"a word timestamp beyond time.Duration is a tag",
			[]model.Cue{{Start: time.Second, Lines: []string{"<2562047:00:00.000>Fish <00:00:01.500>chips"}}},
			"[00:01.00] Fish <00:01.50>chips\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, bom+tt.want, write(t, subtitle(tt.cues...), model.CommonWriteOptions{}))
		})
	}
}

// Ports testCommentsAreWrittenInPlace of tests/Formatters/LyricsFormatterTest.php of subtitle-toolbox-php.
func TestWriteCommentsAndIDTags(t *testing.T) {
	t.Parallel()

	s := subtitle(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"First"}},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"Second"}},
	)
	s.SetMetadata(model.MetadataArtist, "Station Choir")
	require.NoError(t, s.AddComment("Before second", 1))
	require.NoError(t, s.AddComment("Two\nlines", 2))
	assert.Equal(t, bom+"[ar:Station Choir]\n[00:01.00] First\n[#:Before second]\n[00:03.00] Second\n[#:Two lines]\n", write(t, s, model.CommonWriteOptions{}))

	s = subtitle(model.Cue{Start: time.Second, Lines: []string{"Text"}})
	s.SetMetadata(model.MetadataAuthor, "Jane\n Doe ")
	s.SetMetadata(model.MetadataTitle, "")
	s.SetMetadata(model.MetadataLanguage, "en")
	tags := model.NewData(model.Field{Key: "by", Value: "Editor"}, model.Field{Key: "ti", Value: "Other"}, model.Field{Key: "au", Value: "Other"})
	require.NoError(t, s.SetFormatData("lrc", model.NewData(model.Field{Key: "idTags", Value: tags})))
	require.NoError(t, s.AddComment("Last", 5))
	assert.Equal(t, bom+"[ti:]\n[au:Jane Doe]\n[by:Editor]\n[00:01.00] Text\n[#:Last]\n", write(t, s, model.CommonWriteOptions{}))
}

// Rows of the ID tag filter of https://github.com/yama6a/subtitle-toolbox-go/issues/32. A stored tag that would read
// back as something else is not written.
func TestWriteSkipsIDTagsThatReadBackAsOther(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"an integer offset", model.NewData(model.Field{Key: "offset", Value: "500"}), ""},
		{"an integer offset in any case with spaces", model.NewData(model.Field{Key: "OFFSET", Value: " -20\n"}), ""},
		{"an offset that is no integer", model.NewData(model.Field{Key: "offset", Value: "soon"}), "[offset:soon]\n"},
		{"a key with a line end", model.NewData(model.Field{Key: "a]\n[00:05.00]Injected\n[b", Value: "x"}), ""},
		{"a key with a space", model.NewData(model.Field{Key: "my tag", Value: "y"}), ""},
		{"a key of digits", model.NewData(model.Field{Key: "0", Value: "12"}), ""},
		{"a list", []any{"12"}, ""},
		{"a key with digits and _", model.NewData(model.Field{Key: "x_1", Value: "z"}), "[x_1:z]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(model.Cue{Start: time.Second, End: 20 * time.Second, Lines: []string{"First"}})
			require.NoError(t, s.SetFormatData("lrc", model.NewData(model.Field{Key: "idTags", Value: tt.value})))
			out := write(t, s, model.CommonWriteOptions{})
			assert.Equal(t, bom+tt.want+"[00:01.00] First\n", out)
			back, _, err := read(out, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, []cueRow{{time.Second, 6 * time.Second, "First"}}, cueRows(back))
		})
	}
}

// Rows of the 999:59.99 limit of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestWriteClampsAtTheLargestReadableTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start time.Duration
		want  string
	}{
		{"999:59.99", 59999990 * ms, "[999:59.99] w <999:59.99>x\n[999:59.99]\n"},
		{"999:59.995", 59999995 * ms, "[999:59.99] w <999:59.99>x\n[999:59.99]\n"},
		{"20 h", 20 * time.Hour, "[999:59.99] w <999:59.99>x\n[999:59.99]\n"},
		{"1004.6 ms", 1004600 * time.Microsecond, "[00:01.01] w <00:01.01>x\n[999:59.99]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Start: tt.start, End: 21 * time.Hour, Lines: []string{"w <" + markupcore.CoreTimestamp(tt.start) + ">x"}}
			require.NoError(t, c.SetFormatData("lrc", model.NewData(model.Field{Key: "endLine", Value: true})))
			out := write(t, subtitle(c), model.CommonWriteOptions{BOM: model.BOMRemove})
			assert.Equal(t, tt.want, out)
			back, _, err := read(out, model.CommonReadOptions{})
			require.NoError(t, err)
			require.Equal(t, 1, back.Len())
		})
	}
}

// Ports testSubtitleIsFormattedCorrectly, testUnsupportedXmlTagsAreStrippedAway,
// testTimestampWithoutTextIsWrittenBack, testFullExampleRoundTrip and testEscapedTextRoundTripsByteForByte of
// tests/Formatters/LyricsFormatterTest.php, and testValidLrcFileParses and testKeepsIdTags of
// tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestWriteAfterRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"valid.lrc", string(testfiles.Read(t, "lrc/valid.lrc")), string(testfiles.Read(t, "lrc/valid.lrc"))},
		{"with_id_tags.lrc", string(testfiles.Read(t, "lrc/with_id_tags.lrc")), string(testfiles.Read(t, "lrc/with_id_tags_formatted.lrc"))},
		{"tags are text", string(testfiles.Read(t, "lrc/strip_xml.lrc")), string(testfiles.Read(t, "lrc/strip_xml.lrc"))},
		{"end lines", "[00:01.00] First\n[00:02.50]\n[00:04.00] Second\n", bom + "[00:01.00] First\n[00:02.50]\n[00:04.00] Second\n"},
		{
			"full example",
			"[ti:Morning Train]\n[ar:Station Choir]\n[offset:+500]\n" +
				"[00:12.00][01:15.30]The train leaves the station at seven\n" +
				"[00:17.20]\n" +
				"[00:21.10]<00:21.10>Bread <00:21.60>is <00:21.90>warm\n",
			bom + "[ti:Morning Train]\n[ar:Station Choir]\n" +
				"[00:11.50] The train leaves the station at seven\n" +
				"[00:16.70]\n" +
				"[00:20.60] <00:20.60>Bread <00:21.10>is <00:21.40>warm\n" +
				"[01:14.80] The train leaves the station at seven\n",
		},
		{
			"escaped text round trips byte for byte",
			bom + "[ti:Fish & Chips]\n[#:a < b & c]\n[00:01.00] I <3 bread & jam\n[00:03.00] <00:03.00>Fish & <00:03.50>chips &amp; <1:2>\n",
			bom + "[ti:Fish & Chips]\n[#:a < b & c]\n[00:01.00] I <3 bread & jam\n[00:03.00] <00:03.00>Fish & <00:03.50>chips &amp; <1:2>\n",
		},
		{
			"spaced word times",
			"[00:01.00] <00:01.00> Fish <00:01.50> chips <00:02.00>\n",
			bom + "[00:01.00] <00:01.00> Fish <00:01.50> chips <00:02.00>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.in, false)
			assert.Equal(t, tt.want, write(t, s, model.CommonWriteOptions{}))
		})
	}
}

// Ports testWordTimesWithoutSpacesAreWrittenWithoutSpaces of tests/Parsers/LyricsParserTest.php of
// subtitle-toolbox-php. The Hebrew text stays in testdata, so the test compares with the file.
func TestWriteWordTimesWithoutSpaces(t *testing.T) {
	t.Parallel()

	file := string(testfiles.Read(t, "karaoke/hebrew.lrc"))
	assert.Equal(t, file, write(t, readFile(t, "karaoke/hebrew.lrc"), model.CommonWriteOptions{}))
	s := subtitle(model.Cue{Start: 3 * time.Second, End: 5 * time.Second, Lines: []string{"<00:00:03.000>Slow <00:00:03.550>river"}})
	assert.Equal(t, bom+"[00:03.00] <00:03.00>Slow <00:03.55>river\n", write(t, s, model.CommonWriteOptions{}))
}

// Rows of the spaced word time rule of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestWriteSpacedWordTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want string
	}{
		{"<00:00:01.000>Fish <00:00:01.500>chips<00:00:02.000>", "<00:01.00> Fish <00:01.50> chips <00:02.00>"},
		{"<00:00:01.000> Fish<00:00:02.000>", "<00:01.00> Fish <00:02.00>"},
		{"Fish <00:00:02.000>", "Fish <00:02.00>"},
		{"<00:00:02.000>", "<00:02.00>"},
		{"<i>Fish</i><00:00:02.000><b></b>", "Fish <00:02.00>"},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()

			s := subtitle(model.Cue{Start: time.Second, Lines: []string{tt.line}})
			require.NoError(t, s.SetFormatData("lrc", model.NewData(model.Field{Key: "spacedWordTimes", Value: true})))
			assert.Equal(t, bom+"[00:01.00] "+tt.want+"\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestWriteEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, bom, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
}

func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{End: time.Second, Lines: []string{"Hello"}})
	assert.Equal(t, bom+"[00:00.00] Hello\r\n", write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF}))
	assert.Equal(t, "[00:00.00] Hello\n", write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := lrc.Write(failingWriter{}, subtitle(model.Cue{Lines: []string{"x"}}), model.CommonWriteOptions{}, lrc.WriteOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}
