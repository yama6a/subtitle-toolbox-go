package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const convertCommonHelp = `Usage: stbox convert <input> [--to FORMAT] [-o FILE] [options]
       stbox convert <input>... [--to FORMAT] --output-dir DIR [options]

Convert subtitle files to another format, and run text, structure and timing
edits on the way.

Without --to, the output keeps the format of the input. Without -o or
--output-dir, the output goes to standard output. An input argument can be a
file, a directory, a glob such as "season1/*.srt", or -. The tool never
overwrites a file.

Options:
  --to FORMAT            Output format. Default: the input format.
  -o, --output FILE      Write the output of one input to this file in place of
                         standard output. The file must not exist.
  --output-dir DIR       Write each output into this directory, with the base
                         name of its input. Several inputs need it.
  --output-fps RATE      Frame rate of MicroDVD output. Default: the frame rate
                         of a MicroDVD input.
  --line-ending lf|crlf  Line ending of the output. Default: lf.
  --bom                  Start the output with a UTF-8 BOM.
  --no-bom               Write no UTF-8 BOM. Default: the BOM rule of the output
                         format.
  --from FORMAT          Input format. Default: the format detected from the
                         content. When detection finds no format, the file
                         extension sets it.
  --encoding NAME        Encoding of input that is not UTF-8, for example
                         Windows-1252. It replaces code page detection. A BOM,
                         valid UTF-8 and UTF-16 input win over it.
  --lenient              Skip or repair broken cues and print a warning for
                         each.
  --input-fps RATE       Frame rate of a MicroDVD input without a {1}{1}<fps>
                         first line.
  --fps RATE             Sets --input-fps and --output-fps. A specific option
                         wins over --fps.
  --keep-going           Go on with the next file after a file fails. Default:
                         stop at the first failure.
  --language CODE        Language for --case, for example en, de-AT or tr.
  -h, --help             Show this help.
`

const convertGroupList = `
Option groups, in the order that convert runs their edits:
  forced     Keep only the forced cues.
  sdh        Remove hearing-impaired annotations.
  replace    Replace words or patterns in the text.
  text       Change the case, remove tags, fix right-to-left text.
  structure  Join, wrap and unwrap cues.
  retime     Shift and scale the times, or change the frame rate.
  timing     Fix overlaps, short cues and long cues, and add lead-in and
             lead-out.
  ass        Set how ASS output writes word timestamps and the Default style.

Run "stbox convert --help GROUP" for the options of a group,
or "stbox convert --help all" for all options.
`

var convertGroupHelp = []string{
	`forced: Keep only the forced cues.
  --forced-only  Keep only the forced cues, for example the translations of
                 signs.
`,
	`sdh: Remove hearing-impaired annotations.
  --sdh                       Remove hearing-impaired annotations such as [DOOR
                              SLAMS], (laughs) and JOHN:. Remove a cue with no
                              text left.
  --sdh-keep-square-brackets  Keep text in square brackets, such as [DOOR
                              SLAMS].
  --sdh-keep-parentheses      Keep text in parentheses, such as (laughs).
  --sdh-keep-speaker-labels   Keep speaker labels, such as JOHN:.
  --sdh-keep-music-lines      Keep lines that hold only music symbols.
  --sdh-any-case-labels       Also remove speaker labels that are not upper
                              case, such as Baker:.
  --sdh-lyrics                Remove text between two music symbols.
  --sdh-brackets PAIR         Also remove text between this pair of characters,
                              for example "{}" or "**". Repeatable.
`,
	`replace: Replace words or patterns in the text.
  --replace FROM=TO      Replace FROM with TO in the text between tags, for
                         example --replace colour=color. Repeatable.
  --replace-regex        Read each FROM of --replace as a regular expression
                         with delimiters, such as /\.{4,}/. TO can use $1.
  --replace-ignore-case  Match FROM of --replace in any case.
`,
	`text: Change the case, remove tags, fix right-to-left text.
  --case MODE   Change the case of the text between tags: upper, lower or
                sentence.
  --strip-tags  Remove all formatting tags, such as <i> and <font>, from the cue
                text.
  --rtl MODE    Right-to-left text: fix wraps each Arabic or Hebrew line in
                Unicode embedding marks, clean removes all bidi controls.
`,
	`structure: Join, wrap and unwrap cues.
  --structure-unwrap            Join the lines of each cue with a space. Each
                                dialogue dash line starts a line of its own.
  --structure-merge-short       Join cues shorter than 1 s with a neighbor at
                                most 0.25 s away. The joined cue must fit 7 s,
                                --structure-max-cpl and --structure-max-lines.
  --structure-wrap              Break lines longer than --structure-max-cpl
                                characters.
  --structure-max-cpl CHARS     Maximum characters per line for --structure-wrap
                                and --structure-merge-short. Default: 42.
  --structure-max-lines LINES   Maximum number of lines per cue for
                                --structure-wrap and --structure-merge-short.
                                Default: 2.
  --structure-merge-duplicates  Join same-text cues that overlap or touch.
  --structure-merge-same-time   Join cues with the same start and end into one
                                cue with the lines of all of them.
`,
	`retime: Shift and scale the times, or change the frame rate.
  --shift SECONDS         Time to add to every time, for example 2.5 or
                          00:00:02.500, or -2.5 to show the cues earlier.
  --shift-after SECONDS   Shift only the cues that start at this time or later.
  --shift-before SECONDS  Shift only the cues that start before this time.
  --scale FACTOR          Multiply every time by this factor. Must be greater
                          than 0.
  --sync OLD=NEW          Move the time OLD to NEW and correct the times
                          between. OLD is a time, first, last or #N, the start
                          of cue N. Repeatable.
  --from-fps RATE         Frame rate of the video that the subtitle fits now.
                          Needs --to-fps.
  --to-fps RATE           Frame rate of the video that the subtitle must fit.
                          Needs --from-fps.
`,
	`timing: Fix overlaps, short cues and long cues, and add lead-in and lead-out.
  --timing-fix-overlaps          End each cue at least --timing-min-gap seconds
                                 before the next cue starts.
  --timing-min-duration SECONDS  Show each cue for at least this time where the
                                 next cue allows it.
  --timing-lead-in SECONDS       Start each cue this time earlier where the
                                 previous cue allows it.
  --timing-lead-out SECONDS      End each cue this time later where the next cue
                                 allows it. The lead-out comes before the
                                 lead-in.
  --timing-max-duration SECONDS  End each cue at most this time after its start.
                                 Runs after the other timing fixes.
  --timing-min-gap SECONDS       Gap between cues for --timing-fix-overlaps,
                                 --timing-min-duration and the lead options.
                                 Default: 0.
`,
	`ass: Set how ASS output writes word timestamps and the Default style.
  --ass-karaoke-tag TAG  ASS karaoke tag for word timestamps: k, kf or ko.
                         Default: k.
  --ass-style STYLE      Change the Default style, for example
                         'Fontname=Roboto,Fontsize=48,Outline=2'.
`,
}

func TestConvertHelp(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "timings"), nil, 0o600))
	defaultHelp := result{Stdout: convertCommonHelp + convertGroupList}
	all := result{Stdout: convertCommonHelp + "\n" + strings.Join(convertGroupHelp, "\n")}
	unknown := func(topic string) result {
		return usageResult("convert", "Unknown option group "+quote(topic)+
			". The groups are forced, sdh, replace, text, structure, retime, timing, ass, and all for every option.")
	}
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{"default", []string{"convert", "--help"}, defaultHelp},
		{"help command", []string{"help", "convert"}, defaultHelp},
		{"all", []string{"convert", "--help", "all"}, all},
		{"help command with all", []string{"help", "convert", "all"}, all},
		{"file after -h", []string{"convert", "in.srt", "-h", "out.srt"}, defaultHelp},
		{"path with a slash", []string{"convert", "-h", "out/x"}, defaultHelp},
		{"path with a backslash", []string{"convert", "-h", `out\x`}, defaultHelp},
		{"existing file", []string{"convert", "-h", "timings"}, defaultHelp},
		{"unknown group", []string{"convert", "--help", "nope"}, unknown("nope")},
		{"unknown group of the help command", []string{"help", "convert", "nope"}, unknown("nope")},
		{"group names match in case", []string{"convert", "--help", "Timing"}, unknown("Timing")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: dir}.run(t, tt.words...))
		})
	}
	for i, g := range convertGroups() {
		t.Run("group "+g.name, func(t *testing.T) {
			t.Parallel()

			want := result{Stdout: convertGroupHelp[i]}
			assert.Equal(t, want, call{dir: dir}.run(t, "convert", "--help", g.name))
			assert.Equal(t, want, call{dir: dir}.run(t, "help", "convert", g.name))
		})
	}
	for line := range strings.SplitSeq(all.Stdout, "\n") {
		assert.LessOrEqual(t, len(line), helpWidth, line)
		assert.Equal(t, rtrim(line), line)
	}
}
