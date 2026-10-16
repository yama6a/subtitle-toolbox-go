package cli

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// summaryCommand is a command with a name and a summary only, for the layout of the global help.
type summaryCommand struct{ n, s string }

func (c summaryCommand) name() string                             { return c.n }
func (c summaryCommand) summary() string                          { return c.s }
func (summaryCommand) usage() []string                            { return []string{"[options]"} }
func (summaryCommand) details() string                            { return "" }
func (summaryCommand) options() []option                          { return nil }
func (summaryCommand) run(context.Context, *args, *console) error { return nil }

const globalHelpTail = `
Run "stbox help <command>" or "stbox <command> --help" for the options of a
command. A file argument of - reads standard input. One input goes to standard
output, or to -o FILE. Several inputs need --output-dir DIR. The tool never
overwrites a file.

Exit codes:
  0    Success.
  2    Invalid arguments.
  3    A file could not be read or written.
  130  Stopped by Ctrl-C or SIGTERM. Finished outputs stay.

Options:
  -h, --help     Show this help.
  -V, --version  Print the version.
`

func TestGlobalHelp(t *testing.T) {
	t.Parallel()

	head := "stbox 1.2.3\nConverts, retimes, checks and fixes subtitle files.\n\nUsage: stbox <command> [<file>...] [options]\n\n"
	t.Run("binary", func(t *testing.T) {
		t.Parallel()

		want := head + `Commands:
  info     Print the format, the encoding and the cue count of subtitle files.
  formats  List the format names and file extensions for --from and --to.
  help     Show the help of a command.
` + globalHelpTail
		assert.Equal(t, result{Stdout: want}, call{commands: commands()}.run(t))
	})
	t.Run("1.0 commands", func(t *testing.T) {
		t.Parallel()

		commands := []command{
			summaryCommand{"convert", "Convert subtitle files to another format, and run text, structure and timing edits on the way."},
			summaryCommand{"info", "Print the format, the encoding and the cue count of subtitle files."},
			summaryCommand{"formats", "List the format names and file extensions for --from and --to."},
		}
		want := head + `Commands:
  convert  Convert subtitle files to another format, and run text, structure and
           timing edits on the way.
  info     Print the format, the encoding and the cue count of subtitle files.
  formats  List the format names and file extensions for --from and --to.
  help     Show the help of a command.
` + globalHelpTail
		assert.Equal(t, result{Stdout: want}, call{commands: commands}.run(t))
	})
}

func TestDispatch(t *testing.T) {
	t.Parallel()

	help := call{}.run(t).Stdout
	convertHelp := call{}.run(t, "convert", "--help").Stdout
	unknown := func(name string) result { return usageResult("", `Unknown command "`+name+`".`) }
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{"no word", nil, result{Stdout: help}},
		{"long help", []string{"--help"}, result{Stdout: help}},
		{"short help", []string{"-h"}, result{Stdout: help}},
		{"help", []string{"help"}, result{Stdout: help}},
		{"help of help", []string{"help", "help"}, result{Stdout: help}},
		{"long version", []string{"--version"}, result{Stdout: testVersion + "\n"}},
		{"short version", []string{"-V"}, result{Stdout: testVersion + "\n"}},
		{"version ignores later words", []string{"--version", "x", "--nope"}, result{Stdout: testVersion + "\n"}},
		{"help of a command", []string{"help", "convert"}, result{Stdout: convertHelp}},
		{"help of a command with a topic", []string{"help", "convert", "out.srt"}, result{Stdout: convertHelp}},
		{"unknown command", []string{"merge"}, unknown("merge")},
		{"help of an unknown command", []string{"help", "merge"}, unknown("merge")},
		{"help of an option", []string{"help", "--help"}, unknown("--help")},
		{"version after a command", []string{"convert", "-V"}, usageResult("convert", "Unknown option -V.")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{}.run(t, tt.words...))
		})
	}
	for _, removed := range []string{"retime", "shift", "scale", "fps", "sync-fps", "fix", "strip-sdh", "snap"} {
		assert.Equal(t, unknown(removed), call{}.run(t, removed, "-", "--by", "1"))
		assert.NotRegexp(t, "(?m)^  "+removed+" +\\S", help)
	}
}

func TestHelpScan(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	convertHelp := call{dir: dir}.run(t, "convert", "--help")
	require.Equal(t, 0, convertHelp.Code)
	tests := []struct {
		name  string
		words []string
	}{
		{"help wins over an unknown option", []string{"convert", "--bogus", "--help"}},
		{"help wins over a value option without its value", []string{"convert", "--to", "--help"}},
		{"short help", []string{"convert", "-h"}},
		{"help before a file argument", []string{"convert", "trip.srt", "-h", "out.srt"}},
		{"help with a topic that starts with a dash", []string{"convert", "-h", "-x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, convertHelp, call{dir: dir}.run(t, tt.words...))
		})
	}

	t.Run("help after -- is a file", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, result{Code: exitFile, Stderr: "--help: The file does not exist.\n"},
			call{dir: dir}.run(t, "convert", "--to", "vtt", "--", "--help"))
	})
}

func TestHelpTopic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		topic string
	}{
		{"no topic", []string{"--help"}, ""},
		{"topic", []string{"--help", "all"}, "all"},
		{"short help with a topic", []string{"a.srt", "-h", "text"}, "text"},
		{"dash word is no topic", []string{"--help", "--to"}, ""},
		{"first help counts", []string{"-h", "x", "--help", "y"}, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			topic, ok := helpRequest(tt.words)
			assert.True(t, ok)
			assert.Equal(t, tt.topic, topic)
		})
	}
	_, ok := helpRequest([]string{"a.srt", "--", "--help"})
	assert.False(t, ok)
}

type topicCommand struct{ summaryCommand }

func (topicCommand) help(topic string) (string, error) {
	if topic != "" {
		return "", usage(`Unknown option group "` + topic + `".`)
	}
	return "topics\n", nil
}

func TestHelpErrorIsAUsageErrorOfTheCommand(t *testing.T) {
	t.Parallel()

	cl := call{commands: []command{topicCommand{summaryCommand{"convert", "Convert."}}}}
	assert.Equal(t, result{Stdout: "topics\n"}, cl.run(t, "convert", "--help"))
	want := usageResult("convert", `Unknown option group "timings".`)
	assert.Equal(t, want, cl.run(t, "convert", "--help", "timings"))
	assert.Equal(t, want, cl.run(t, "help", "convert", "timings"))
}

func TestCommandHelp(t *testing.T) {
	t.Parallel()

	want := `Usage: stbox convert <input> [--to FORMAT] [-o FILE] [options]
       stbox convert <input>... [--to FORMAT] --output-dir DIR [options]

Write subtitle files in the format of --to.

Options:
  --to FORMAT            Output format. Default: the input format.
  -o, --output FILE      Write the output of one input to this file in place of
                         standard output. The file must not exist.
  --output-dir DIR       Write each output into this directory, with the base
                         name of its input. Several inputs need it.
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
  --keep-going           Go on with the next file after a file fails. Default:
                         stop at the first failure.
  -h, --help             Show this help.
`
	assert.Equal(t, result{Stdout: want}, call{}.run(t, "convert", "--help"))

	info := call{}.run(t, "help", "info").Stdout
	assert.Contains(t, info, "\nOptions:\n  --json           Print JSON: a list with one object for each input file.\n  --from FORMAT  ")
}

func TestCommandHelpWithDetails(t *testing.T) {
	t.Parallel()

	cmd := detailsCommand{summaryCommand{"formats", "List the format names and file extensions for --from and --to."}}
	want := `Usage: stbox formats

List the format names and file extensions for --from and --to.

When two formats share an extension, the first one in the list owns it. So .json
is the JSON of this library. Pass --from to read another one.

Options:
  -h, --help  Show this help.
`
	assert.Equal(t, want, commandHelp(cmd))
}

type detailsCommand struct{ summaryCommand }

func (detailsCommand) usage() []string { return []string{""} }
func (detailsCommand) details() string {
	return "When two formats share an extension, the first one in the list owns it. So .json is the JSON of this " +
		"library. Pass --from to read another one."
}

func TestEveryCommandHasHelp(t *testing.T) {
	t.Parallel()

	for _, c := range testCommands() {
		cmd := c.name()
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()

			got := call{}.run(t, cmd, "--help")
			assert.Equal(t, 0, got.Code)
			assert.Regexp(t, "^Usage: stbox "+cmd+"[ \n]", got.Stdout)
			assert.Contains(t, got.Stdout, "\n  -h, --help ")
			assert.Empty(t, got.Stderr)
			assert.Equal(t, got, call{}.run(t, "help", cmd))
			assert.Equal(t, got, call{}.run(t, cmd, "-h"))
		})
	}
}

func TestEveryHelpPageFitsIn80ColumnsWithoutTrailingSpaces(t *testing.T) {
	t.Parallel()

	pages := map[string]string{
		"help":        call{}.run(t, "help").Stdout,
		"binary help": call{commands: commands()}.run(t, "help").Stdout,
	}
	for _, cmd := range testCommands() {
		pages["help "+cmd.name()] = call{}.run(t, "help", cmd.name()).Stdout
	}
	for page, text := range pages {
		require.NotEmpty(t, text, page)
		for line := range strings.SplitSeq(text, "\n") {
			assert.LessOrEqual(t, len(line), helpWidth, "%s: %s", page, line)
			assert.Equal(t, rtrim(line), line, "%s: trailing space", page)
		}
	}
}

func TestOptionNamesFollowTheNamingRules(t *testing.T) {
	t.Parallel()

	for _, cmd := range testCommands() {
		for _, o := range cmd.options() {
			where := cmd.name() + " --" + o.name
			if regexp.MustCompile(`(^|-)no-`).MatchString(o.name) {
				assert.Empty(t, o.value, "%s takes a value", where)
			}
			if strings.HasSuffix(o.name, "-frames") || o.value == "FRAMES" {
				assert.Equal(t, "FRAMES", o.value, "%s must count frames", where)
				assert.True(t, strings.HasSuffix(o.name, "-frames"), "%s counts frames", where)
			}
			if o.value == "SECONDS" {
				assert.False(t, strings.HasSuffix(o.name, "-frames"), "%s is in seconds", where)
			}
			assert.NotEqual(t, "help", o.name, where)
		}
	}
}

func TestEachHelpDefaultIsTheDefaultThatTheCommandUses(t *testing.T) {
	t.Parallel()

	codeDefaults := map[string]string{}
	for _, le := range lineEndings {
		if le.value == (stbox.WriteOptions{}).LineEnding {
			codeDefaults["line-ending"] = le.name
		}
	}
	described := []string{
		"the format detected from the content", "stop at the first failure", "the input format",
		"the BOM rule of the output format",
	}
	unwrap := regexp.MustCompile(`\n {3,}(\S)`)
	defaultText := regexp.MustCompile(`(?m)^  (?:-\w, )?--([\w-]+).*? Default: (.+?)\.(?: |$)`)
	found := map[string]bool{}
	for _, c := range testCommands() {
		cmd := c.name()
		help := unwrap.ReplaceAllString(call{}.run(t, cmd, "--help").Stdout, " $1")
		for _, m := range defaultText.FindAllStringSubmatch(help, -1) {
			opt, value := m[1], m[2]
			found[opt] = true
			if want, ok := codeDefaults[opt]; ok {
				assert.Equal(t, want, value, "%s --%s", cmd, opt)
				continue
			}
			assert.Contains(t, described, value, "%s --%s", cmd, opt)
		}
	}
	for opt := range codeDefaults {
		assert.True(t, found[opt], "--%s", opt)
	}
}

func TestRemovedOptionsAreUnknown(t *testing.T) {
	t.Parallel()

	for _, opt := range []string{"in-place", "force"} {
		dir := newDir(t)
		before := snapshot(t, dir)
		assert.Equal(t, usageResult("convert", "Unknown option --"+opt+"."),
			call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "--"+opt))
		assert.Equal(t, before, snapshot(t, dir))
		assert.NotRegexp(t, "--"+opt+`\b`, call{}.run(t, "convert", "--help").Stdout)
	}
}
