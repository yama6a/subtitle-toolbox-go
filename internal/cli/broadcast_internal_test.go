package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	ferryFitReport = `ferry.srt: Cue #0 at 1 s: replaced "Š" with "S", "…" with "...". (transliterated)
ferry.srt: Cue #0 at 1 s: wrapped the text into 3 lines of at most 32 characters. (wrapped)
ferry.srt: Cue #0 at 1 s shows at 1.969 s. The frames before its start cannot hold its caption data. (delayed)
ferry.srt: Cue #2 at 4.3 s: replaced "–" with "-". (transliterated)
ferry.srt: Cue #2 at 4.3 s: wrapped the text into 2 lines of at most 32 characters. (wrapped)
ferry.srt: Cue #2 at 4.3 s shows at 5.172 s. The frames before its start cannot hold its caption data. (delayed)
ferry.srt: Cue #3 at 5 s is left out. Its caption data needs the frames up to 5.839 s, but the cue ends at 5.4 s. (dropped)
`
	wrapAdvice = "Pass --structure-wrap --structure-max-cpl 32 --structure-max-lines 4."
	mccRates   = "The MCC formatter accepts the frame rates 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60, got 12."
)

var broadcastFiles = map[string]string{
	"news.mcc":   "mcc/real/v1_23976_timecode.mcc",
	"roll.scc":   "scc/real/rollup_news_ndf.scc",
	"ferry.srt":  "scc/fit/own_ferry.srt",
	"bakery.stl": "stl/real/bakery_teletext_25fps.stl",
	"signs.itt":  "forced/forced_signs_2398.itt",
}

// readThenWrite reads the file at path below testdata with ro and writes it in format f with wo.
func readThenWrite(t *testing.T, path string, ro stbox.ReadOptions, f stbox.Format, wo stbox.WriteOptions) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, path), ro)
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = stbox.Write(&buf, s, f, wo)
	require.NoError(t, err)
	return buf.String()
}

func TestBroadcastCalls(t *testing.T) {
	t.Parallel()

	rate25 := model.FrameRate{Num: 25, Den: 1}
	rollUpLines := stbox.ReadOptions{SCC: scc.ReadOptions{RollUp: scc.RollUpLines}}
	broken := "Scenarist_SCC V1.0\n\n00:00:01:00\tzz\n"
	brokenMsg := `broken.scc: line 3: The SCC line has the invalid byte pair "zz".`
	emDash := "1\n00:00:01,000 --> 00:00:02,000\nA \u2014 \u0416 \u00ab x\n"
	five := "1\n00:00:01,000 --> 00:00:03,000\n1\n2\n3\n4\n5\n"
	one := "1\n00:00:01,000 --> 00:00:03,000\nHi\n"

	// checks inspects the directory after the call of the row with the same name.
	checks := map[string]func(t *testing.T, dir string){
		"SCC limits in a batch": func(t *testing.T, dir string) {
			t.Helper()
			assert.FileExists(t, filepath.Join(dir, "out", "one.scc"))
			assert.NoFileExists(t, filepath.Join(dir, "out", "five.scc"))
		},
		"forced iTT cues": func(t *testing.T, dir string) {
			t.Helper()
			assert.Equal(t, 3, strings.Count(readFile(t, dir, "signs.srt"), " --> "))
			call{dir: dir}.run(t, "convert", "signs.itt", "--to", "srt", "-o", "all.srt")
			assert.Equal(t, 6, strings.Count(readFile(t, dir, "all.srt"), " --> "))
		},
	}
	tests := []struct {
		name  string
		args  []string
		stdin string
		// inline holds files that the test writes into the directory before the call.
		inline map[string]string
		want   func(t *testing.T) result
	}{
		{
			"MCC output at another rate",
			[]string{"convert", "news.mcc", "--output-fps", "25", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "mcc/real/v1_23976_timecode.mcc", stbox.ReadOptions{}, stbox.MCC,
					stbox.WriteOptions{MCC: mcc.WriteOptions{FrameRate: rate25}})}
			},
		},
		{
			"MCC output at the stored rate",
			[]string{"convert", "news.mcc", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "mcc/real/v1_23976_timecode.mcc", stbox.ReadOptions{}, stbox.MCC, stbox.WriteOptions{})}
			},
		},
		{
			"roll-up lines in any case",
			[]string{"convert", "roll.scc", "--to", "srt", "-o", "-", "--scc-roll-up", "Lines"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "scc/real/rollup_news_ndf.scc", rollUpLines, stbox.SubRip, stbox.WriteOptions{})}
			},
		},
		{
			"roll-up lines from standard input",
			[]string{"convert", "-", "--from", "scc", "--to", "srt", "-o", "-", "--scc-roll-up", "lines"},
			string(testfiles.Read(t, "scc/real/rollup_news_ndf.scc")), nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "scc/real/rollup_news_ndf.scc", rollUpLines, stbox.SubRip, stbox.WriteOptions{})}
			},
		},
		{
			"roll-up screen is the default",
			[]string{"convert", "roll.scc", "--to", "srt", "-o", "-", "--scc-roll-up", "screen"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "scc/real/rollup_news_ndf.scc", stbox.ReadOptions{}, stbox.SubRip, stbox.WriteOptions{})}
			},
		},
		{
			"roll-up on a SubRip input",
			[]string{"convert", "trip.srt", "-o", "-", "--scc-roll-up", "lines"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: convertAs(t, "trip.srt", stbox.SubRip)}
			},
		},
		{
			"unknown roll-up mode",
			[]string{"convert", "roll.scc", "-o", "-", "--scc-roll-up", "rows"},
			"", nil,
			func(*testing.T) result {
				return usageResult("convert", `The option --scc-roll-up must be screen or lines, got "rows".`)
			},
		},
		{
			"SCC output that needs a wrap",
			[]string{"convert", "ferry.srt", "--to", "scc", "-o", "-"},
			"", nil,
			func(*testing.T) result {
				return result{Code: exitFile, Stderr: "ferry.srt: Cue #0 at 1 s has a line with 46 characters, but SCC allows 32. " + wrapAdvice + "\n"}
			},
		},
		{
			"SCC output with --scc-fit",
			[]string{"convert", "ferry.srt", "--to", "scc", "--scc-fit", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{
					Stdout: string(testfiles.Read(t, "scc/fit/own_ferry.scc")),
					Stderr: ferryFitReport,
				}
			},
		},
		{
			"--scc-fit with another output",
			[]string{"convert", "ferry.srt", "--to", "vtt", "--scc-fit", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "scc/fit/own_ferry.srt", stbox.ReadOptions{}, stbox.WebVTT, stbox.WriteOptions{})}
			},
		},
		{
			"MCC output reports the fit",
			[]string{"convert", "ferry.srt", "--to", "mcc", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{
					Stdout: readThenWrite(t, "scc/fit/own_ferry.srt", stbox.ReadOptions{}, stbox.MCC, stbox.WriteOptions{}),
					Stderr: ferryFitReport,
				}
			},
		},
		{
			"SCC split case",
			[]string{
				"convert", "bakery.stl", "--from", "stl", "--to", "scc", "-o", "-",
				"--structure-wrap", "--structure-max-cpl", "32", "--structure-max-lines", "4",
			},
			"", nil,
			func(*testing.T) result {
				return result{Code: exitFile, Stderr: "bakery.stl: Cue #4 at 36014 s has a line with 38 characters, but SCC allows 32. " +
					"Split the cue into cues of 4 lines of 32 characters, then wrap the lines.\n"}
			},
		},
		{
			"SCC limits in a batch",
			[]string{"convert", "five.srt", "one.srt", "--to", "scc", "--output-dir", "out", "--keep-going"},
			"",
			map[string]string{"five.srt": five, "one.srt": one},
			func(*testing.T) result {
				return result{
					Code:   exitFile,
					Stdout: "one.srt -> out/one.scc\n2 files: 1 succeeded, 1 failed.\n",
					Stderr: "five.srt: Cue #0 at 1 s has 5 lines, but SCC allows 4. " + wrapAdvice + "\n",
				}
			},
		},
		{
			"strict SCC read",
			[]string{"convert", "broken.scc", "--to", "srt", "-o", "-"},
			"",
			map[string]string{"broken.scc": broken},
			func(*testing.T) result { return result{Code: exitFile, Stderr: brokenMsg + "\n" + lenientHint + "\n"} },
		},
		{
			"lenient SCC read",
			[]string{"convert", "broken.scc", "--to", "srt", "-o", "-", "--lenient"},
			"",
			map[string]string{"broken.scc": broken},
			func(*testing.T) result {
				return result{Stdout: "\ufeff", Stderr: brokenMsg + " (skipped)\n"}
			},
		},
		{
			"iTT output without a rate",
			[]string{"convert", "trip.srt", "--to", "itt", "-o", "-"},
			"", nil,
			func(*testing.T) result {
				return result{Code: exitFile, Stderr: "trip.srt: iTT output needs the frame rate of the video. Pass --fps or --output-fps.\n"}
			},
		},
		{
			"iTT output with --fps",
			[]string{"convert", "trip.srt", "--to", "itt", "--fps", "25", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: readThenWrite(t, "cli/trip.srt", stbox.ReadOptions{}, stbox.ITT,
					stbox.WriteOptions{ITT: itt.WriteOptions{FrameRate: rate25}})}
			},
		},
		{
			"MCC rate that MCC lacks, with --to",
			[]string{"convert", "trip.srt", "--to", "mcc", "--output-fps", "12", "-o", "-"},
			"", nil,
			func(*testing.T) result { return usageResult("convert", mccRates) },
		},
		{
			"MCC rate that MCC lacks, without --to",
			[]string{"convert", "news.mcc", "--output-fps", "12", "-o", "-"},
			"", nil,
			func(*testing.T) result { return result{Code: exitFile, Stderr: "news.mcc: " + mccRates + "\n"} },
		},
		{
			"iTT rate that iTT lacks",
			[]string{"convert", "trip.srt", "--to", "itt", "--output-fps", "12", "-o", "-"},
			"", nil,
			func(*testing.T) result {
				return usageResult("convert", "The ITT formatter accepts the frame rates 23.976, 24, 25, 29.97 and 30, got 12.")
			},
		},
		{
			"output rate for another output",
			[]string{"convert", "trip.srt", "--output-fps", "12", "-o", "-"},
			"", nil,
			func(t *testing.T) result {
				t.Helper()
				return result{Stdout: convertAs(t, "trip.srt", stbox.SubRip)}
			},
		},
		{
			"forced iTT cues",
			[]string{"convert", "signs.itt", "--to", "srt", "-o", "signs.srt", "--forced-only"},
			"", nil,
			func(*testing.T) result { return result{Stdout: "signs.itt -> signs.srt\n"} },
		},
		{
			"EBU STL output with characters that the table lacks",
			[]string{"convert", "emdash.srt", "--to", "stl", "-o", "out.stl"},
			"",
			map[string]string{"emdash.srt": emDash},
			func(*testing.T) result {
				return result{
					Stdout: "emdash.srt -> out.stl\n",
					Stderr: "emdash.srt: Cue 1: the character table has no \"\u2014\", so the file holds \"?\".\n" +
						"emdash.srt: Cue 1: the character table has no \"\u0416\", so the file holds \"?\".\n",
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := fixtureDir(t, broadcastFiles)
			for name, content := range tt.inline {
				writeTestFile(t, filepath.Join(dir, name), content)
			}
			got := call{dir: dir, stdin: tt.stdin}.run(t, tt.args...)
			assert.Equal(t, tt.want(t), got)
			if check := checks[tt.name]; check != nil {
				check(t, dir)
			}
		})
	}
}

func TestWriteOptionsOnlyForTheirFormat(t *testing.T) {
	t.Parallel()

	rate := model.FrameRate{Num: 12, Den: 1}
	w := writeOutput{outputFPS: rate, sccFit: true}
	want := map[stbox.Format]stbox.WriteOptions{
		stbox.MicroDVD:  {MicroDVD: microdvd.WriteOptions{FrameRate: rate}},
		stbox.SpruceSTL: {SpruceSTL: sprucestl.WriteOptions{FrameRate: rate}},
		stbox.ITT:       {ITT: itt.WriteOptions{FrameRate: rate}},
		stbox.MCC:       {MCC: mcc.WriteOptions{FrameRate: rate}},
		stbox.SCC:       {SCC: scc.WriteOptions{Fit: true}},
	}
	for _, f := range stbox.Formats() {
		if f.CanWrite() {
			assert.Equal(t, want[f], w.optionsFor(f), f)
		}
	}
	require.NoError(t, w.optionsFor(stbox.SubRip).Validate())
	assert.Error(t, w.optionsFor(stbox.MCC).Validate())
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestBroadcastOptionsOnEveryCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  []string
		extra []string
	}{
		{"info roll-up", []string{"info", "trip.srt"}, []string{"--scc-roll-up", "lines"}},
		{"validate roll-up", []string{"validate", "trip.srt", "--preset", "bbc"}, []string{"--scc-roll-up", "lines"}},
		{"diff roll-up", []string{"diff", "trip.srt", "shop.vtt"}, []string{"--scc-roll-up", "lines"}},
		{"sync roll-up", []string{"sync", "trip.srt", "--reference", "shop.vtt"}, []string{"--scc-roll-up", "lines"}},
		{"sync fit", []string{"sync", "trip.srt", "--reference", "shop.vtt"}, []string{"--scc-fit"}},
		{"dual roll-up", []string{"dual", "--primary", "trip.srt", "--secondary", "shop.vtt"}, []string{"--scc-roll-up", "lines"}},
		{"dual fit", []string{"dual", "--primary", "trip.srt", "--secondary", "shop.vtt"}, []string{"--scc-fit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			assert.Equal(t, call{dir: dir}.run(t, tt.base...), call{dir: dir}.run(t, append(tt.base, tt.extra...)...))
		})
	}
}
