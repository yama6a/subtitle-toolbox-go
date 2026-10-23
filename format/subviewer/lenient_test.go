package subviewer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const timingForm = "is not in the form hh:mm:ss.ff,hh:mm:ss.ff."

// Ports testSubViewerKeepsTheTextOfTheSkippedCueInTheWarning of tests/Parsers/LenientSubViewerTest.php of
// subtitle-toolbox-php.
func TestLenientSkippedCueKeepsItsTextInTheWarning(t *testing.T) {
	t.Parallel()

	_, warnings := mustRead(t, string(testfiles.Read(t, "lenient/bad_time_line.sub")), true)
	require.Len(t, warnings, 2)
	assert.Equal(t, []string{"00:00:04.00,00:00:0x.00", "Apples are cheap today."}, warnings[1].Lines)
	assert.Equal(t, []string{"Downloaded from a subtitle site"}, warnings[0].Lines)
}

// Ports testSubViewerReadsALooseTimingLineOnlyInLenientMode of tests/Parsers/LenientSubViewerTest.php of
// subtitle-toolbox-php.
func TestLenientReadsALooseTimingLineOnlyInLenientMode(t *testing.T) {
	t.Parallel()

	for _, line := range []string{"00:00:01.5000,00:00:03.0000", "00:00:01.5,00:00:03", "0:0:1.50,0:0:3.00"} {
		t.Run(line, func(t *testing.T) {
			t.Parallel()

			content := "00:00:00.00,00:00:01.00\nFirst\n\n" + line + "\nHello\n"
			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{
				{0, time.Second, []string{"First"}},
				{1500 * ms, 3 * time.Second, []string{"Hello"}},
			}, cueRows(s))
			assert.Equal(t, []warningRow{{
				`The timing line "` + line + `" ` + timingForm + " The parser read it as 1.5 s to 3 s.", 4, 1, []string{line}, model.Repaired,
			}}, warningRows(warnings))
			assert.Contains(t, write(t, s, subviewer.WriteOptions{}), "00:00:01.50,00:00:03.00\nHello")

			err := readError(t, content)
			assert.Equal(t, 4, err.Line)
			assert.Equal(t, `The timing line "`+line+`" `+timingForm, err.Msg)
		})
	}
}

// Ports testSubViewer1SkipsABrokenHeaderLine of tests/Parsers/LenientSubViewerTest.php of subtitle-toolbox-php.
func TestLenientVersion1SkipsABrokenHeaderLine(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "[TITLE]\nMarket\nbroken\n"+script+"[00:00:01]\nHello\n[00:00:02]\n", true)
	assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, []string{"Hello"}}}, cueRows(s))
	assert.Equal(t, []warningRow{{`The line "broken" is not a SubViewer 1 header tag.`, 3, 0, []string{"broken"}, model.Skipped}}, warningRows(warnings))
	assert.Equal(t, map[string]string{"title": "Market"}, s.AllMetadata())
}

func TestLenientVersion2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantCues     []cueRow
		wantWarnings []warningRow
	}{
		{
			"a skipped cue holds its timing line and its text lines",
			"00:00:01.00,00:00:02.00\nOne\n\n00:00:03.00,00:75:04.00\nTwo\nThree\n\n00:00:05.00,00:00:06.00\nFour\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}, {5 * time.Second, 6 * time.Second, []string{"Four"}}},
			[]warningRow{{
				`The timing line "00:00:03.00,00:75:04.00" has a time that is not valid.`, 4, 1,
				[]string{"00:00:03.00,00:75:04.00", "Two", "Three"},
				model.Skipped,
			}},
		},
		{
			"a skipped cue at the file end",
			"00:00:01.00,00:00:02.00\nOne\n\n00:00:0x.00,00:00:04.00\nTwo\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}},
			[]warningRow{{
				`The timing line "00:00:0x.00,00:00:04.00" has a time that is not valid.`, 4, 1, nil, model.Skipped,
			}},
		},
		{
			"the end before the start swaps up to 30 seconds",
			"00:00:05.00,00:00:03.00\nOne\n",
			[]cueRow{{3 * time.Second, 5 * time.Second, []string{"One"}}},
			[]warningRow{{
				"The cue ends at 3 s, before it starts at 5 s. The parser swapped the times.", 1, 0, []string{"00:00:05.00,00:00:03.00"}, model.Repaired,
			}},
		},
		{
			"the end more than 30 seconds before the start skips the cue",
			"00:01:05.00,00:00:03.00\nOne\n\n00:02:00.00,00:03:00.00\nTwo\n",
			[]cueRow{{2 * time.Minute, 3 * time.Minute, []string{"Two"}}},
			[]warningRow{{
				"The cue ends at 3 s, before it starts at 65 s.", 1, 0, []string{"00:01:05.00,00:00:03.00", "One"}, model.Skipped,
			}},
		},
		{
			"a loose time that rounds to the limit skips the cue",
			"99999:59:59.9996,99999:59:59.9999\nOne\n\n00:00:01.00,00:00:02.00\nTwo\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"Two"}}},
			[]warningRow{{
				`The timing line "99999:59:59.9996,99999:59:59.9999" has a time that is not valid.`, 1, 0,
				[]string{"99999:59:59.9996,99999:59:59.9999", "One"},
				model.Skipped,
			}},
		},
		{
			"a loose time that rounds below the limit",
			"99999:59:59,99999:59:59.9994\nOne\n",
			[]cueRow{{99999*time.Hour + 59*time.Minute + 59*time.Second, 99999*time.Hour + 59*time.Minute + 59*time.Second + 999*ms, []string{"One"}}},
			[]warningRow{{
				`The timing line "99999:59:59,99999:59:59.9994" ` + timingForm + " The parser read it as 359999999 s to 359999999.999 s.", 1, 0,
				[]string{"99999:59:59,99999:59:59.9994"},
				model.Repaired,
			}},
		},
		{
			"a text line before the first timing line is skipped",
			"Downloaded from a site\n00:00:01.00,00:00:02.00\nOne\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}},
			[]warningRow{{`The line "Downloaded from a site" is neither a header tag nor a timing line.`, 1, 0, []string{"Downloaded from a site"}, model.Skipped}},
		},
		{
			"fields with spaces are no timing line",
			"00:00:01.00 , 00:00:02.00\nOne\n",
			nil,
			[]warningRow{
				{`The line "00:00:01.00 , 00:00:02.00" is neither a header tag nor a timing line.`, 1, 0, []string{"00:00:01.00 , 00:00:02.00"}, model.Skipped},
				{`The line "One" is neither a header tag nor a timing line.`, 2, 0, []string{"One"}, model.Skipped},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.wantCues, cueRows(s))
			if tt.wantWarnings == nil {
				assert.Empty(t, warnings)
				return
			}
			for i := range tt.wantWarnings {
				if tt.wantWarnings[i].Lines == nil {
					tt.wantWarnings[i].Lines = []string{"00:00:0x.00,00:00:04.00", "Two"}
				}
			}
			assert.Equal(t, tt.wantWarnings, warningRows(warnings))
		})
	}
}

func TestLenientVersion1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantCues     []cueRow
		wantWarnings []warningRow
	}{
		{
			"a swapped cue",
			script + "[00:00:05]\nOne\n[00:00:03]\n",
			[]cueRow{{3 * time.Second, 5 * time.Second, []string{"One"}}},
			[]warningRow{{"The cue ends at 3 s, before it starts at 5 s. The parser swapped the times.", 2, 0, []string{"[00:00:05]"}, model.Repaired}},
		},
		{
			"a cue that ends more than 30 seconds before it starts",
			script + "[00:01:05]\nOne\n[00:00:03]\n\n[00:02:00]\nTwo\n[00:02:03]\n",
			[]cueRow{{2 * time.Minute, 123 * time.Second, []string{"Two"}}},
			[]warningRow{{"The cue ends at 3 s, before it starts at 65 s.", 2, 0, []string{"[00:01:05]"}, model.Skipped}},
		},
		{
			"a bad time line leaves the text unread",
			script + "[00:00:01]\nOne\n[00:00:02]\n\n[00:00:75]\nTwo\n[00:00:06]\n",
			[]cueRow{{time.Second, 2 * time.Second, []string{"One"}}},
			[]warningRow{{`The time "[00:00:75]" is not valid.`, 6, 1, []string{"[00:00:75]"}, model.Skipped}},
		},
		{
			"a delay that moves a time past the limit",
			"[DELAY]359999995\n" + script + "[00:00:01]\nOne\n[00:00:02]\n\n[00:00:09]\nTwo\n[00:00:10]\n",
			[]cueRow{{359999996 * time.Second, 359999997 * time.Second, []string{"One"}}},
			[]warningRow{
				{`The time "[00:00:09] with DELAY 359999995" is not below 100000 hours.`, 7, 1, []string{"[00:00:09]"}, model.Skipped},
				{`The time "[00:00:10] with DELAY 359999995" is not below 100000 hours.`, 9, 1, []string{"[00:00:10]"}, model.Skipped},
			},
		},
		{
			"a negative delay beyond the limit",
			"[DELAY]-999999999\n" + script + "[00:00:01]\nOne\n[00:00:02]\n",
			nil,
			[]warningRow{
				{`The time "[00:00:01] with DELAY -999999999" is not below 100000 hours.`, 3, 0, []string{"[00:00:01]"}, model.Skipped},
				{`The time "[00:00:02] with DELAY -999999999" is not below 100000 hours.`, 5, 0, []string{"[00:00:02]"}, model.Skipped},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.wantCues, cueRows(s))
			assert.Equal(t, tt.wantWarnings, warningRows(warnings))
		})
	}
}
