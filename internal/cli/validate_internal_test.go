package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
)

const validateHelp = `Usage: stbox validate <input>... --preset netflix-en|bbc [options]
       stbox validate <input>... [--max-cpl CHARS] [--check-overlaps] [...]
                      [options]

Check the cues against reading speed, line length, timing and text rules.

Prints one line per broken rule. Cue numbers start at 1. The exit code is 1 when
a file breaks a rule.
The netflix-en preset has the limits of the Netflix English (USA) Timed Text
Style Guide. These are 20 characters per second, 42 characters per line and 2
lines. A cue lasts 0.833 s to 7 s and keeps a gap of 2 frames, with no overlaps.
The bbc preset has the limits of the BBC Subtitle Guidelines. These are 37
characters per line, 180 words per minute and 0.3 s per word.
A rule option overrides the value of the preset.

Options:
  --preset NAME                       Rule set: netflix-en or bbc.
  --video-fps RATE                    Frame rate of the video, for the 2-frame
                                      gap of netflix-en. Default: 23.976.
  --max-cps CHARS                     Maximum characters per second.
  --max-cpl CHARS                     Maximum characters per line.
  --max-lines LINES                   Maximum lines per cue.
  --min-duration SECONDS              Minimum duration of a cue.
  --max-duration SECONDS              Maximum duration of a cue.
  --min-gap SECONDS                   Minimum gap between cues.
  --check-overlaps                    Report overlapping cues.
  --check-empty-cues                  Report cues without text.
  --max-wpm WORDS                     Maximum words per minute.
  --min-seconds-per-word SECONDS      Minimum duration of a cue per word.
  --max-speakers SPEAKERS             Maximum speakers per cue, from dialogue
                                      dashes or <v> names.
  --dialogue-dash STYLE               Report dialogue dashes in another style
                                      than STYLE, for example "- " or "-".
  --allowed-characters LIST           Report other characters. LIST is a list or
                                      a class such as "[A-Za-z0-9 .,!?]".
  --check-double-spaces               Report two or more spaces between words.
  --check-leading-or-trailing-spaces  Report lines that start or end with a
                                      space.
  --check-unbalanced-tags             Report formatting tags without a partner
                                      tag.
  --check-all-caps-lines              Report lines in upper case only.
  --json                              Print JSON: a list with one object for
                                      each input file.
  --from FORMAT                       Input format. Default: the format detected
                                      from the content. When detection finds no
                                      format, the file extension sets it.
  --encoding NAME                     Encoding of input that is not UTF-8, for
                                      example Windows-1252. It replaces code
                                      page detection. A BOM, valid UTF-8 and
                                      UTF-16 input win over it.
  --lenient                           Skip or repair broken cues and print a
                                      warning for each.
  --input-fps RATE                    Frame rate of a MicroDVD input without a
                                      {1}{1}<fps> first line, of Spruce STL
                                      timecodes, and of CSV or TSV times in
                                      hh:mm:ss:ff.
  --fps RATE                          Sets --input-fps and --video-fps. A
                                      specific option wins over --fps.
  --scc-roll-up MODE                  How SCC and MCC input reads roll-up
                                      captions: screen gives one cue per screen,
                                      lines gives one cue per row. Default:
                                      screen.
  --keep-going                        Go on with the next file after a file
                                      fails. Default: stop at the first failure.
  -h, --help                          Show this help.
`

func TestValidateHelp(t *testing.T) {
	t.Parallel()

	assert.Equal(t, result{Stdout: validateHelp}, call{}.run(t, "validate", "--help"))
}

func TestValidateTextRules(t *testing.T) {
	t.Parallel()

	vtt := "WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:04.000\n-Where is the bus?\n- At the <i>corner.\n\n" +
		"00:00:05.000 --> 00:00:08.000\nTHE BUS IS LATE\nWait <i> here</i>\n\n" +
		"00:00:09.000 --> 00:00:12.000\n<v Anna>Rain today.\n<v Ben>Sun tomorrow.\n<v Cleo>Snow later.\n\n" +
		"00:00:13.000 --> 00:00:15.000\n&nbsp;Caf\u00e9 open.\n"
	want := result{Code: exitResult, Stdout: "stdin: cue 1: noUnbalancedTags 1\n" +
		"stdin: cue 1: dialogueDashStyle 1\n" +
		"stdin: cue 2: noDoubleSpaces 1\n" +
		"stdin: cue 2: noAllCapsLines 1\n" +
		"stdin: cue 3: maxSpeakersPerCue 3, limit 2\n" +
		"stdin: cue 4: noLeadingOrTrailingSpaces 1\n" +
		"stdin: cue 4: allowedCharacters 2\n"}

	assert.Equal(t, want, call{stdin: vtt}.run(t, "validate", "-", "--dialogue-dash", "- ", "--check-unbalanced-tags",
		"--check-all-caps-lines", "--check-double-spaces", "--check-leading-or-trailing-spaces", "--max-speakers", "2",
		"--allowed-characters", `[A-Za-z0-9 .,!?<>/\-]`))
}

func TestValidateEveryRuleOption(t *testing.T) {
	t.Parallel()

	srt := "1\n00:00:01,000 --> 00:00:01,200\n- A\u00a0\u00a0B\u00a0\n- <i>C\n\n2\n00:00:01,100 --> 00:00:01,150\n\n"
	got := call{stdin: srt}.run(t, "validate", "-", "--max-cps", "1", "--max-cpl", "1", "--max-lines", "1", "--min-duration", "1",
		"--max-duration", "0.001", "--min-gap", "5", "--check-overlaps", "--check-empty-cues", "--max-wpm", "1",
		"--min-seconds-per-word", "00:00.5", "--max-speakers", "1", "--dialogue-dash", "-", "--allowed-characters", "x",
		"--check-double-spaces", "--check-leading-or-trailing-spaces", "--check-unbalanced-tags", "--check-all-caps-lines")

	assert.Equal(t, exitResult, got.Code)
	for _, rule := range []string{
		"maxCharactersPerSecond", "maxCharactersPerLine", "maxLinesPerCue", "minDuration", "maxDuration", "noOverlap", "noEmptyCues",
		"maxWordsPerMinute", "minSecondsPerWord", "maxSpeakersPerCue", "dialogueDashStyle", "allowedCharacters", "noDoubleSpaces",
		"noLeadingOrTrailingSpaces", "noUnbalancedTags", "noAllCapsLines",
	} {
		assert.Contains(t, got.Stdout, ": "+rule+" ", rule)
	}
}

func TestValidateJSON(t *testing.T) {
	t.Parallel()

	srt := "1\n00:00:01,000 --> 00:00:01,000\nHello\n\n2\n00:00:01,000 --> 00:00:03,000\nHi\n\n3\n00:00:02,500 --> 00:00:04,000\nYo\n"
	dir := newDir(t)
	tests := []struct {
		name   string
		stdin  string
		words  []string
		want   string
		code   int
		stderr string
	}{
		{
			"a limit", "",
			[]string{"trip.srt", "--max-cpl", "42"},
			`[
    {
        "file": "trip.srt",
        "format": "srt",
        "valid": false,
        "violations": [
            {
                "cueIndex": 1,
                "rule": "maxCharactersPerLine",
                "value": 57,
                "infinite": false,
                "limit": 42
            }
        ],
        "warnings": []
    }
]
`, exitResult, "",
		},
		{
			"infinite value and no limit", srt,
			[]string{"-", "--max-cps", "20", "--check-overlaps"},
			`[
    {
        "file": "-",
        "format": "srt",
        "valid": false,
        "violations": [
            {
                "cueIndex": 0,
                "rule": "maxCharactersPerSecond",
                "value": null,
                "infinite": true,
                "limit": 20
            },
            {
                "cueIndex": 2,
                "rule": "noOverlap",
                "value": 0.5,
                "infinite": false,
                "limit": null
            }
        ],
        "warnings": []
    }
]
`, exitResult, "",
		},
		{
			"valid with warnings", "",
			[]string{"broken.srt", "--max-cpl", "42", "--lenient"},
			`[
    {
        "file": "broken.srt",
        "format": "srt",
        "valid": true,
        "violations": [],
        "warnings": [
            {
                "lineNumber": 6,
                "blockIndex": 1,
                "message": "Block #1 has no timing line on its second line. The line is \"00:00:03,000 => 00:00:04,000\".",
                "action": "skipped"
            }
        ]
    }
]
`, 0, "broken.srt: line 6: Block #1 has no timing line on its second line. The line is \"00:00:03,000 => 00:00:04,000\". (skipped)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir, stdin: tt.stdin}.run(t, append([]string{"validate"}, append(tt.words, "--json")...)...)
			assert.Equal(t, result{Code: tt.code, Stdout: tt.want, Stderr: tt.stderr}, got)
		})
	}
}

func TestValidateBatch(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		result{Code: exitResult, Stdout: "trip.srt: cue 2: noOverlap 0.5\nshop.vtt: no problems\n2 files: 1 valid, 1 with problems, 0 failed.\n"},
		runCLI(t, "validate", "trip.srt", "shop.vtt", "--check-overlaps"))
	assert.Equal(t, result{Stdout: "shop.vtt: no problems\n"}, runCLI(t, "validate", "shop.vtt", "--max-cps", "30"))

	got := runCLI(t, "validate", "trip.srt", "notes.txt", "shop.vtt", "--check-overlaps", "--keep-going")
	assert.Equal(t, exitFile, got.Code)
	assert.Equal(t, "trip.srt: cue 2: noOverlap 0.5\nshop.vtt: no problems\n3 files: 1 valid, 1 with problems, 1 failed.\n", got.Stdout)
	got = runCLI(t, "validate", "trip.srt", "notes.txt", "shop.vtt", "--check-overlaps")
	assert.Equal(t, exitFile, got.Code)
	assert.Equal(t, "trip.srt: cue 2: noOverlap 0.5\n3 files: 0 valid, 1 with problems, 1 failed, 1 skipped.\n", got.Stdout)
}

func TestValidateUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"unknown option", []string{"trip.srt", "--no-overlap"}, "Unknown option --no-overlap."},
		{"no rule option", []string{"shop.vtt"}, "Pass --preset or at least one rule option."},
		{"unknown preset", []string{"trip.srt", "--preset", "nope"}, `The option --preset must be netflix-en or bbc, got "nope".`},
		{
			"video-fps without netflix-en",
			[]string{"trip.srt", "--preset", "bbc", "--video-fps", "25"},
			"--video-fps sets the frame rate of the netflix-en gap rule. Pass --preset netflix-en, or leave out --video-fps.",
		},
		{"bad video-fps", []string{"trip.srt", "--preset", "netflix-en", "--video-fps", "0"}, "The option --video-fps must be greater than 0."},
		{"invalid class", []string{"-", "--allowed-characters", "[z-a]"}, `The allowed characters "[z-a]" are not a valid character class: [z-a] range in reverse order.`},
		{
			"POSIX class",
			[]string{"-", "--allowed-characters", "[[:alpha:]]"},
			`The allowed characters "[[:alpha:]]" hold a POSIX class such as [:alpha:]. Use a Unicode class such as \p{L}.`,
		},
		{
			"dash style",
			[]string{"-", "--dialogue-dash", "x"},
			`The option --dialogue-dash must be a hyphen, a Unicode hyphen, an en dash or an em dash, with or without one space after it, got "x".`,
		},
		{"zero cps", []string{"-", "--max-cps", "0"}, "The option --max-cps must be greater than 0."},
		{"cpl not whole", []string{"-", "--max-cpl", "4.5"}, `The option --max-cpl needs a whole number of 1 or more, got "4.5".`},
		{"negative duration", []string{"-", "--min-duration", "-1"}, "The option --min-duration must be greater than 0."},
		{"first error wins", []string{"-", "--max-wpm", "x", "--max-cpl", "0"}, `The option --max-cpl needs a whole number of 1 or more, got "0".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, usageResult("validate", tt.msg), runCLI(t, append([]string{"validate"}, tt.words...)...))
		})
	}
}

func TestValidateInputOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			"input frame rate",
			[]string{"frames.sub", "--input-fps", "25", "--max-duration", "1"},
			result{Code: exitResult, Stdout: "frames.sub: cue 1: maxDuration 2, limit 1\nframes.sub: cue 2: maxDuration 2, limit 1\n"},
		},
		{"frame rate", []string{"frames.sub", "--fps", "50", "--max-duration", "1"}, result{Stdout: "frames.sub: no problems\n"}},
		{"format", []string{"trip.srt", "--from", "srt", "--max-cpl", "99"}, result{Stdout: "trip.srt: no problems\n"}},
		{
			"encoding",
			[]string{"latin1.srt", "--encoding", "Windows-1252", "--max-cpl", "3"},
			result{Code: exitResult, Stdout: "latin1.srt: cue 1: maxCharactersPerLine 13, limit 3\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, runCLI(t, append([]string{"validate"}, tt.words...)...))
		})
	}
}

// Ports testValidateWithThePreset, testValidateTakesTheVideoFrameRateForTheGap, testValidateWithTheBbcPreset and
// testRuleOptionsOverrideTheBase of subtitle-toolbox-php.
func TestValidatePresets(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, testfiles.Path(t, "validation/own_netflix_checks.srt"), filepath.Join(dir, "checks.srt"))
	tests := []struct {
		name  string
		words []string
		want  string
		code  int
	}{
		{"netflix", []string{"trip.srt", "--preset", "NETFLIX-EN"}, "trip.srt: cue 2: maxCharactersPerLine 57, limit 42\n" +
			"trip.srt: cue 2: maxCharactersPerSecond 27.6, limit 20\n" + "trip.srt: cue 2: noOverlap 0.5\n" +
			"trip.srt: cue 3: maxCharactersPerSecond 42.5, limit 20\n" + "trip.srt: cue 3: minDuration 0.4, limit 0.833\n", exitResult},
		{"netflix with override", []string{"shop.vtt", "--preset", "netflix-en", "--max-cps", "30"}, "shop.vtt: no problems\n", 0},
		{"bbc", []string{"trip.srt", "--preset", "bbc"}, "trip.srt: cue 2: maxCharactersPerLine 57, limit 37\n" +
			"trip.srt: cue 2: maxWordsPerMinute 336, limit 180\n" + "trip.srt: cue 2: minSecondsPerWord 0.179, limit 0.3\n" +
			"trip.srt: cue 3: maxWordsPerMinute 450, limit 180\n" + "trip.srt: cue 3: minSecondsPerWord 0.133, limit 0.3\n", exitResult},
		{
			"bbc with overrides",
			[]string{"trip.srt", "--preset", "bbc", "--max-wpm", "500", "--min-seconds-per-word", "00:00.1"},
			"trip.srt: cue 2: maxCharactersPerLine 57, limit 37\n", exitResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, result{Code: tt.code, Stdout: tt.want}, call{dir: dir}.run(t, append([]string{"validate"}, tt.words...)...))
		})
	}

	gaps := []struct {
		name  string
		words []string
		want  string
	}{
		{"default video rate", []string{"--preset", "netflix-en"}, "checks.srt: cue 2: minGap 0.04, limit 0.083\n"},
		{"video rate 25", []string{"--preset", "netflix-en", "--video-fps", "25"}, "checks.srt: cue 2: minGap 0.04, limit 0.08\n"},
		{"fps 25", []string{"--preset", "netflix-en", "--fps", "25"}, "checks.srt: cue 2: minGap 0.04, limit 0.08\n"},
		{"video rate 50", []string{"--preset", "netflix-en", "--video-fps", "50"}, ""},
	}
	for _, tt := range gaps {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, append([]string{"validate", "checks.srt"}, tt.words...)...)
			var gap string
			for line := range strings.Lines(got.Stdout) {
				if strings.Contains(line, ": minGap ") {
					gap += line
				}
			}
			assert.Equal(t, tt.want, gap)
		})
	}
}
