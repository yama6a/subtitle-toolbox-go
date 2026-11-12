package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
)

// Ported from tests/Cli/BinaryEditTest.php of subtitle-toolbox-php.
func TestConvertFixesCommonErrors(t *testing.T) {
	t.Parallel()

	optional := "adCues,doubleApostrophes,loneLowercaseI,unneededPeriods,dialogueOnOneLine,musicNotes,sentenceStartCase"
	dir := fixtureDir(t, map[string]string{
		"web.srt": "fixing/web-errors.srt", "optional.srt": "fixing/optional-rules.srt", "ads.srt": "fixing/ad-cues.srt",
	})
	optionalFixed := string(testfiles.Read(t, "fixing/optional-rules.fixed.srt"))
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			"default rules",
			[]string{"web.srt", "--errors-fix", "--language", "en", "--line-ending", "crlf"},
			result{Stdout: string(testfiles.Read(t, "fixing/web-errors.fixed.srt"))},
		},
		{
			"optional rules",
			[]string{"optional.srt", "--errors-fix", "--language", "en", "--errors-enable", optional},
			result{Stdout: optionalFixed},
		},
		{
			"optional rules in upper case",
			[]string{"optional.srt", "--errors-fix", "--language", "en", "--errors-enable", strings.ToUpper(optional)},
			result{Stdout: optionalFixed},
		},
		{
			"names are trimmed and empty names skipped",
			[]string{"optional.srt", "--errors-fix", "--language", "en", "--errors-enable", " , " + strings.ReplaceAll(optional, ",", " ,\t") + ","},
			result{Stdout: optionalFixed},
		},
		{
			"default rules change nothing in the optional file",
			[]string{"optional.srt", "--errors-fix", "--language", "en", "--no-bom"},
			result{Stdout: string(testfiles.Read(t, "fixing/optional-rules.srt"))},
		},
		{
			"ad cues with the list of fixes",
			[]string{"ads.srt", "--errors-fix", "--language", "en", "--errors-enable", "adCues", "--errors-list-fixes"},
			result{
				Stdout: string(testfiles.Read(t, "fixing/ad-cues.fixed.srt")),
				Stderr: `ads.srt: cue 1: adCues: "Support us and become a VIP member\nto hide every advert at www.OpenSubtitles.org" -> ""` + "\n" +
					`ads.srt: cue 6: adCues: "Sincronizado y corregido por nubegris\nwww.addic7ed.com" -> ""` + "\n" +
					`ads.srt: cue 7: missingSpaceAfterPunctuation: "We landed late.It was cold." -> "We landed late. It was cold."` + "\n" +
					`ads.srt: cue 8: adCues: "Heruntergeladen von YTS.MX" -> ""` + "\n",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: dir}.run(t, append([]string{"convert", "--to", "srt", "-o", "-"}, tt.words...)...))
		})
	}
}

// Ported from testFixListPrintsTextThatIsNotUtf8WithReplacementCharacters of tests/RobustnessLimitsTest.php.
func TestConvertListsFixesOfTextThatIsNotUTF8WithReplacementCharacters(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"latin1.srt": "fixing/latin1_unclosed_italic.srt"})
	got := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "UTF-8", "--errors-fix", "--errors-list-fixes", "--to", "srt")
	assert.Equal(t, 0, got.Code)
	assert.Equal(t, "latin1.srt: cue 1: unbalancedTags: \"<i>Caf\ufffd au lait.\" -> \"<i>Caf\ufffd au lait.</i>\"\n", got.Stderr)
}

func TestConvertErrorsUsageErrors(t *testing.T) {
	t.Parallel()

	names := "adCues, doubleApostrophes, loneLowercaseI, unneededPeriods, dialogueOnOneLine, musicNotes, sentenceStartCase"
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"replace list without fix", []string{"--errors-replace-list", "list.xml"}, "Pass --errors-fix with --errors-replace-list."},
		{"enable without fix", []string{"--errors-enable", "adCues"}, "Pass --errors-fix with --errors-enable."},
		{"list without fix", []string{"--errors-list-fixes"}, "Pass --errors-fix with --errors-list-fixes."},
		{"enable checked before list", []string{"--errors-list-fixes", "--errors-enable", "adCues"}, "Pass --errors-fix with --errors-enable."},
		{
			"unknown rule",
			[]string{"--errors-fix", "--errors-enable", "adCues,dialogOnOneLine"},
			`Unknown rule "dialogOnOneLine" in --errors-enable. The valid names are ` + names + ".",
		},
		{
			"rule that is on by default",
			[]string{"--errors-fix", "--errors-enable", "ocrPipe"},
			`Unknown rule "ocrPipe" in --errors-enable. The valid names are ` + names + ".",
		},
		{
			"the name 0",
			[]string{"--errors-fix", "--errors-enable", "0"},
			`Unknown rule "0" in --errors-enable. The valid names are ` + names + ".",
		},
		{"language without case or fix", []string{"--timing-fix-overlaps", "--language", "en"}, "Pass --case or --errors-fix with --language."},
		{"errors before sdh", []string{"--sdh-lyrics", "--errors-list-fixes"}, "Pass --errors-fix with --errors-list-fixes."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.msg), call{dir: dir}.run(t, append([]string{"convert", "trip.srt", "--to", "srt", "-o", "-"}, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

// Ported from testFixCommonErrors of tests/Cli/BinaryEditTest.php of subtitle-toolbox-php.
func TestConvertAppliesAReplaceList(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"pal.srt": "fixing/text-pal.ocr.srt", "list.xml": "fixing/user_OCRFixReplaceList.xml"})
	got := call{dir: dir}.run(t, "convert", "pal.srt", "--to", "srt", "-o", "-", "--errors-fix", "--language", "en",
		"--errors-replace-list", "list.xml", "--errors-list-fixes")
	assert.Equal(t, 0, got.Code)
	assert.Equal(t, string(testfiles.Read(t, "fixing/text-pal.fixed.srt")), got.Stdout)
	assert.True(t, strings.HasPrefix(got.Stderr, "pal.srt: cue 1: "), got.Stderr)
	assert.Contains(t, got.Stderr, ": replaceList: ")
}

func TestConvertReadsAReplaceListWithABOM(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	list := "\ufeff" + string(testfiles.Read(t, "fixing/user_OCRFixReplaceList.xml"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bom.xml"), []byte(list), 0o600))
	got := call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--errors-fix", "--errors-replace-list", "bom.xml")
	assert.Equal(t, 0, got.Code, got.Stderr)
}

func TestConvertReplaceListFileErrorsStopBeforeAnyOutput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.xml"), []byte("<ReplaceList>\n<WholeWords></ReplaceList>"), 0o600))
	tests := []struct {
		name, list, stderr string
	}{
		{"missing file", "missing.xml", "Error: missing.xml: The file does not exist.\n"},
		{
			"broken XML", "bad.xml",
			"Error: bad.xml: line 2: The OCR replace list is not valid XML: element <WholeWords> closed by </ReplaceList>.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--errors-fix", "--errors-replace-list", tt.list)
			assert.Equal(t, result{Code: exitFile, Stderr: tt.stderr}, got)
		})
	}
}

func TestOptionalRuleNamesFollowTheLibrary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "adCues, doubleApostrophes, loneLowercaseI, unneededPeriods, dialogueOnOneLine, musicNotes, sentenceStartCase",
		ruleNames(fixing.OptionalRules()))
}

func TestConvertReplaceListIsNotAnOutput(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"list.srt": "fixing/user_OCRFixReplaceList.xml"})
	got := call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "list.srt", "--errors-fix", "--errors-replace-list", "list.srt")
	assert.Equal(t, usageResult("convert", "The output list.srt is a file that the command reads. Pass another output file or directory."), got)
}
