package textenc_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

type detected struct {
	Encoding string `json:"encoding"`
	Text     string `json:"text"`
}

func readDetected(t *testing.T) map[string]detected {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "detected.json"))
	require.NoError(t, err)
	var want map[string]detected
	require.NoError(t, json.Unmarshal(data, &want))
	return want
}

// Ports testDetectsTheCodePageOfTheFile of tests/Encoding/CodePageDetectorTest.php and
// testCodePageIsDetectedWithoutSourceEncoding of tests/EncodingTest.php of subtitle-toolbox-php.
func TestDetectCodePage(t *testing.T) {
	t.Parallel()

	want := readDetected(t)
	bad, err := filepath.Glob(filepath.Join(fixtures, "invalid-utf-8", "one-bad-byte.*"))
	require.NoError(t, err)
	require.Len(t, bad, 10)
	files := make([]string, 0, 2+len(want)+len(bad))
	files = append(files, "arabic-utf-8.srt", "utf-8-declared-utf-16.ttml")
	for name := range want {
		files = append(files, name)
	}
	for _, path := range bad {
		files = append(files, "invalid-utf-8/"+filepath.Base(path))
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			data := readFixture(t, file)
			name, text, ok := textenc.DetectCodePage(data)
			w, detects := want[file]
			assert.Equal(t, detects, ok)
			assert.Equal(t, w.Encoding, name)
			if !detects {
				return
			}
			assert.Contains(t, strings.ReplaceAll(string(text), "\r\n", "\n"), w.Text)

			got, err := textenc.Decode(data, "")
			require.NoError(t, err)
			assert.Equal(t, text, got.Text)
			assert.Equal(t, w.Encoding, got.Encoding)
			assert.Equal(t, "The content is not UTF-8. Detection picked "+w.Encoding+". Set the source encoding if that is wrong.", got.Warning)

			chosen, err := textenc.Decode(data, w.Encoding)
			require.NoError(t, err)
			assert.Equal(t, text, chosen.Text)
			assert.Empty(t, chosen.Warning)
		})
	}
}

func cp949(t *testing.T, s string) string {
	t.Helper()

	b, err := korean.EUCKR.NewEncoder().String(s)
	require.NoError(t, err)
	return b
}

// Ports testKoreanCp949IsNotReadAsHebrewOrArabic and testKoreanCp949WithABadByteKeepsItsBytes of
// tests/EncodingTest.php of subtitle-toolbox-php.
func TestDetectKorean(t *testing.T) {
	t.Parallel()

	for i, text := range readTexts(t).KoreanCues {
		data := cp949(t, "1\n00:00:01,000 --> 00:00:02,000\n"+text+"\n")
		name, got, ok := textenc.DetectCodePage([]byte(data))
		require.True(t, ok, "cue %d", i)
		assert.Equal(t, "CP949", name)
		assert.Contains(t, string(got), strings.Split(text, "\n\n")[0])

		cut := []byte(data + "\xB0")
		_, _, ok = textenc.DetectCodePage(cut)
		assert.False(t, ok, "cue %d with a bad byte", i)
		dec, err := textenc.Decode(cut, "")
		require.NoError(t, err)
		assert.Equal(t, cut, dec.Text)
		assert.Equal(t, "UTF-8", dec.Encoding)
	}
}

// Ports testCjkTextWithABadByteAfterTheSampleIsNotDetected and testCutOfTheSampleKeepsCjkCharactersWhole of
// tests/Encoding/CodePageDetectorTest.php of subtitle-toolbox-php.
func TestDetectLongCJKText(t *testing.T) {
	t.Parallel()

	tx := readTexts(t)
	korean := strings.Repeat("1\r\n00:00:01,000 --> 00:00:02,000\r\n"+cp949(t, tx.KoreanLine)+"\r\n\r\n", 20000)
	name, _, ok := textenc.DetectCodePage([]byte(korean))
	assert.True(t, ok)
	assert.Equal(t, "CP949", name)
	_, _, ok = textenc.DetectCodePage([]byte(korean + "\xB0"))
	assert.False(t, ok)

	gbk, err := simplifiedchinese.GBK.NewEncoder().String(tx.ChineseLine)
	require.NoError(t, err)
	chinese := strings.Repeat("1\r\n00:00:01,000 --> 00:00:02,000\r\n"+gbk+"...\r\n\r\n", 20000)
	name, _, ok = textenc.DetectCodePage([]byte(chinese))
	assert.True(t, ok)
	assert.Equal(t, "GBK", name)
}

func TestDetectCodePageWithoutHighBytes(t *testing.T) {
	t.Parallel()

	_, _, ok := textenc.DetectCodePage([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
	assert.False(t, ok)
}

func TestScoreText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		languages []string
		want      float64
	}{
		{name: "ASCII scores 0", text: "Hello", languages: []string{"pl"}, want: 0},
		{name: "Polish letters", text: "Dzi\u0119kuj\u0119, musz\u0119 ju\u017c i\u015b\u0107.", languages: []string{"pl"}, want: 1},
		{name: "Polish letters in Turkish", text: "Dzi\u0119kuj\u0119", languages: []string{"tr"}, want: -2},
		{name: "best language wins", text: "Dzi\u0119kuj\u0119", languages: []string{"tr", "pl"}, want: 1},
		{name: "known symbol", text: "caf\u00e9 \u20ac", languages: []string{"fr"}, want: 1},
		{name: "unknown symbol", text: "caf\u00e9 \u00a4", languages: []string{"fr"}, want: -0.5},
		{name: "lower-case start", text: "\u00e9t\u00e9", languages: []string{"fr"}, want: 0},
		{name: "case change", text: "a\u00c9", languages: []string{"fr"}, want: -1},
		{name: "extra capitals", text: "\u00c9\u00c9\u00c9\u00e9", languages: []string{"fr"}, want: 0.5},
		{name: "symbol between letters", text: "caf\u00e9\u00a4s", languages: []string{"fr"}, want: -2},
		{name: "script switch", text: "a\u03b1", languages: []string{"el"}, want: -1},
		{name: "misplaced Thai mark", text: "\u0e34\u0e01", languages: []string{"th"}, want: -0.5},
		{name: "leading Thai vowel at the end", text: "\u0e01\u0e40", languages: []string{"th"}, want: -0.5},
		{name: "Thai mark after a consonant", text: "\u0e01\u0e34", languages: []string{"th"}, want: 1},
		{name: "rare Han letter", text: "\u4e00\u9fa5", languages: []string{"zh-Hans"}, want: 0.5},
		{name: "rare Hangul syllable", text: "\ub620", languages: []string{"ko"}, want: 0},
		{name: "KS X 1001 syllable", text: "\uac00", languages: []string{"ko"}, want: 1},
		{name: "kana", text: "\u3042", languages: []string{"ja"}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, textenc.ScoreText(tt.text, tt.languages), 1e-9)
		})
	}
}
