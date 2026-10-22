package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

const formatsHelp = `Usage: stbox formats

List the format names and file extensions for --from and --to.

When two formats share an extension, the first one in the list owns it. So .json
is the JSON of this library. Pass --from to read another one.

Options:
  -h, --help  Show this help.
`

func TestFormatsHelp(t *testing.T) {
	t.Parallel()

	want := result{Stdout: formatsHelp}
	assert.Equal(t, want, call{}.run(t, "formats", "--help"))
	assert.Equal(t, want, call{}.run(t, "formats", "-h"))
	assert.Equal(t, want, call{}.run(t, "help", "formats"))
}

func TestFormatsPrintsTheListOf1x(t *testing.T) {
	t.Parallel()

	//nolint:dupword // the output repeats yes
	want := strings.Join([]string{
		"Name  Extensions        Read  Write",
		"ass   .ass .ssa         yes   yes",
		"json  .json             yes   yes",
		"sami  .smi .sami        yes   yes",
		"srt   .srt              yes   yes",
		"ttml  .ttml .dfxp .xml  yes   yes",
		"vtt   .vtt              yes   yes",
	}, "\n") + "\n"
	assert.Equal(t, result{Stdout: want}, call{}.run(t, "formats"))
}

func TestFormatsListsTheRegistry(t *testing.T) {
	t.Parallel()

	word := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	rows := [][]string{{"Name", "Extensions", "Read", "Write"}}
	for _, f := range stbox.Formats() {
		if !f.CanRead() && !f.CanWrite() {
			continue
		}
		var exts []string
		for _, e := range f.Extensions() {
			exts = append(exts, "."+e)
		}
		rows = append(rows, []string{string(f), strings.Join(exts, " "), word(f.CanRead()), word(f.CanWrite())})
	}
	got := call{}.run(t, "formats")
	assert.Equal(t, result{Stdout: table(rows, 0, 2)}, got)
	assert.Regexp(t, `\AName +Extensions +Read  Write\n`, got.Stdout)
	assert.Regexp(t, `(?m)^srt +\.srt +yes   yes$`, got.Stdout)
	assert.NotContains(t, got.Stdout, "\ntxt ")
}

func TestFormatsUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"argument", []string{"formats", "x"}, "The formats command takes no arguments."},
		{"argument after dashes", []string{"formats", "--", "-"}, "The formats command takes no arguments."},
		{"option", []string{"formats", "--json"}, "Unknown option --json."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, usageResult("formats", tt.want), call{}.run(t, tt.words...))
		})
	}
}
