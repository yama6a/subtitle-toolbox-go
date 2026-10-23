package stbox_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

type detectTest struct {
	name    string
	content string
	want    stbox.Format
	later   string // the format that PHP detects, for a row whose format has no signature in this release
}

// detectTests ports the signatures of tests/FormatDetectorTest.php of subtitle-toolbox-php, then adds rows of its own.
func detectTests() []detectTest {
	return []detectTest{
		{name: "WebVTT with title", content: "WEBVTT - Weather report\n\n00:01.000 --> 00:02.000\nRain\n", want: stbox.WebVTT},
		{name: "WebVTT header only", content: "WEBVTT", want: stbox.WebVTT},
		{name: "SubRip with dot", content: "1\n00:00:01.000 --> 00:00:04.000\nHello\n", want: stbox.SubRip},
		{name: "SubRip without cue number", content: "00:00:01,000 --> 00:00:04,000\nHello\n", want: stbox.SubRip},
		{name: "SubRip after five lines", content: "a\nb\nc\nd\n\n 1\n00:00:01,000 --> 00:00:04,000\nHello\n", want: stbox.SubRip},
		{name: "SSA header", content: "[Script Info]\r\nScriptType: v4.00\r\n", want: stbox.ASS},
		{name: "ASS header in lower case", content: "[script info]\n", want: stbox.ASS},
		{name: "ASS header with spaces after it", content: "[Script Info] \t\nTitle: Bakery\n", want: stbox.ASS},
		{name: "ASS header only", content: "[Script Info]", want: stbox.ASS},
		{name: "ASS before an LRC tag", content: "[Script Info]\n[ti:Morning Train]\n", want: stbox.ASS},
		{name: "JSON with cues first", content: `{"cues": [], "metadata": {"title": "version"}, "version": 1}`, want: stbox.JSON},
		{name: "JSON with spaces", content: "{\n  \"version\" : 1 ,\n  \"cues\" : [ ]\n}\n", want: stbox.JSON},
		{name: "TTML with XML declaration", content: "<?xml version=\"1.0\"?>\n<!-- made by hand -->\n<tt xmlns=\"http://www.w3.org/ns/ttml\"/>", want: stbox.TTML},
		{name: "TTML with prefixed root", content: `<tt:tt xmlns:tt="http://www.w3.org/ns/ttml"></tt:tt>`, want: stbox.TTML},
		{name: "TTML with DOCTYPE", content: "<!DOCTYPE tt>\n<tt>\n<body/></tt>", want: stbox.TTML},
		{name: "DFXP 2006/04", content: "<tt xmlns=\"http://www.w3.org/2006/04/ttaf1\" xml:lang=\"en\">\n<body/></tt>", want: stbox.TTML},
		{
			name:    "iTT detects as TTML",
			content: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<tt xmlns=\"http://www.w3.org/ns/ttml\" xmlns:ttp=\"http://www.w3.org/ns/ttml#parameter\" ttp:timeBase=\"smpte\" ttp:frameRate=\"24\">",
			want:    stbox.TTML,
		},
		{name: "JSON with a segments key", content: `{"version": 1, "formatData": {"x": {"segments": [1]}}, "cues": []}`, want: stbox.JSON},
		{name: "JSON with an events list", content: `{"version": 1, "formatData": {"x": {"events": [{"tStartMs": 0}]}}, "cues": []}`, want: stbox.JSON},
		{name: "JSON with a chapters list", content: `{"version": 1, "formatData": {"chapters": {"chapters": []}}, "cues": []}`, want: stbox.JSON},
		{
			name:    "JSON with podcast segments",
			content: `{"version": 1, "formatData": {"x": {"segments": [{"startTime": 0, "body": ""}]}}, "cues": []}`,
			want:    stbox.JSON,
		},

		{name: "SAMI in lower case", content: "<sami><body></body></sami>", want: stbox.SAMI},
		{name: "SAMI after a line end", content: "<SAMI>\n<HEAD>\n", want: stbox.SAMI},
		{name: "SAMI after the XML prolog", content: "<?xml version=\"1.0\"?>\n<!-- made by hand -->\n<!DOCTYPE sami>\n<Sami>", want: stbox.SAMI},
		{name: "MPSub with FORMAT first", content: "FORMAT=25\n\n0 50\nHello\n", later: "MPSub"},
		{name: "MPSub with header comment", content: "TITLE=Bakery\nFORMAT=TIME   # seconds\n\n1 2\nHello\n", later: "MPSub"},
		{name: "MicroDVD with fps line", content: "{1}{1}25\n{24}{72}Hello\n", later: "MicroDVD"},
		{name: "MicroDVD without end frame", content: "{24}{}Hello\n", later: "MicroDVD"},
		{name: "MicroDVD next to MPL2", content: "{10}{25}Hello|world\n", later: "MicroDVD"},
		{name: "SBV", content: "0:00:01.500,0:00:04.000\nHello\n", later: "SBV"},
		{name: "SBV with three digits", content: "0:00:01.500,0:00:04.000\nHello[br]world\n", later: "SBV"},
		{name: "SBV with two hour digits", content: "00:00:01.500,00:00:04.000\nHello\n", later: "SBV"},
		{name: "SBV next to TMPlayer", content: "0:00:01.000,0:00:02.000\nHello\n", later: "SBV"},
		{name: "SubViewer 2 information", content: "[INFORMATION]\r\n[TITLE]Bakery\r\n[END INFORMATION]\r\n", later: "SubViewer"},
		{name: "SubViewer 2 timing line", content: "00:00:01.50,00:00:04.00\nHello[br]world\n", later: "SubViewer"},
		{name: "SubViewer 2 after [SUBTITLE]", content: "[SUBTITLE]\n[COLF]&HFFFFFF,[SIZE]18\n00:00:01.50,00:00:04.00\nHello\n", later: "SubViewer"},
		{name: "SubViewer 1", content: "[TITLE]\nBakery\n[DELAY]\n0\n******** START SCRIPT ********\n[00:00:01]\nHello\n", later: "SubViewer"},
		{name: "LRC with ID tag", content: "[ti:Morning Train]\n[00:12.00]Hello\n", later: "LRC"},
		{name: "LRC without fraction", content: "[00:12]Hello\n", later: "LRC"},
		{name: "LRC next to MPL2", content: "[00:01.00]Hello\n[00:02.50]world\n", later: "LRC"},
		{name: "LRC with minutes only", content: "[00:01]Hello\n", later: "LRC"},
		{name: "MPL2", content: "[10][25]Hello|/world\n", later: "MPL2"},
		{name: "TMPlayer", content: "00:00:01:Hello|world\n", later: "TMPlayer"},
		{name: "TMPlayer+ with equals sign", content: "0:00:01=Hello\n", later: "TMPlayer"},
		{name: "TMPlayer+ with line numbers", content: "00:00:01,1=Hello\n00:00:01,2=world\n", later: "TMPlayer"},
		{name: "TMPlayer with a number", content: "00:00:01:5 trains leave\n", later: "TMPlayer"},

		{name: "Spruce STL", content: "00:00:01:10 , 00:00:03:10 , The bakery opens at seven.\n", later: "Spruce STL, 2.x"},
		{name: "Spruce STL without spaces", content: "00:00:01:10,00:00:03:10,The bakery opens at seven.\n", later: "Spruce STL, 2.x"},
		{name: "Spruce STL with tabs", content: "$FadeOut\t=\t0\n \n00:00:01:10\t,\t00:00:03:10\t,\tBakery\n", later: "Spruce STL, 2.x"},
		{name: "Spruce STL quote after comment", content: "//Subtitles\n00:00:01:10 , 00:00:03:10 , \"Bakery\"\n", later: "Spruce STL, 2.x"},
		{name: "EBU STL at 30 fps", content: "865STL30.011" + strings.Repeat(" ", 1012), later: "EBU STL, 2.x"},
		{name: "SCC header", content: "Scenarist_SCC V1.0\n\n00:00:00:00\t9420 9420\n", later: "SCC, 2.x"},
		{name: "YouTube srv3", content: "<?xml version=\"1.0\" encoding=\"utf-8\" ?><timedtext format=\"3\">\n<body>\n</body>\n</timedtext>\n", later: "YouTube, 4.x"},
		{name: "YouTube srv1", content: `<transcript><text start="1.2" dur="2.3">Hello</text></transcript>`, later: "YouTube, 4.x"},
		{name: "Podcasting 2.0 HTML", content: "<cite>Anna:</cite>\n<time>0:00</time>\n<p>Hello</p>\n", later: "HTML transcript, 4.x"},
		{name: "HTML document with time", content: "<!DOCTYPE html>\n<html><body>\n<CITE>Anna:</CITE>\n<TIME>0:00</TIME><p>Hi</p></body></html>", later: "HTML transcript, 4.x"},
		{name: "Whisper JSON", content: `{"text": " Hello", "segments": [{"id": 0, "start": 0.0, "end": 2.0, "text": " Hello"}], "language": "en"}`, later: "Whisper, 4.x"},
		{name: "whisper.cpp JSON", content: "{\n\t\"systeminfo\": \"\",\n\t\"transcription\": [\n\t]\n}\n", later: "Whisper, 4.x"},
		{name: "Whisper JSON with words", content: `{"segments": [], "words": [{"word": "Hi", "start": 0, "end": 1}], "results": [{"x": 1}]}`, later: "Whisper, 4.x"},
		{name: "Whisper JSON with a BOM", content: "\xEF\xBB\xBF\r\n{\"text\": \"\", \"segments\": []}", later: "Whisper, 4.x"},
		{name: "Whisper JSON with a cues key", content: `{"version": "1", "cues": [], "segments": []}`, later: "Whisper, 4.x"},
		{name: "Whisper JSON with a body", content: `{"segments": [{"start": 0.0, "end": 2.0, "text": " Hello", "body": 1}]}`, later: "Whisper, 4.x"},
		{name: "YouTube json3", content: `{"wireMagic": "pb3", "events": [ {"tStartMs": 0, "id": 1}, {"tStartMs": 0, "segs": []} ]}`, later: "YouTube, 4.x"},
		{name: "Podcasting 2.0 JSON", content: `{"version": "1.0.0", "segments": [{"speaker": "Anna", "startTime": 0.5, "body": "I"}]}`, later: "Podcasting 2.0, 4.x"},
		{name: "Podcasting 2.0 JSON body first", content: `{"segments":[{"body":"Hi","endTime":1,"startTime":0}]}`, later: "Podcasting 2.0, 4.x"},
		{
			name:    "Podcast transcript, not chapters",
			content: `{"version": "1.0.0", "chapters": [], "segments": [{"startTime": 0, "body": "Hi"}]}`,
			later:   "Podcasting 2.0, 4.x",
		},

		{name: "UTF-8 BOM", content: "\xEF\xBB\xBFWEBVTT\n", want: stbox.WebVTT},
		{name: "2 UTF-8 BOMs and blank lines", content: "\xEF\xBB\xBF\xEF\xBB\xBF\n\n1\n00:00:01,000 --> 00:00:02,000\nHi\n", want: stbox.SubRip},
		{name: "NUL, VT and tab first", content: "\x00\v\t WEBVTT\n", want: stbox.WebVTT},
		{name: "CR LF", content: "1\r\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n", want: stbox.SubRip},
		{name: "CR only", content: "1\r00:00:01,000 --> 00:00:02,000\rHi\r", want: stbox.SubRip},
		{name: "2 CRs before LF", content: "1\r\r\n00:00:01,000 --> 00:00:02,000\r\r\nHi\r\r\n", want: stbox.SubRip},
		{name: "CR after the header", content: "WEBVTT\r\r00:01.000 --> 00:02.000\rRain\r", want: stbox.WebVTT},
		{name: "BOM and CR LF before JSON", content: "\xEF\xBB\xBF\r\n{\"version\": 1, \"cues\": []}", want: stbox.JSON},
		{name: "bad UTF-8 in a JSON string", content: "{\"version\": 1, \"cues\": [], \"metadata\": {\"title\": \"caf\xe9\"}}", want: stbox.JSON},
		{name: "lone surrogate escape in JSON", content: `{"t": "\ud800", "version": 1, "cues": []}`, want: stbox.JSON},
		{name: "JSON key that starts with NUL", content: `{"\u0000a": 1, "version": 1, "cues": []}`, want: stbox.JSON},
		{name: "invalid JSON goes on to the signatures", content: "{24}{72}Hello\n", later: "MicroDVD"},
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	for _, tt := range detectTests() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NotEqual(t, tt.want == "", tt.later == "", "a row sets either want or later")
			f, ok := stbox.Detect([]byte(tt.content))
			assert.Equal(t, tt.want != "", ok)
			assert.Equal(t, tt.want, f)
		})
	}
}

// Ports unknownContent of tests/FormatDetectorTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestDetectFindsNoFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"empty string", ""},
		{"blank lines", "\n\r\n  \n"},
		{"BOM only", "\xEF\xBB\xBF"},
		{"plain text", "The train to the coast leaves at 7:15.\nBring a coat.\n"},
		{"JSON with version string", `{"version": "1", "cues": []}`},
		{"JSON with cues as text", `{"version": 1, "text": "\"cues\": ["}`},
		{"JSON", `{"cues": [{"start": 1, "end": 2, "text": "Hello"}]}`},
		{"HTML", "<!DOCTYPE html>\n<html><head><title>Bakery</title></head><body><p>Hello</p></body></html>"},
		{"XHTML", "<?xml version=\"1.0\"?>\n<html xmlns=\"http://www.w3.org/1999/xhtml\"></html>"},
		{"INI section with colon", "[server:main]\nport=80\n"},
		{"key value lines", "TITLE=Bakery\nAUTHOR=Jane Doe\n"},
		{"number without timing", "1\nHello\n"},
		{"SubRip after six lines", "a\nb\nc\nd\ne\nf\n1\n00:00:01,000 --> 00:00:04,000\nHello\n"},
		{"WebVTT timing without header or number", "00:00:01.000 --> 00:00:04.000\nHello\n"},
		{"WebVTT after text without header", "Converted online\n\n1\n00:00:01.000 --> 00:00:04.000\nHello\n"},
		{"SubRip timing after text without number", "Converted online\n\n00:00:01,000 --> 00:00:04,000\nHello\n"},
		{"WEBVTT inside a word", "WEBVTTX\n"},
		{"SAMI inside a word", "<SAMIPARAM>\n"},
		{"SAMI after text", "Converted online\n<SAMI>\n"},
		{"EBU STL at 24 fps", "850STL24.011" + strings.Repeat(" ", 1012)},
		{"events without tStartMs", `{"events": [{"start": 1}]}`},
		{"events as text", `{"text": "\"events\": [{\"tStartMs\": 1}]"}`},
		{"timedtext inside a word", "<timedtextx/>"},
		{"tt inside a word", "<?xml version=\"1.0\"?>\n<ttx/>"},
		{"tt after text", "Subtitles by the film club\n<tt xmlns=\"http://www.w3.org/ns/ttml\"/>"},
		{"MPL2 with a letter", "[1a][25]Hello\n"},
		{"clock time in text", "10:30 is the time.\n"},
		{"bracketed time in text", "Meet at [10:30] by the pier.\n[ferry] leaves at noon\n"},
		{"time tag with 4 minute digits", "[1234:45.67] Hello\n"},
		{"time tag with 61 seconds", "[00:61.00] Hello\n"},
		{"chapters without version", `{"chapters": [{"startTime": 0}]}`},
		{"chapters as text", `{"version": "1", "text": "\"chapters\": ["}`},
		{"OGM without name line", "CHAPTER01=00:00:00.000\nCHAPTER02=00:01:00.000\n"},
		{"YouTube chapters", "0:00 Intro\n2:48 Hearing aids\n4:20 Progress report\n"},
		{"HTML without time", "<cite>Anna:</cite>\n<p>Hello</p>\n"},
		{"cite and time as text", "Hello <cite> and <time>\n"},
		{"invalid JSON", `{"segments": [{"start": 0,`},
		{"JSON with a bad escape", `{"text": "\x", "segments": []}`},
		{"JSON list", `[{"tStartMs": 0}]`},
		{"segments as an object", `{"segments": {"0": {"start": 0}}}`},
		{"first event without tStartMs", `{"wireMagic": "pb3", "events": [{"id": 1}, {"tStartMs": 0, "segs": []}]}`},
		{"nested segments", `{"results": {"segments": [{"startTime": 0, "body": "Hi"}]}}`},
		{"Amazon Transcribe", `{"jobName": "a", "results": {"transcripts": [{"transcript": ""}], "items": []}}`},
		{"Amazon Transcribe speakers", `{"results": {"speaker_labels": {"segments": []}, "transcripts": []}}`},
		{"Deepgram", `{"metadata": {"channels": 1}, "results": {"channels": [{"alternatives": []}]}}`},
		{"Deepgram with topics", `{"results": {"topics": {"segments": []}, "channels": [{"detected_language": "en", "alternatives": []}]}}`},
		{"AssemblyAI", `{"id": "x", "audio_url": "https://example.com/a.mp3", "words": null}`},
		{"AssemblyAI words only", `{"words": [{"text": "Hi", "start": 250, "end": 650}]}`},
		{"Google V1", `{"results": [{"alternatives": [{"transcript": "hi"}], "resultEndTime": "1s"}]}`},
		{"Google V2 end first", `{"results": [{"resultEndOffset": "1s", "alternatives": []}], "metadata": {}}`},
		{"Podcast chapters first", `{"chapters": [], "version": "1.2.0"}`},
		{"FFmpeg metadata", ";FFMETADATA1\ntitle=Meetup\n"},
		{"OGM with blank line", "CHAPTER00 = 00:00:00.000\r\n\r\nCHAPTER00NAME=Intro\r\n"},
		{"Headerless CSV with quotes", "00:00:01:10,00:00:03:10,\"The bakery opens at seven.\"\n"},
		{"two frame timecodes", "00:00:01:10:00:00:03:10 Hello\n"},
		{"Podcast chapters, not a transcript", `{"version": "1.2.0", "chapters": [{"startTime": 0, "title": "Intro"}]}`},

		{"JSON object of no format", `{"segments": []}`},
		{"MPL2 lookalike", "[1]"},
		{"ASS header on line 2", "; Script\n[Script Info]\n"},
		{"ASS header with text after it", "[Script Info] v4\n"},
		{"form feed first", "\fWEBVTT\n"},
		{"BOM after a space", " \xEF\xBB\xBFWEBVTT\n"},
		{"library JSON after text", "x{\"version\": 1, \"cues\": []}"},
		{"library JSON with trailing text", `{"version": 1, "cues": []} x`},
		{"WebVTT inside a JSON object", `{"WEBVTT": 1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ok := stbox.Detect([]byte(tt.content))
			assert.False(t, ok)
			assert.Empty(t, f)
		})
	}
}

type fixture struct {
	rel    string
	format stbox.Format
}

// fixtures returns each file in a format folder and its real/ subfolder, except .md files. Each format issue
// adds its folder.
func fixtures(t *testing.T) []fixture {
	t.Helper()

	folders := map[string]stbox.Format{"ass": stbox.ASS, "json": stbox.JSON, "sami": stbox.SAMI, "srt": stbox.SubRip, "ttml": stbox.TTML, "vtt": stbox.WebVTT}
	var list []fixture
	for dir, f := range folders {
		for _, sub := range []string{dir, dir + "/real"} {
			entries, err := os.ReadDir(testfiles.Path(t, sub))
			require.NoError(t, err)
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) != ".md" {
					list = append(list, fixture{rel: sub + "/" + e.Name(), format: f})
				}
			}
		}
	}
	slices.SortFunc(list, func(a, b fixture) int { return strings.Compare(a.rel, b.rel) })
	return list
}

// fixtureEncodings holds the source encoding of the fixtures that need one.
func fixtureEncodings() map[string]string {
	return map[string]string{"sami/real/own_korean_cp949.smi": "CP949"}
}

// brokenFixtures break their own format on purpose, so detection finds another format or none.
func brokenFixtures() map[string]stbox.Format {
	return map[string]stbox.Format{"vtt/missing_webvtt_header.vtt": stbox.SubRip}
}

// Ports testDetectsTheFormatOfEveryFixture and testDetectsBrokenFixturesByTheirShape of
// tests/FormatDetectorTest.php of subtitle-toolbox-php.
func TestDetectEveryFixture(t *testing.T) {
	t.Parallel()

	broken := brokenFixtures()
	for _, fx := range fixtures(t) {
		t.Run(fx.rel, func(t *testing.T) {
			t.Parallel()

			want := fx.format
			if b, ok := broken[fx.rel]; ok {
				want = b
			}
			f, ok := stbox.Detect(testfiles.Read(t, fx.rel))
			assert.Equal(t, want != "", ok)
			assert.Equal(t, want, f)
		})
	}
}

// Ports testAutoDetectionGivesTheSameCuesAsTheDetectedFormat of tests/FormatDetectorTest.php of subtitle-toolbox-php.
func TestDetectionGivesTheCuesOfTheFormat(t *testing.T) {
	t.Parallel()

	for _, fx := range fixtures(t) {
		if !strings.Contains(fx.rel, "/real/") {
			continue
		}
		t.Run(fx.rel, func(t *testing.T) {
			t.Parallel()

			content := testfiles.Read(t, fx.rel)
			encoding := fixtureEncodings()[fx.rel]
			explicit, _, wantErr := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: fx.format, Encoding: encoding})
			detected, info, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Encoding: encoding})
			if wantErr != nil {
				require.EqualError(t, err, wantErr.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, fx.format, info.Format)
			assert.Equal(t, explicit.Cues(), detected.Cues())
		})
	}
}

// Ports testNeverReturnsAFormatThatIsNotAutoDetected of tests/FormatDetectorTest.php of subtitle-toolbox-php.
func TestDetectNeverReturnsAFormatThatIsNotAutoDetected(t *testing.T) {
	t.Parallel()

	fsys := os.DirFS(testfiles.Root(t))
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := fs.ReadFile(fsys, p)
		require.NoError(t, err)
		if f, ok := stbox.Detect(content); ok {
			assert.True(t, f.IsAutoDetected(), "%s detects as %s", p, f)
		}
		return nil
	})
	require.NoError(t, err)
}

// Ports testIgnoresUtf8BomAndLeadingBlankLines and testDetectsSubRipWithoutALeadingCueNumber of
// tests/FormatDetectorTest.php of subtitle-toolbox-php.
func TestDetectThenRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  []byte
		lenient  bool
		wantCues int
	}{
		{"BOM and blank lines", []byte("\xEF\xBB\xBF\r\n\r\n  \n1\r\n00:00:01,000 --> 00:00:04,000\r\nHello\r\n"), false, 1},
		{"no cue numbers", testfiles.Read(t, "lenient/no_cue_numbers.srt"), true, 3},
		{"text before the first cue", testfiles.Read(t, "lenient/text_before_first_cue.srt"), true, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, ok := stbox.Detect(tt.content)
			require.True(t, ok)
			assert.Equal(t, stbox.SubRip, f)
			s, info, err := stbox.Parse(bytes.NewReader(tt.content), stbox.ReadOptions{Lenient: tt.lenient})
			require.NoError(t, err)
			assert.Equal(t, stbox.SubRip, info.Format)
			assert.Equal(t, tt.wantCues, s.Len())
		})
	}
}

func FuzzDetect(f *testing.F) {
	for _, tt := range detectTests() {
		f.Add([]byte(tt.content))
	}
	root := testfiles.Root(f)
	for _, dir := range []string{"ass", "json", "sami/real", "srt", "ttml/real", "forced", "vtt", "lenient"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) != ".md" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		got, ok := stbox.Detect(content)
		if ok != (got != "") {
			t.Fatalf("Detect returned %q and %t", got, ok)
		}
		if ok && !got.IsAutoDetected() {
			t.Fatalf("Detect returned %s, which is not auto-detected", got)
		}
		withBOM, _ := stbox.Detect(append([]byte("\xEF\xBB\xBF"), content...))
		if withBOM != got {
			t.Fatalf("Detect returned %q, and %q after a BOM", got, withBOM)
		}
	})
}
