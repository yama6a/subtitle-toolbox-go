package mcc_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	header2 = "File Format=MacCaption_MCC V2.0\nTime Code Rate=30DF\n\n"
	// edm is the CEA-608 command EDM, erase displayed memory, with parity.
	edm = 0x942C
)

// packet returns the data of an MCC line at 29.97 fps with one field 1 pair, as the writer writes it.
func packet(word uint16) string {
	triplets := make([]cea708.CDPTriplet, 0, 20)
	triplets = append(triplets, cea708.CDPTriplet{Type: cea708.Field1, Valid: true, Data: word}, cea708.CDPTriplet{Type: cea708.Field2, Valid: true, Data: 0x8080})
	for range 18 {
		triplets = append(triplets, cea708.CDPTriplet{Type: cea708.DTVCCData})
	}
	return mcc.EncodeHex(mcc.ANCPacket(cea708.WriteCDP(4, 0, triplets)))
}

func readFile(t *testing.T, name string, common model.CommonReadOptions, opts mcc.ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return mcc.Read(strings.NewReader(string(testfiles.Read(t, "mcc/real/"+name))), common, opts)
}

type cueWant struct {
	start, end time.Duration
	lines      []string
	alignment  model.Alignment
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func summarize(cues []model.Cue) []cueWant {
	out := make([]cueWant, len(cues))
	for i, c := range cues {
		out[i] = cueWant{c.Start, c.End, c.Lines, c.Position.Alignment}
	}
	return out
}

func TestReadOptions(t *testing.T) {
	t.Parallel()

	cea608 := []cueWant{
		{ms(2302), ms(4671), []string{"THE WINTER TIMETABLE", "STARTS ON MONDAY."}, model.BottomCenter},
		{ms(5472), ms(7007), []string{"<i>[ANNOUNCER]</i>"}, model.BottomCenter},
		{ms(8475), ms(11011), []string{"THE COAST TRAIN LEAVES AT", "6:50, TEN MINUTES EARLIER."}, model.BottomCenter},
	}
	// RDC, PAC row 15, "AB", a null pair, then "CD": the null pair splits the paint-on data into 2 states.
	paintOn := header2
	for frame, word := range []uint16{0x9429, 0x9429, 0x94E0, 0x94E0, 0xC1C2, 0x8080, 0x43C4} {
		paintOn += fmt.Sprintf("00:00:01:%02d\t%s\n", frame, packet(word))
	}
	paintOn += "00:00:03:00\t" + packet(edm) + "\n"

	tests := []struct {
		name    string
		file    string
		content string
		opts    mcc.ReadOptions
		want    []cueWant
	}{
		{name: "service 1 by default", file: "captionmaker_608_708.mcc", want: []cueWant{
			{ms(2302), ms(5472), []string{"L'horaire d'hiver", "débute lundi…"}, model.BottomCenter},
			{ms(5472), ms(7007), []string{"<i>♪ Annonce ♪</i>"}, model.TopLeft},
			{ms(8475), ms(11011), []string{`<font color="#ffff00">Le train côtier part</font>`, "à 6 h 50."}, model.BottomCenter},
		}},
		{name: "service 2", file: "captionmaker_608_708.mcc", opts: mcc.ReadOptions{Service: 2}, want: []cueWant{
			{ms(2302), ms(7007), []string{"El horario de invierno", "empieza el lunes."}, model.BottomCenter},
		}},
		{name: "service 3 has no captions, so CEA-608", file: "captionmaker_608_708.mcc", opts: mcc.ReadOptions{Service: 3}, want: cea608},
		{name: "CEA-608 only", file: "captionmaker_608_708.mcc", opts: mcc.ReadOptions{CEA608Only: true}, want: cea608},
		{name: "a null pair ends a line of paint-on data", content: paintOn, opts: mcc.ReadOptions{CEA608Only: true}, want: []cueWant{
			{ms(1134), ms(1201), []string{"AB"}, model.BottomCenter},
			{ms(1201), ms(3003), []string{"ABCD"}, model.BottomCenter},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s *model.Subtitle
			var err error
			if tt.file != "" {
				s, _, err = readFile(t, tt.file, model.CommonReadOptions{}, tt.opts)
			} else {
				s, _, err = mcc.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, tt.opts)
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, summarize(s.Cues()))
		})
	}
}

func TestReadsTheCuesOfTheSCCFileWithTheSameBytePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, mcc, scc string
		rollUp         mcc.RollUp
	}{
		{"CaptionMaker shape at 29.97 fps", "captionmaker_popon_df.mcc", "captionmaker_popon_df.scc", mcc.RollUpScreen},
		{"59.94 fps with field 2 lines", "v2_5994df_fields.mcc", "rollup_news_ndf.scc", mcc.RollUpScreen},
		{"59.94 fps with roll-up rows", "v2_5994df_fields.mcc", "rollup_news_ndf.scc", mcc.RollUpLines},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want, _, err := scc.Read(strings.NewReader(string(testfiles.Read(t, "scc/real/"+tt.scc))), model.CommonReadOptions{}, scc.ReadOptions{RollUp: tt.rollUp})
			require.NoError(t, err)
			got, _, err := readFile(t, tt.mcc, model.CommonReadOptions{}, mcc.ReadOptions{RollUp: tt.rollUp})
			require.NoError(t, err)
			assert.Equal(t, want.Cues(), got.Cues())
		})
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	data := func(hex string) string { return "00:00:00:00\t" + hex }
	tests := []struct {
		name    string
		content string
		// msg is the strict error. line is its line, 0 for none.
		msg  string
		line int
		// repaired says that the lenient read repairs the file with a warning in place of skipping a line.
		repaired bool
		// clean says that both modes read the file with no warning.
		clean bool
	}{
		{
			name: "no header", content: "\nScenarist_SCC V1.0\n", line: 2,
			msg: `An MCC file must start with the line "File Format=MacCaption_MCC V2.0" or "File Format=MacCaption_MCC V1.0".`,
		},
		{
			name: "unknown version", content: "File Format=MacCaption_MCC V3.0\n", line: 1,
			msg: "The MCC version 3.0 is unknown. The parser reads 1.0 and 2.0.",
		},
		{name: "no time code", content: header2 + "captions\n", line: 4, msg: `The MCC line "captions" does not start with a time code.`},
		{name: "invalid letter", content: header2 + data("T49SV") + "\n", line: 4, msg: `The MCC data has the invalid character "V" at position 5.`},
		{name: "half a byte", content: header2 + data("T4") + "\n", line: 4, msg: "The MCC data ends with half a byte."},
		{name: "no CDP identifier", content: header2 + data("T0A9670ZZZZZZZZZZZZZZ") + "\n", line: 4, msg: "The data holds no caption distribution packet."},
		{
			name: "short cc_data", content: header2 + data("T0BS0B4F43ZZ72F4FC") + "\n", line: 4,
			msg: "The caption distribution packet is shorter than its 20 cc_data triplets.",
		},
		{
			name: "no Time Code Rate", content: "File Format=MacCaption_MCC V2.0\n\n" + data(packet(edm)) + "\n", repaired: true,
			msg: "The MCC file has no Time Code Rate line.",
		},
		{
			name: "unknown Time Code Rate", content: "File Format=MacCaption_MCC V2.0\nTime Code Rate=25DF\n", repaired: true,
			msg: `The MCC Time Code Rate "25DF" is unknown.`,
		},
		{
			name: "time above 100000 hours", content: header2 + "100001:00:00:00\t" + packet(edm) + "\n", line: 4,
			msg: `The time "100001:00:00:00" is not below 100000 hours.`,
		},
		{
			name: "time of 999998 hours", content: header2 + "999998:00:00:00\t" + packet(edm) + "\n", line: 4,
			msg: `The time "999998:00:00:00" is not below 100000 hours.`,
		},
		{name: "time below the limit with 5 hour digits", content: header2 + "99999:59:59:00\t" + packet(edm) + "\n", clean: true},
		{name: "AFD packet", content: header2 + data("4105080800000000000000005E") + "\n", clean: true},
		{name: "CDP without sections", content: header2 + data("T07S074F430000") + "\n", clean: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := mcc.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, mcc.ReadOptions{})
			if tt.clean {
				require.NoError(t, err)
			} else {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.msg, perr.Msg)
				assert.Equal(t, tt.line, perr.Line)
			}

			content := tt.content
			if tt.line == 4 {
				content += "00:00:01:00\t" + packet(edm) + "\n"
			}
			s, warnings, err := mcc.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, mcc.ReadOptions{})
			if !tt.clean && !tt.repaired && tt.line != 4 {
				require.Error(t, err, "a header error also fails in lenient mode")
				return
			}
			require.NoError(t, err)
			switch {
			case tt.clean:
				assert.Empty(t, warnings)
			case tt.repaired:
				require.Len(t, warnings, 1)
				assert.Equal(t, tt.msg+" The parser read 30DF.", warnings[0].Message)
				assert.Equal(t, model.Repaired, warnings[0].Action)
				file, _ := s.FormatData("mcc")
				dropFrame, _ := file.Bool("dropFrame")
				assert.True(t, dropFrame)
			default:
				require.Len(t, warnings, 1)
				assert.Equal(t, tt.msg, warnings[0].Message)
				assert.Equal(t, 4, warnings[0].Line)
				assert.Equal(t, model.Skipped, warnings[0].Action)
			}
		})
	}
}

func TestReadWarningsKeepFileOrder(t *testing.T) {
	t.Parallel()

	content := header2 + "100001:00:00:00\t" + packet(edm) + "\n" + "00:00:00:00\tT4\n" + "100002:00:00:00\t" + packet(edm) + "\n"
	_, warnings, err := mcc.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, mcc.ReadOptions{})
	require.NoError(t, err)
	lines := make([]int, len(warnings))
	for i, w := range warnings {
		lines[i] = w.Line
	}
	assert.Equal(t, []int{4, 5, 6}, lines)
}
