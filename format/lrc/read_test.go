package lrc_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testEndTimeIsNextCueStartIncludingCentiseconds, testOffsetMakesLyricsShowEarlier,
// testNegativeOffsetMakesLyricsShowLater, testOffsetDoesNotMakeTimesNegative,
// testLineWithSeveralTimestampsBecomesOneCuePerTimestamp, testTimestampWithoutTextEndsTheLastCue,
// testAcceptsTimestampsWithoutFractionAndWithMilliseconds, testTimestampNeedsADotBeforeTheFraction,
// testTimeTagWithoutTextDoesNotTakeAnIdTagOrATimeTagLine, testWordTimestampsBecomeCoreMarkup,
// testPlainTextIsEscaped and testTextAroundWordTimestampsIsEscaped of tests/Parsers/LyricsParserTest.php of
// subtitle-toolbox-php, then adds rows of its own.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{"end at the next start", "[00:01.00] First\n[00:02.75] Second\n", []cueRow{{time.Second, 2750 * ms, "First"}, {2750 * ms, 7750 * ms, "Second"}}},
		{
			"offset makes lyrics show earlier",
			"[00:12.00] First\n[offset:+500]\n[00:17.20] Second <00:18.00>word\n",
			[]cueRow{{11500 * ms, 16700 * ms, "First"}, {16700 * ms, 21700 * ms, "Second <00:00:17.500>word"}},
		},
		{"negative offset makes lyrics show later", "[offset:-250]\n[00:12.00] First\n", []cueRow{{12250 * ms, 17250 * ms, "First"}}},
		{"offset does not make times negative", "[offset:1000]\n[00:00.50] First\n", []cueRow{{0, 5 * time.Second, "First"}}},
		{"offset 0", "[offset:0]\n[00:00.50] First\n", []cueRow{{500 * ms, 5500 * ms, "First"}}},
		{"only the first offset counts", "[offset:100]\n[offset:900]\n[00:01.00] First\n", []cueRow{{900 * ms, 5900 * ms, "First"}}},
		{"offset tag in any case", "[OFFSET: 200 ]\n[00:01.00] First\n", []cueRow{{800 * ms, 5800 * ms, "First"}}},
		{
			"several time tags",
			"[00:12.00][01:15.30]Chorus\n[00:17.20]Verse\n",
			[]cueRow{{12 * time.Second, 17200 * ms, "Chorus"}, {17200 * ms, 75300 * ms, "Verse"}, {75300 * ms, 80300 * ms, "Chorus"}},
		},
		{"time tag without text ends the last cue", "[00:01.00] First\n[00:03.50]\n", []cueRow{{time.Second, 3500 * ms, "First"}}},
		{
			"without fraction and with milliseconds",
			"[00:01] First\n[00:02.345] Second\n",
			[]cueRow{{time.Second, 2345 * ms, "First"}, {2345 * ms, 7345 * ms, "Second"}},
		},
		{"a dot before the fraction", "[00:12x00] Wrong\n[00:13.00] Right\n", []cueRow{{13 * time.Second, 18 * time.Second, "Right"}}},
		{"a comma before the fraction is no time tag", "[00:12,00] Wrong\n[00:13.00] Right\n", []cueRow{{13 * time.Second, 18 * time.Second, "Right"}}},
		{
			"time tag without text takes no ID tag or time tag line",
			"[00:01.00]\n[ar:Harbour Band]\n[00:02.00]\n[00:03.00]Gulls\n",
			[]cueRow{{3 * time.Second, 8 * time.Second, "Gulls"}},
		},
		{"time tag without text takes no broken time tag", "[00:01.00]\n[00:0x.00]Gulls\n", nil},
		{"text on the next line", "[00:02:35]\nHello\n", []cueRow{{155 * time.Second, 160 * time.Second, "Hello"}}},
		{"a line of spaces is no text", "[00:01.00]\n \t \nHello\n", []cueRow{{time.Second, 6 * time.Second, "Hello"}}},
		{
			"word times become word timestamps",
			"[00:21.10]<00:21.10>Bread <00:21.60>is <61:01.905>warm\n",
			[]cueRow{{21100 * ms, 26100 * ms, "<00:00:21.100>Bread <00:00:21.600>is <01:01:01.905>warm"}},
		},
		{"plain text is escaped", "[00:01.00]I <3 bread & jam\n", []cueRow{{time.Second, 6 * time.Second, "I &lt;3 bread &amp; jam"}}},
		{
			"text around word times is escaped",
			"[00:01.00]<00:01>Fish & <00:01.50>chips <3 <1:2>\n",
			[]cueRow{{time.Second, 6 * time.Second, "<00:00:01.000>Fish &amp; <00:00:01.500>chips &lt;3 &lt;1:2&gt;"}},
		},
		{"tabs and space runs become 1 space", "[00:01.00]\tA  \t b\t\n", []cueRow{{time.Second, 6 * time.Second, "A b"}}},
		{"text after a space between time tags", "[00:01.00] [00:02.00]\n", []cueRow{{time.Second, 6 * time.Second, "[00:02.00]"}}},
		{"CR LF, a lone CR and a BOM", bom + "[00:01.00]A\r\n\r\n[00:02.00]B\r[00:03.00]\r\n", []cueRow{{time.Second, 2 * time.Second, "A"}, {2 * time.Second, 3 * time.Second, "B"}}},
		{"only ID tags", "[ti:Rain]\n[ar:Choir]\n", nil},
		{"lines that are no LRC are ignored", "[00:01.00]A\n[server:main\nport=80\n", []cueRow{{time.Second, 6 * time.Second, "A"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, lenient := range []bool{false, true} {
				s, warnings := mustRead(t, tt.content, lenient)
				assert.Equal(t, tt.want, cueRows(s), "lenient %v", lenient)
				if !lenient {
					assert.Empty(t, warnings)
				}
			}
		})
	}
}

// Rows of the time tag shapes of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestReadTimeTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag  string
		want time.Duration
		ok   bool
	}{
		{"[00:12]", 12 * time.Second, true},
		{"[00:12.5]", 12500 * ms, true},
		{"[00:12.50]", 12500 * ms, true},
		{"[00:12.05]", 12050 * ms, true},
		{"[00:12.500]", 12500 * ms, true},
		{"[00:12.005]", 12005 * ms, true},
		{"[0:01.00]", time.Second, true},
		{"[123:45.67]", 123*time.Minute + 45670*ms, true},
		{"[999:59.999]", 999*time.Minute + 59999*ms, true},
		{"[01:02:03]", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"[1:2:03.5]", time.Hour + 2*time.Minute + 3500*ms, true},
		{"[99:59:59.999]", 99*time.Hour + 59*time.Minute + 59999*ms, true},
		{"[ 00:12.00 ]", 12 * time.Second, true},
		{"[\t00:12.00\t]", 12 * time.Second, true},
		{"[00:12,00]", 0, false},
		{"[1:75:00.00]", 0, false},
		{"[00:61.00]", 0, false},
		{"[00:12.0000]", 0, false},
		{"[1234:45.67]", 0, false},
		{"[100:00:00]", 0, false},
		{"[00:12.]", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.tag+"Text\n", false)
			if !tt.ok {
				assert.Zero(t, s.Len())
				return
			}
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.want, c.Start)
		})
	}
}

// Ports testIdTagsBecomeMetadataOrLrcFormatData, testIdTagKeysAreCaseInsensitiveAndValuesAreTrimmed,
// testInvalidOffsetIsKeptAsFormatData and testIdTagsAndCommentsAreNotEscaped of tests/Parsers/LyricsParserTest.php
// of subtitle-toolbox-php.
func TestReadIDTags(t *testing.T) {
	t.Parallel()

	s := readFile(t, "lrc/with_id_tags.lrc")
	assert.Equal(t, map[string]string{
		model.MetadataAuthor: "Creator of the Songtext",
		model.MetadataTitle:  "Lyrics (song) title",
		model.MetadataAlbum:  "Album where the song is from",
	}, s.AllMetadata())
	assert.Equal(t, map[string]any{"idTags": map[string]any{"by": "Creator of the LRC file", "length": "How long the song is"}}, fileData(t, s))

	s, _ = mustRead(t, "[TI: Morning Train ]\n[Ar:Station Choir]\n[AL:]\n[00:01.00] Text\n", false)
	assert.Equal(t, map[string]string{model.MetadataTitle: "Morning Train", model.MetadataArtist: "Station Choir", model.MetadataAlbum: ""}, s.AllMetadata())

	s, _ = mustRead(t, "[offset:soon]\n[00:12.00] First\n", false)
	assert.Equal(t, []cueRow{{12 * time.Second, 17 * time.Second, "First"}}, cueRows(s))
	assert.Equal(t, map[string]any{"idTags": map[string]any{"offset": "soon"}}, fileData(t, s))

	s, _ = mustRead(t, "[ti:Fish & Chips]\n[re:<Editor>]\n[#:a < b & c]\n[00:01.00]Text\n", false)
	title, _ := s.Metadata(model.MetadataTitle)
	assert.Equal(t, "Fish & Chips", title)
	assert.Equal(t, map[string]any{"idTags": map[string]any{"re": "<Editor>"}}, fileData(t, s))
	assert.Equal(t, []model.Comment{{Text: "a < b & c", BeforeCue: 0}}, s.Comments())

	s, _ = mustRead(t, "[offset:+500]\n[offset:200]\n[00:12.00] First\n", false)
	assert.Nil(t, fileData(t, s))
}

// Ports testRealFileMetadata of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestReadIDTagOrder(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/subsrt-sample.lrc")
	for key, want := range map[string]string{
		model.MetadataTitle:  "Weather (morning) report",
		model.MetadataArtist: "Station choir",
		model.MetadataAlbum:  "Songs from the bakery",
		model.MetadataAuthor: "Writer of the words",
	} {
		got, _ := s.Metadata(key)
		assert.Equal(t, want, got, key)
	}
	d, ok := s.FormatData("lrc")
	require.True(t, ok)
	tags, ok := d.Object("idTags")
	require.True(t, ok)
	assert.Equal(t, []string{"length", "by", "offset", "re", "ve"}, tags.Keys())
}

// Ports testCommentTagsBecomeComments and testCommentTagStaysBeforeItsLineWhenTheLinesAreOutOfTimeOrder of
// tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestReadComments(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[#:Header note]\n[00:01.00] First\n[#:Before second]\n[00:02.00] Second\n", false)
	assert.Equal(t, []model.Comment{{Text: "Header note", BeforeCue: 0}, {Text: "Before second", BeforeCue: 1}}, s.Comments())

	s = readFile(t, realDir+"/own-comment-before-earlier-line.lrc")
	var texts []string
	for _, c := range s.All() {
		texts = append(texts, c.Text())
	}
	assert.Equal(t, []string{"The door opens", "The shop is open", "The shop closes"}, texts)
	assert.Equal(t, []model.Comment{
		{Text: "The door opens before the shop", BeforeCue: 0},
		{Text: "The shop opens at nine", BeforeCue: 1},
		{Text: "End of the shop song", BeforeCue: 3},
	}, s.Comments())
}

// Ports testExceededMinutesIgnoredCue, testExceededSecondsIgnoresCue, testExceededMilliSecondAccuracyIgnoresCue,
// testMissingTextIgnoresCue, testMissingTimestampIgnoresCue, testTimestampWithoutTextEndsThePreviousCue and
// testReadsTimeTagShapesInStrictAndLenientMode of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestReadFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want []cueRow
	}{
		{"exceeded_minutes.lrc", []cueRow{{83450 * ms, 169010 * ms, "First Text"}, {169010 * ms, 174010 * ms, "Second Text"}}},
		{"exceeded_seconds.lrc", []cueRow{{83450 * ms, 225670 * ms, "First Text"}, {225670 * ms, 230670 * ms, "Third Text"}}},
		{"exceeded_centi_accuracy.lrc", []cueRow{{83450 * ms, 225670 * ms, "First Text"}, {225670 * ms, 230670 * ms, "Third Text"}}},
		{"missing_text.lrc", []cueRow{{83450 * ms, 154560 * ms, "First Text"}, {225670 * ms, 230670 * ms, "Third Text"}}},
		{"missing_timestamps.lrc", []cueRow{{83450 * ms, 225670 * ms, "First Text"}, {225670 * ms, 230670 * ms, "Third Text"}}},
		{"spaces_in_time_tags.lrc", []cueRow{{12 * time.Second, 15500 * ms, "The boats come home at dusk"}, {15500 * ms, 20500 * ms, "Gulls follow every one"}}},
		{"hours_with_text_on_next_line.lrc", []cueRow{{155 * time.Second, 171 * time.Second, "The boats come home at dusk"}, {171 * time.Second, 176 * time.Second, "Gulls follow every one"}}},
		{"one_digit_fraction.lrc", []cueRow{{12500 * ms, 15250 * ms, "The boats come home at dusk"}, {15250 * ms, 20250 * ms, "Gulls follow every one"}}},
		{"one_digit_minutes.lrc", []cueRow{{time.Second, 65 * time.Second, "The boats come home at dusk"}, {65 * time.Second, 70 * time.Second, "Gulls follow every one"}}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			content := string(testfiles.Read(t, "lrc/"+tt.file))
			for _, lenient := range []bool{false, true} {
				s, warnings := mustRead(t, content, lenient)
				assert.Equal(t, tt.want, cueRows(s))
				if lenient && tt.file != "exceeded_minutes.lrc" && tt.file != "exceeded_seconds.lrc" && tt.file != "exceeded_centi_accuracy.lrc" {
					assert.Empty(t, warnings)
				}
			}
		})
	}
}

// Ports testLastCueLastsFiveSecondsByDefault and testLastCueDurationIsAnOption of
// tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php, and the last cue rules of
// https://github.com/yama6a/subtitle-toolbox-go/issues/14.
func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		last    *time.Duration
		want    time.Duration
	}{
		{"5 s by default", "[00:01.00] First\n", nil, 6 * time.Second},
		{"2.5 s", "[00:01.00] First\n", new(2500 * ms), 3500 * ms},
		{"0 keeps the cue", "[00:01.00] First\n", new(time.Duration(0)), time.Second},
		{"a duration beyond int64 with the start", "[00:01.00] First\n", new(time.Duration(math.MaxInt64)), timecode.MaxValidTime},
		{"the end stops at the time limit", "[offset:-359999998000]\n[00:00.00] First\n", nil, timecode.MaxValidTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(tt.content, model.CommonReadOptions{LastCueDuration: tt.last})
			require.NoError(t, err)
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.want, c.End)
		})
	}
}

// Rows of the end line rule of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestReadEndLine(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[00:01.00] First\n[00:02.50]\n[00:04.00] Second\n[00:05.00] Third\n", false)
	var endLines []bool
	for _, c := range s.All() {
		d, _ := c.FormatData("lrc")
		endLine, _ := d.Bool("endLine")
		endLines = append(endLines, endLine)
	}
	assert.Equal(t, []bool{true, false, false}, endLines)
	assert.Equal(t, []cueRow{{time.Second, 2500 * ms, "First"}, {4 * time.Second, 5 * time.Second, "Second"}, {5 * time.Second, 10 * time.Second, "Third"}}, cueRows(s))
}

// Ports testSpaceBeforeAWordTimestampAtTheLineEndIsDropped and testWordTimesWithoutSpacesAreWrittenWithoutSpaces
// of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestReadSpacedWordTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		text    string
		spaced  bool
	}{
		{"spaces on both sides", "[00:01.00] <00:01.00> Fish <00:01.50> chips <00:02.00>\n", "<00:00:01.000>Fish <00:00:01.500>chips<00:00:02.000>", true},
		{"no spaces", "[00:01.00]<00:01.00>Fish <00:01.50>chips\n", "<00:00:01.000>Fish <00:00:01.500>chips", false},
		{"only a space before the last word time", "[00:01.00]Fish <00:01.50>\n", "Fish<00:00:01.500>", true},
		{"a tab after a word time", "[00:01.00]<00:01.00>\tFish\n", "<00:00:01.000>Fish", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, false)
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.text, c.Text())
			var want map[string]any
			if tt.spaced {
				want = map[string]any{"spacedWordTimes": true}
			}
			assert.Equal(t, want, fileData(t, s))
		})
	}

	s := readFile(t, "karaoke/hebrew.lrc")
	d, _ := s.FormatData("lrc")
	_, ok := d.Bool("spacedWordTimes")
	assert.False(t, ok)
}

// Ports testTextThatIsNotUtf8KeepsItsBytes of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestReadInvalidUTF8(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "[00:01.00]caf\xE9 & tea\n", false)
	assert.Equal(t, []cueRow{{time.Second, 6 * time.Second, "caf\xE9 &amp; tea"}}, cueRows(s))
	assert.Empty(t, warnings)
	assert.Equal(t, bom+"[00:01.00] caf\xE9 & tea\n", write(t, s, model.CommonWriteOptions{}))

	_, warnings = mustRead(t, "[00:01.00]caf\xE9 & tea\n", true)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
}

// Rows of the errors and warnings of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestReadLenientWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []warningRow
		cues    int
	}{
		{
			"broken time tags",
			"[ti:Walk]\n\n[00:01.00]A\n[00:1x.00]B\n  [1:75:00.00]\tC\n[00:12.0000]D\n[00:02.00]E\n",
			[]warningRow{
				{`The line "[00:1x.00]B" has a time tag that is not valid.`, 4, 2, []string{"[00:1x.00]B"}, model.Skipped},
				{`The line "[1:75:00.00] C" has a time tag that is not valid.`, 5, 3, []string{"[1:75:00.00] C"}, model.Skipped},
				{`The line "[00:12.0000]D" has a time tag that is not valid.`, 6, 4, []string{"[00:12.0000]D"}, model.Skipped},
			},
			2,
		},
		{
			"a huge negative offset",
			"[offset:-99999999999999999999]\n[00:01.00]A\n[ti:x]\n[00:12.00]B <00:13.00>c\n",
			[]warningRow{
				{`The time "[00:01.00]" is not below 100000 hours.`, model.NoLine, 1, []string{"[00:01.00]A"}, model.Skipped},
				{`The time "<00:13.00>" is not below 100000 hours.`, model.NoLine, 3, []string{"[00:12.00]B <00:13.00>c"}, model.Skipped},
			},
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.want, warningRows(warnings))
			assert.Equal(t, tt.cues, s.Len())
		})
	}
}

// Rows of the huge offset rule of https://github.com/yama6a/subtitle-toolbox-go/issues/32.
func TestReadHugeOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		msg     string
		block   int
		line    string
	}{
		{"beyond int64", "[offset:-99999999999999999999]\n[00:12.00]x\n", `The time "[00:12.00]" is not below 100000 hours.`, 1, "[00:12.00]x"},
		{"at the limit", "[offset:-359999988000]\n[00:12.00]x\n", `The time "[00:12.00]" is not below 100000 hours.`, 1, "[00:12.00]x"},
		{"a word time", "[offset:-359999988000]\n[00:01.00]x <00:12.00>y\n", `The time "<00:12.00>" is not below 100000 hours.`, 1, "[00:01.00]x <00:12.00>y"},
		{"spaces in the tag", "[offset:-359999988000]\n[ 00:12.00\t]x\n", `The time "[ 00:12.00 ]" is not below 100000 hours.`, 1, "[ 00:12.00 ]x"},
		{"the text on the next line", "[offset:-359999988000]\n[00:12.00]\nx\n", `The time "[00:12.00]" is not below 100000 hours.`, 2, "[00:12.00]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := read(tt.content, model.CommonReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{Format: model.LRC, Msg: tt.msg}, *perr)

			s, warnings := mustRead(t, tt.content, true)
			assert.Zero(t, s.Len())
			assert.Equal(t, []warningRow{{tt.msg, model.NoLine, tt.block, []string{tt.line}, model.Skipped}}, warningRows(warnings))
		})
	}

	s, _ := mustRead(t, "[offset:99999999999999999999]\n[00:12.00]x\n", false)
	assert.Equal(t, []cueRow{{0, 5 * time.Second, "x"}}, cueRows(s))
	s, _ = mustRead(t, "[offset:-359999987999]\n[00:12.00]x\n", false)
	assert.Equal(t, []cueRow{{timecode.MaxValidTime, timecode.MaxValidTime, "x"}}, cueRows(s))
}

// Ports the "text but no cues" rule of https://github.com/yama6a/subtitle-toolbox-go/issues/14 for LRC.
func TestReadTextWithoutCues(t *testing.T) {
	t.Parallel()

	content := "[ti:Harbour walk]\n\n  The ferry  leaves at nine.\n"
	_, _, err := read(content, model.CommonReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, model.ParseError{Format: model.LRC, Line: 3, Msg: "The file has text but no cues."}, *perr)

	s, warnings := mustRead(t, content, true)
	assert.Zero(t, s.Len())
	assert.Equal(t, []warningRow{{"The file has text but no cues.", 3, model.NoBlock, nil, model.Skipped}}, warningRows(warnings))
	title, _ := s.Metadata(model.MetadataTitle)
	assert.Equal(t, "Harbour walk", title)

	for _, content := range []string{"[00:1x.00]Broken\n", "[ti:x]\n[#:note]\n", "[00:01.00]\n"} {
		s, warnings := mustRead(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
	}
}

func TestReadBlankContentGivesNoCues(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", " \n\r\n", bom} {
		s, warnings := mustRead(t, content, false)
		assert.Zero(t, s.Len())
		assert.Empty(t, warnings)
	}
}
