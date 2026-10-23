package subviewer_test

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	info2 = "[INFORMATION]\n[TITLE]\n[AUTHOR]\n[SOURCE]\n[PRG]\n[FILEPATH]\n[DELAY]0\n[CD TRACK]0\n[COMMENT]\n[END INFORMATION]\n[SUBTITLE]\n"
	head1 = "[TITLE]\n[AUTHOR]\n[SOURCE]\n[PRG]\n[FILEPATH]\n[DELAY]\n0\n[CD TRACK]\n0\n[BEGIN]\n" + script
	tail1 = "[end]\n******** END SCRIPT ********\n"
)

var (
	v1 = subviewer.WriteOptions{Version: subviewer.V1}
	v2 = subviewer.WriteOptions{Version: subviewer.V2}
)

// Ports the tests of tests/Formatters/SubViewerFormatterTest.php of subtitle-toolbox-php.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    *model.Subtitle
		opts subviewer.WriteOptions
		want string
	}{
		{
			"SubViewer 2 by default",
			withMetadata(subtitle(
				cue(1500*ms, 4004*ms, "<b>Fresh bread</b>", "every day"),
				cue(3725006*ms, 3727995*ms, "Closed on Sunday"),
			), model.MetadataTitle, "Bakery"),
			subviewer.WriteOptions{},
			"[INFORMATION]\n[TITLE]Bakery\n[AUTHOR]\n[SOURCE]\n[PRG]\n[FILEPATH]\n[DELAY]0\n[CD TRACK]0\n[COMMENT]\n" +
				"[END INFORMATION]\n[SUBTITLE]\n" +
				"00:00:01.50,00:00:04.00\nFresh bread[br]every day\n\n" +
				"01:02:05.01,01:02:08.00\nClosed on Sunday\n",
		},
		{
			"SubViewer 1 with the version option",
			withMetadata(subtitle(cue(1500*ms, 4400*ms, "Fresh bread", "every day")), model.MetadataAuthor, "Jane Doe"),
			v1,
			"[TITLE]\n[AUTHOR]\nJane Doe\n[SOURCE]\n[PRG]\n[FILEPATH]\n[DELAY]\n0\n[CD TRACK]\n0\n[BEGIN]\n" + script +
				"[00:00:02]\nFresh bread|every day\n[00:00:04]\n\n" + tail1,
		},
		{
			"markup is removed, entities decoded and cues without text skipped",
			subtitle(cue(time.Second, 2*time.Second, "<i>Tea &amp; cake &lt;3</i>"), cue(3*time.Second, 4*time.Second, "<i> </i>")),
			subviewer.WriteOptions{},
			info2 + "00:00:01.00,00:00:02.00\nTea & cake <3\n",
		},
		{
			"SubViewer 1 skips cues without text",
			subtitle(cue(time.Second, 2*time.Second, "<i> </i>"), cue(3*time.Second, 4*time.Second, "B")),
			v1,
			head1 + "[00:00:03]\nB\n[00:00:04]\n\n" + tail1,
		},
		{"SubViewer 2 without cues", subtitle(), v2, info2},
		{"SubViewer 1 without cues", subtitle(), v1, head1 + tail1},
		{
			"a cue without text between two cues leaves one empty line in SubViewer 2",
			subtitle(cue(time.Second, 2*time.Second, "A"), cue(3*time.Second, 4*time.Second), cue(5*time.Second, 6*time.Second, "C")),
			v2,
			info2 + "00:00:01.00,00:00:02.00\nA\n\n00:00:05.00,00:00:06.00\nC\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, tt.opts))
		})
	}
}

// Ports testSubViewer1WritesDelayZeroBecauseTheParserAppliedIt of tests/Formatters/SubViewerFormatterTest.php.
func TestVersion1WritesDelayZeroBecauseTheReaderAppliedIt(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "[DELAY]\n3\n"+script+"[00:00:01]\nHello\n[00:00:02]\n", false)
	assert.Equal(t, "[TITLE]\n[AUTHOR]\n[DELAY]\n0\n"+script+"[00:00:04]\nHello\n[00:00:05]\n\n"+tail1, write(t, s, v1))
}

func TestVersionRule(t *testing.T) {
	t.Parallel()

	v1File := "[TITLE]\nBakery\n[DELAY]\n2\n" + script + "[00:00:01]\nHello\n[00:00:02]\n"
	v2File := "[INFORMATION]\n[TITLE]Bakery\n[COMMENT]x\n[END INFORMATION]\n[SUBTITLE]\n[COLF]&HFFFFFF\n00:00:01.00,00:00:02.00\nHello\n"
	tests := []struct {
		name    string
		content string
		opts    subviewer.WriteOptions
		want    string
	}{
		{
			"a SubViewer 1 file stays SubViewer 1", v1File,
			subviewer.WriteOptions{},
			"[TITLE]\nBakery\n[AUTHOR]\n[DELAY]\n0\n" + script + "[00:00:03]\nHello\n[00:00:04]\n\n" + tail1,
		},
		{
			"a SubViewer 2 file stays SubViewer 2", v2File,
			subviewer.WriteOptions{},
			"[INFORMATION]\n[TITLE]Bakery\n[AUTHOR]\n[COMMENT]x\n[END INFORMATION]\n[SUBTITLE]\n[COLF]&HFFFFFF\n00:00:01.00,00:00:02.00\nHello\n",
		},
		{
			"V2 for a SubViewer 1 file", v1File, v2,
			"[INFORMATION]\n[TITLE]Bakery\n[AUTHOR]\n[DELAY]0\n[END INFORMATION]\n[SUBTITLE]\n00:00:03.00,00:00:04.00\nHello\n",
		},
		{
			"V1 for a SubViewer 2 file", v2File, v1,
			"[TITLE]\nBakery\n[AUTHOR]\n[COMMENT]\nx\n" + script + "[00:00:01]\nHello\n[00:00:02]\n\n" + tail1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, false)
			assert.Equal(t, tt.want, write(t, s, tt.opts))
		})
	}
}

func TestStoredVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version any
		want    subviewer.Version
	}{
		{"1", int64(1), subviewer.V1},
		{"1.0", float64(1), subviewer.V1},
		{"2", int64(2), subviewer.V2},
		{"0", int64(0), subviewer.V2},
		{"3", int64(3), subviewer.V2},
		{"a string", "1", subviewer.V2},
		{"null", nil, subviewer.V2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(cue(time.Second, 2*time.Second, "A"))
			require.NoError(t, s.SetFormatData("subviewer", model.NewData(model.Field{Key: "version", Value: tt.version})))
			want := write(t, subtitle(cue(time.Second, 2*time.Second, "A")), subviewer.WriteOptions{Version: tt.want})
			assert.Equal(t, want, write(t, s, subviewer.WriteOptions{}))
		})
	}
}

func TestStoredHeaderAndDefaults(t *testing.T) {
	t.Parallel()

	withHeader := func(header any) *model.Subtitle {
		s := subtitle(cue(time.Second, 2*time.Second, "A"))
		require.NoError(t, s.SetFormatData("subviewer", model.NewData(model.Field{Key: "header", Value: header})))
		return s
	}
	object := model.NewData(model.Field{Key: "COMMENT", Value: "Hi"}, model.Field{Key: "TITLE", Value: "Ignored"}, model.Field{Key: "AUTHOR", Value: "Ignored"}, model.Field{Key: "DELAY", Value: "7"})
	tests := []struct {
		name string
		s    *model.Subtitle
		opts subviewer.WriteOptions
		want string
	}{
		{
			"an object keeps its order, the title and author come from the metadata", withHeader(object), v2,
			"[INFORMATION]\n[TITLE]\n[AUTHOR]\n[COMMENT]Hi\n[DELAY]7\n[END INFORMATION]\n[SUBTITLE]\n00:00:01.00,00:00:02.00\nA\n",
		},
		{
			"an object in SubViewer 1 writes the delay as 0", withHeader(object), v1,
			"[TITLE]\n[AUTHOR]\n[COMMENT]\nHi\n[DELAY]\n0\n" + script + "[00:00:01]\nA\n[00:00:02]\n\n" + tail1,
		},
		{
			"a list has the keys 0, 1 and so on", withHeader([]any{"a", "b"}), v2,
			"[INFORMATION]\n[TITLE]\n[AUTHOR]\n[0]a\n[1]b\n[END INFORMATION]\n[SUBTITLE]\n00:00:01.00,00:00:02.00\nA\n",
		},
		{
			"an empty header has no default tags", withHeader([]any{}), v2,
			"[INFORMATION]\n[TITLE]\n[AUTHOR]\n[END INFORMATION]\n[SUBTITLE]\n00:00:01.00,00:00:02.00\nA\n",
		},
		{"no header gives the default tags", subtitle(cue(time.Second, 2*time.Second, "A")), v2, info2 + "00:00:01.00,00:00:02.00\nA\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, tt.opts))
		})
	}
}

// The writer never writes a header that reads back as another header. Each row writes one stored tag and reads it.
func TestHeaderReadsBack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		key, value   string
		want1, want2 map[string]string
	}{
		{"a plain tag", "SOURCE", "Disc", map[string]string{"SOURCE": "Disc"}, map[string]string{"SOURCE": "Disc"}},
		{"an empty value", "SOURCE", "", map[string]string{"SOURCE": ""}, map[string]string{"SOURCE": ""}},
		{"a key with a space and a digit", "CD TRACK 2", "x", map[string]string{"CD TRACK 2": "x"}, map[string]string{"CD TRACK 2": "x"}},
		{"a key that is a number", "5", "x", map[string]string{"5": "x"}, map[string]string{"5": "x"}},
		{"LF in the value", "SOURCE", "a\nb", map[string]string{"SOURCE": "a b"}, map[string]string{"SOURCE": "a b"}},
		{"CR LF in the value", "SOURCE", "a\r\nb\rc", map[string]string{"SOURCE": "a  b c"}, map[string]string{"SOURCE": "a  b c"}},
		{"a lower-case key", "source", "x", nil, nil},
		{"a mixed-case key", "Source", "x", nil, nil},
		{"an empty key", "", "x", nil, nil},
		{"a key with ]", "A]B", "x", nil, nil},
		{"a key with a space at the end", "SOURCE ", "x", nil, nil},
		{"a key with a space at the start", " SOURCE", "x", nil, nil},
		{"a key with LF", "SO\nURCE", "x", nil, nil},
		{"a key with a letter beyond ASCII", "CAFÉ", "x", map[string]string{"CAFÉ": "x"}, map[string]string{"CAFÉ": "x"}},
		{"a key with a lower-case letter beyond ASCII", "CAFé", "x", map[string]string{"CAFé": "x"}, map[string]string{"CAFé": "x"}},
		{"INFORMATION", "INFORMATION", "x", map[string]string{"INFORMATION": "x"}, nil},
		{"END INFORMATION", "END INFORMATION", "x", map[string]string{"END INFORMATION": "x"}, nil},
		{"SUBTITLE", "SUBTITLE", "x", map[string]string{"SUBTITLE": "x"}, nil},
		{"COLF", "COLF", "x", map[string]string{"COLF": "x"}, nil},
		{"SIZE", "SIZE", "x", map[string]string{"SIZE": "x"}, nil},
		{"a value with a style tag", "SOURCE", "a [FONT] b", map[string]string{"SOURCE": "a [FONT] b"}, map[string]string{"SOURCE": ""}},
		{"a value that looks like a timing line", "SOURCE", "x,1:30:00", map[string]string{"SOURCE": "x,1:30:00"}, map[string]string{"SOURCE": ""}},
		{"a value with a comma and text", "SOURCE", "a,b", map[string]string{"SOURCE": "a,b"}, map[string]string{"SOURCE": "a,b"}},
		{"a value that starts with [", "SOURCE", "[x]y", map[string]string{"SOURCE": "[x]y"}, map[string]string{"SOURCE": "[x]y"}},
		{"a value that is the start script line", "SOURCE", "******** START SCRIPT ********", map[string]string{"SOURCE": "******** START SCRIPT ********"}, map[string]string{"SOURCE": "******** START SCRIPT ********"}},
		{"a value of spaces", "SOURCE", "  ", map[string]string{"SOURCE": ""}, map[string]string{"SOURCE": ""}},
	}
	for _, tt := range tests {
		for _, version := range []subviewer.Version{subviewer.V1, subviewer.V2} {
			t.Run(tt.name+" in version "+version.String(), func(t *testing.T) {
				t.Parallel()

				s := subtitle(cue(time.Second, 2*time.Second, "A"))
				require.NoError(t, s.SetFormatData("subviewer", model.NewData(model.Field{Key: "header", Value: model.NewData(model.Field{Key: tt.key, Value: tt.value})})))
				back, warnings := mustRead(t, write(t, s, subviewer.WriteOptions{Version: version}), false)
				assert.Empty(t, warnings)
				want := tt.want1
				if version == subviewer.V2 {
					want = tt.want2
				}
				assert.Equal(t, orEmpty(want), header(back))
				assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"A"}}}, cueRows(back))
				assert.Empty(t, back.AllMetadata())
			})
		}
	}
}

func TestMetadataReadsBack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, title string
		want        string
	}{
		{"LF", "A\nB", "A B"},
		{"CR LF", "A\r\nB", "A  B"},
		{"a bracket", "[x]y", "[x]y"},
		{"the start script line", "******** START SCRIPT ********", "******** START SCRIPT ********"},
		{"a style tag in SubViewer 2", "Part [SIZE]", "Part [SIZE]"},
		{"a timing look-alike in SubViewer 2", "Meet,1:30:00", "Meet,1:30:00"},
	}
	for _, tt := range tests {
		for _, version := range []subviewer.Version{subviewer.V1, subviewer.V2} {
			t.Run(tt.name+" in version "+version.String(), func(t *testing.T) {
				t.Parallel()

				s := withMetadata(subtitle(cue(time.Second, 2*time.Second, "A")), model.MetadataTitle, tt.title)
				back, _ := mustRead(t, write(t, s, subviewer.WriteOptions{Version: version}), false)
				want := map[string]string{"title": tt.want}
				if version == subviewer.V2 && (tt.name == "a style tag in SubViewer 2" || tt.name == "a timing look-alike in SubViewer 2") {
					want = map[string]string{}
				}
				assert.Equal(t, want, back.AllMetadata())
				assert.Equal(t, 1, back.Len())
			})
		}
	}
}

func TestStyleLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style any
		want  string
	}{
		{"a style line", "[COLF]&HFFFFFF,[SIZE]18", "[COLF]&HFFFFFF,[SIZE]18\n"},
		{"CR and LF become spaces", "[COLF]a\r\nb", "[COLF]a  b\n"},
		{"white space at the ends goes", "  [FONT]Arial \n", "[FONT]Arial\n"},
		{"a text that is no style line", "Hello", ""},
		{"a style line that looks like a timing line", "[COLF]a,1:2:3", ""},
		{"an empty style", "", ""},
		{"null", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(cue(time.Second, 2*time.Second, "A"))
			require.NoError(t, s.SetFormatData("subviewer", model.NewData(model.Field{Key: "header", Value: []any{}})))
			d, _ := s.FormatData("subviewer")
			d.Set("style", tt.style)
			require.NoError(t, s.SetFormatData("subviewer", d))
			want := "[INFORMATION]\n[TITLE]\n[AUTHOR]\n[END INFORMATION]\n[SUBTITLE]\n" + tt.want + "00:00:01.00,00:00:02.00\nA\n"
			assert.Equal(t, want, write(t, s, v2))
			back, _ := mustRead(t, want, false)
			assert.Equal(t, 1, back.Len())
		})
	}
}

func TestTimesRoundAndStopBelowTheLimit(t *testing.T) {
	t.Parallel()

	limit := timecode.MaxValidTime
	tests := []struct {
		name       string
		start, end time.Duration
		want1      string
		want2      string
	}{
		{"half a second rounds up", 1500 * ms, 2500 * ms, "[00:00:02]", "00:00:01.50,00:00:02.50"},
		{"just below half a second", 1499 * ms, 2499 * ms, "[00:00:01]", "00:00:01.50,00:00:02.50"},
		{"half a centisecond rounds up", 1005 * ms, 2015 * ms, "[00:00:01]", "00:00:01.01,00:00:02.02"},
		{"1.025 rounds up", 1025 * ms, 1035 * ms, "[00:00:01]", "00:00:01.03,00:00:01.04"},
		{"just below half a centisecond", 1004 * ms, 1014 * ms, "[00:00:01]", "00:00:01.00,00:00:01.01"},
		{"100 hours has 3 digits", 100 * time.Hour, 100*time.Hour + time.Second, "[100:00:00]", "100:00:00.00,100:00:01.00"},
		{"the largest time", 0, limit, "[00:00:00]", "00:00:00.00,99999:59:59.99"},
		{"a time below the limit by half a centisecond", limit - 4*ms, limit - 5*ms, "[99999:59:59]", "99999:59:59.99,99999:59:59.99"},
		{"a negative time is 0", -5 * time.Second, -time.Second, "[00:00:00]", "00:00:00.00,00:00:00.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(cue(tt.start, tt.end, "A"))
			out := write(t, s, v1)
			assert.Contains(t, out, tt.want1+"\nA\n")
			out = write(t, s, v2)
			assert.Contains(t, out, tt.want2+"\nA\n")
			for _, opts := range []subviewer.WriteOptions{v1, v2} {
				_, _ = mustRead(t, write(t, s, opts), false)
			}
		})
	}
}

func TestLineEndingAndBOM(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(time.Second, 2*time.Second, "A"))
	var out bytes.Buffer
	require.NoError(t, subviewer.Write(&out, s, model.CommonWriteOptions{}, v2))
	assert.Equal(t, info2+"00:00:01.00,00:00:02.00\nA\n", out.String(), "no BOM by default")

	out.Reset()
	require.NoError(t, subviewer.Write(&out, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}, v1))
	want := "\xEF\xBB\xBF" + bytes.NewBufferString(head1+"[00:00:01]\nA\n[00:00:02]\n\n"+tail1).String()
	assert.Equal(t, string(bytes.ReplaceAll([]byte(want), []byte("\n"), []byte("\r\n"))), out.String())
}

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, v := range []subviewer.Version{subviewer.VersionDefault, subviewer.V1, subviewer.V2} {
		require.NoError(t, subviewer.WriteOptions{Version: v}.Validate())
	}
	err := subviewer.WriteOptions{Version: subviewer.Version(3)}.Validate()
	var aerr *model.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Equal(t, "WriteOptions.SubViewer.Version", aerr.Option)
	assert.Equal(t, "The SubViewer version must be 1 or 2, got 3.", aerr.Msg)

	var out bytes.Buffer
	err = subviewer.Write(&out, subtitle(), model.CommonWriteOptions{}, subviewer.WriteOptions{Version: subviewer.Version(200)})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Empty(t, out.String())
}

func TestVersionString(t *testing.T) {
	t.Parallel()

	for v, want := range map[subviewer.Version]string{subviewer.VersionDefault: "default", subviewer.V1: "1", subviewer.V2: "2", subviewer.Version(3): "Version(3)"} {
		assert.Equal(t, want, v.String(), strconv.Itoa(int(v)))
	}
}

func withMetadata(s *model.Subtitle, key, value string) *model.Subtitle {
	s.SetMetadata(key, value)
	return s
}
