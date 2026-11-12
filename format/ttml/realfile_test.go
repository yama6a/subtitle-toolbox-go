package ttml_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

type realCue struct {
	Start, End float64
	Text       string
	Alignment  model.Alignment
}

func describe(c model.Cue) realCue {
	return realCue{c.Start.Seconds(), c.End.Seconds(), c.Text(), c.Position.Alignment}
}

// Ports testRealFileTakesTheCueTimesFromTimedSpansInStrictAndLenientMode of tests/Parsers/TtmlRealFileTest.php of
// subtitle-toolbox-php. The read golden holds the strict read.
func TestRealFileTakesTheCueTimesFromTimedSpansInLenientMode(t *testing.T) {
	t.Parallel()

	s, warnings := readFile(t, realDir+"/word_timed_spans.ttml", true)
	var cues []realCue
	for _, c := range s.All() {
		cues = append(cues, describe(c))
	}
	assert.Equal(t, []realCue{
		{1, 4.5, "The <00:00:02.000>tide <00:00:03.000>turns", model.BottomCenter},
		{5.5, 8, "at <00:00:06.500><i>dusk</i>", model.BottomCenter},
		{10, 13, "Boats <00:00:11.000>return <00:00:12.000>home", model.BottomCenter},
	}, cues)
	assert.Empty(t, warnings)
}

// Ports testRealFileParses of tests/ForcedCuesTest.php of subtitle-toolbox-php, the TTML rows.
func TestRealFileForcedCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		first, last string
		flags       []bool
	}{
		{"w3c_imsc11_forced.ttml", "1s 6s Lyc\u00e9e", "4s 6s Nous \u00e9tions inscrits au m\u00eame lyc\u00e9e.", []bool{true, false}},
		{"forced_inheritance.ttml", "1s 3s Wir gehen zum Hafen.", "10s 12s Der Zug f\u00e4hrt um acht.", []bool{false, true, true, true, false}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, "forced/"+tt.file, false)
			first, _ := s.Cue(0)
			last, _ := s.Cue(s.Len() - 1)
			assert.Equal(t, tt.first, first.Start.String()+" "+first.End.String()+" "+first.Text())
			assert.Equal(t, tt.last, last.Start.String()+" "+last.End.String()+" "+last.Text())
			assert.Equal(t, tt.flags, forcedFlags(s))
		})
	}
}

func TestForcedCues(t *testing.T) {
	t.Parallel()

	const itts = ` xmlns:itts="http://www.w3.org/ns/ttml/profile/imsc1#styling"`
	tests := []struct {
		name, root, head, body string
		want                   []bool
	}{
		{"paragraph", itts, "", `<div><p end="1s" itts:forcedDisplay=" true ">a</p><p end="1s" itts:forcedDisplay="True">b</p></div>`, []bool{true, false}},
		{"div", itts, "", `<div itts:forcedDisplay="true"><p end="1s">a</p><p end="1s" itts:forcedDisplay="false">b</p></div>`, []bool{true, false}},
		{"nearest container", itts, "", `<div itts:forcedDisplay="true"><div itts:forcedDisplay="false"><p end="1s">a</p></div></div>`, []bool{false}},
		{
			"region", itts, `<layout><region xml:id="r" itts:forcedDisplay="true"/><region xml:id="q"><style itts:forcedDisplay="false"/><style itts:forcedDisplay="true"/></region></layout>`,
			`<div><p end="1s" region="r">a</p><p end="1s" region="q">b</p><p end="1s" region="r" itts:forcedDisplay="false">c</p></div>`,
			[]bool{true, false, false},
		},
		{
			"container wins over the region", itts, `<layout><region xml:id="r" itts:forcedDisplay="true"/></layout>`,
			`<div itts:forcedDisplay="false"><p end="1s" region="r">a</p></div>`,
			[]bool{false},
		},
		{
			"style reference, the last that sets it wins", itts,
			`<styling><style xml:id="f" itts:forcedDisplay="true"/><style xml:id="n" itts:forcedDisplay="false"/><style xml:id="x"/></styling>`,
			`<div><p end="1s" style="n f x">a</p><p end="1s" style="f n">b</p><p end="1s" style="f" itts:forcedDisplay="false">c</p></div>`,
			[]bool{true, false, false},
		},
		{
			"span", itts, "",
			`<div><p end="1s">a <span itts:forcedDisplay="true">b</span></p><p end="1s">a <span><span itts:forcedDisplay="true">b</span></span></p>` +
				`<p end="1s" itts:forcedDisplay="true"><span itts:forcedDisplay="false">b</span></p></div>`,
			[]bool{true, true, true},
		},
		{"undeclared prefix", "", "", `<div><p end="1s" itts:forcedDisplay="true">a</p></div>`, []bool{true}},
		{"other namespace", ` xmlns:itts="urn:x"`, "", `<div><p end="1s" itts:forcedDisplay="true">a</p></div>`, []bool{false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, document(tt.body, tt.root, tt.head), false)
			assert.Equal(t, tt.want, forcedFlags(s))
		})
	}
}

func forcedFlags(s *model.Subtitle) []bool {
	var flags []bool
	for _, c := range s.All() {
		flags = append(flags, c.Forced)
	}
	return flags
}

func TestRealFilesReadInLenientModeWithoutWarnings(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		_, warnings := readFile(t, realDir+"/"+name, true)
		assert.Empty(t, warnings, name)
	}
}

func realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, realDir))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	return names
}
