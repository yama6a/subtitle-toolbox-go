package mcc_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func readSRT(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, name)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts mcc.WriteOptions) (string, mcc.WriteReport) {
	t.Helper()

	var out bytes.Buffer
	report, err := mcc.Write(&out, s, common, opts)
	require.NoError(t, err)
	return out.String(), report
}

func read(t *testing.T, content string, opts mcc.ReadOptions) []model.Cue {
	t.Helper()

	s, _, err := mcc.Read(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s.Cues()
}

var dataLineRegexp = regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d:\d\d\t(\S+)$`)

func TestWriteRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rate      model.FrameRate
		dropFrame model.DropFrameMode
		label     string
		code      int
		triplets  int
		fps       float64
	}{
		{"23.976", model.FrameRate23976, model.DropFrameKeep, "24", 1, 25, 23.976},
		{"24", model.FrameRate24, model.DropFrameKeep, "24", 2, 25, 24},
		{"25", model.FrameRate25, model.DropFrameKeep, "25", 3, 24, 25},
		{"29.97 drop-frame", model.FrameRate2997, model.DropFrameKeep, "30DF", 4, 20, 29.97},
		{"29.97 non-drop", model.FrameRate2997, model.DropFrameOff, "30", 4, 20, 29.97},
		{"30", model.FrameRate30, model.DropFrameKeep, "30", 5, 20, 30},
		{"50", model.FrameRate50, model.DropFrameKeep, "50", 6, 12, 50},
		{"59.94 drop-frame", model.FrameRate5994, model.DropFrameKeep, "60DF", 7, 10, 59.94},
		{"60", model.FrameRate60, model.DropFrameKeep, "60", 8, 10, 60},
	}
	input := readSRT(t, "srt/real/own_styled.srt")
	sccOut := new(bytes.Buffer)
	_, err := scc.Write(sccOut, input, model.CommonWriteOptions{}, scc.WriteOptions{Fit: true})
	require.NoError(t, err)
	sccSub, _, err := scc.Read(sccOut, model.CommonReadOptions{}, scc.ReadOptions{})
	require.NoError(t, err)
	want608 := sccSub.Cues()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := write(t, input, model.CommonWriteOptions{}, mcc.WriteOptions{FrameRate: tt.rate, DropFrame: tt.dropFrame})
			assert.Contains(t, out, "\nTime Code Rate="+tt.label+"\n")

			lines := dataLineRegexp.FindAllStringSubmatch(out, -1)
			require.NotEmpty(t, lines)
			for i, m := range lines {
				anc, msg := mcc.DecodeHex(m[1])
				require.Empty(t, msg)
				assert.Equal(t, byte(sum(anc[:len(anc)-1])), anc[len(anc)-1], "ANC checksum of line %d", i)
				assert.Zero(t, byte(sum(anc[3:len(anc)-1])), "CDP checksum of line %d", i)
				assert.Equal(t, tt.code, int(anc[6]>>4), "rate code of line %d", i)
				assert.Equal(t, tt.triplets, int(anc[11]&0x1F), "triplets of line %d", i)
			}

			cea608 := read(t, out, mcc.ReadOptions{CEA608Only: true})
			require.Len(t, cea608, len(want608))
			for i, c := range cea608 {
				assert.Equal(t, want608[i].Lines, c.Lines)
				assert.InDelta(t, want608[i].Start.Seconds(), c.Start.Seconds(), 0.04)
			}
			cea708 := read(t, out, mcc.ReadOptions{})
			require.Len(t, cea708, input.Len())
			tolerance := time.Duration(float64(time.Second)/tt.fps) + time.Millisecond
			for i, c := range cea708 {
				source, _ := input.Cue(i)
				assert.InDelta(t, source.Start, c.Start, float64(tolerance))
				assert.InDelta(t, source.End, c.End, float64(tolerance))
			}
		})
	}
}

func sum(b []byte) int {
	n := 0
	for _, v := range b {
		n += int(v)
	}
	return n
}

func TestWriteOutput(t *testing.T) {
	t.Parallel()

	oneCue := func() *model.Subtitle {
		s := &model.Subtitle{}
		s.AddCue(model.Cue{Start: 61 * time.Second, End: 62 * time.Second, Lines: []string{"BUS 7"}})
		return s
	}
	stored := func(name string) func(t *testing.T) *model.Subtitle {
		return func(t *testing.T) *model.Subtitle {
			t.Helper()

			s, _, err := mcc.Read(bytes.NewReader(testfiles.Read(t, "mcc/real/"+name)), model.CommonReadOptions{}, mcc.ReadOptions{})
			require.NoError(t, err)
			return s
		}
	}
	tests := []struct {
		name     string
		subtitle func(t *testing.T) *model.Subtitle
		common   model.CommonWriteOptions
		opts     mcc.WriteOptions
		prefix   string
		matches  string
		suffix   string
	}{
		{
			name: "a new file is version 2.0 at 30DF with a version 5 UUID", subtitle: func(*testing.T) *model.Subtitle { return oneCue() },
			prefix: "File Format=MacCaption_MCC V2.0\n",
			matches: `\nUUID=[0-9A-F]{8}-[0-9A-F]{4}-5[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}\nCreation Program=Subtitle Toolbox\n` +
				`Creation Date=\w+day, \w+ \d+, \d{4}\nTime Code Rate=30DF\n\n00:01:00:`,
		},
		{
			name: "the input version, rate and header stay", subtitle: stored("v1_23976_timecode.mcc"),
			prefix: "File Format=MacCaption_MCC V1.0\n",
			matches: `\nUUID=0A1B2C3D-4E5F-4061-8273-8495A6B7C8D9\nCreation Program=Subtitle Toolbox fixture generator\n` +
				`Creation Date=Sunday, October 11, 2026\nCreation Time=17:40:22\nTime Code Rate=24\n\n`,
		},
		{
			name: "59.94 drop-frame forces version 2.0", subtitle: stored("ffmpeg_painton_v1.mcc"),
			opts:   mcc.WriteOptions{FrameRate: model.FrameRate5994, DropFrame: model.DropFrameOn},
			prefix: "File Format=MacCaption_MCC V2.0\n", matches: `\nTime Code Rate=60DF\n`,
		},
		{
			name: "a frame rate with the drop-frame flag counts as on", subtitle: func(*testing.T) *model.Subtitle { return oneCue() },
			opts:   mcc.WriteOptions{FrameRate: model.FrameRate2997DF},
			prefix: "File Format=MacCaption_MCC V2.0\n", matches: `\nTime Code Rate=30DF\n`,
		},
		{
			name: "an empty subtitle writes the header only", subtitle: func(*testing.T) *model.Subtitle { return &model.Subtitle{} },
			suffix: "Time Code Rate=30DF\n\n",
		},
		{
			name: "line ending and BOM", subtitle: func(*testing.T) *model.Subtitle { return &model.Subtitle{} },
			common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd},
			prefix: "\ufeffFile Format=MacCaption_MCC V2.0\r\n", suffix: "Time Code Rate=30DF\r\n\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.subtitle(t)
			out, _ := write(t, s, tt.common, tt.opts)
			assert.True(t, strings.HasPrefix(out, tt.prefix), out[:40])
			assert.True(t, strings.HasSuffix(out, tt.suffix))
			if tt.matches != "" {
				assert.Regexp(t, tt.matches, out)
			}
			again, _ := write(t, s, tt.common, tt.opts)
			assert.Equal(t, out, again)
		})
	}
}

func TestWriteReadsBackTheFormatData(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"captionmaker_608_708.mcc", "captionmaker_popon_df.mcc", "ffmpeg_painton_v1.mcc", "ffmpeg_rollup.mcc", "v1_23976_timecode.mcc", "v2_5994df_fields.mcc"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in, _, err := readFile(t, name, model.CommonReadOptions{}, mcc.ReadOptions{})
			require.NoError(t, err)
			out, _ := write(t, in, model.CommonWriteOptions{}, mcc.WriteOptions{})
			back, _, err := mcc.Read(strings.NewReader(out), model.CommonReadOptions{}, mcc.ReadOptions{})
			require.NoError(t, err)
			want, _ := in.FormatData("mcc")
			got, _ := back.FormatData("mcc")
			assert.Equal(t, want, got)
		})
	}
}

func TestWriteReport(t *testing.T) {
	t.Parallel()

	ferry := readSRT(t, "scc/fit/own_ferry.srt")
	_, report := write(t, ferry, model.CommonWriteOptions{}, mcc.WriteOptions{})
	var sccOut bytes.Buffer
	sccReport, err := scc.Write(&sccOut, ferry, model.CommonWriteOptions{}, scc.WriteOptions{Fit: true})
	require.NoError(t, err)
	assert.Len(t, report.Changes, 7)
	assert.Equal(t, sccReport.Changes, report.Changes)

	cjk := &model.Subtitle{}
	cjk.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{string(testfiles.Read(t, "scc/fit/own_long_cjk.txt"))}})
	_, err = mcc.Write(&bytes.Buffer{}, cjk, model.CommonWriteOptions{}, mcc.WriteOptions{})
	assert.ErrorIs(t, err, model.ErrUnwritable)
}

func TestHeaderTextMatchesTheMCCNotice(t *testing.T) {
	t.Parallel()

	assert.Equal(t, mcc.HeaderText, string(testfiles.Read(t, "mcc/header.txt")))
}

func TestValidate(t *testing.T) {
	t.Parallel()

	rates := "The MCC formatter accepts the frame rates 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60, got "
	tests := []struct {
		name    string
		err     error
		wantErr *model.InvalidArgumentError
	}{
		{"service 63", mcc.ReadOptions{Service: 63}.Validate(), nil},
		{
			"service 64", mcc.ReadOptions{Service: 64}.Validate(),
			&model.InvalidArgumentError{Option: "ReadOptions.MCC.Service", Msg: "The CEA-708 service must be from 1 to 63, got 64."},
		},
		{
			"channel 3", mcc.ReadOptions{Channel: 3}.Validate(),
			&model.InvalidArgumentError{Option: "ReadOptions.MCC.Channel", Msg: "The MCC data channel must be 1 or 2, got 3."},
		},
		{
			"roll-up mode", mcc.ReadOptions{RollUp: "rows"}.Validate(),
			&model.InvalidArgumentError{Option: "ReadOptions.MCC.RollUp", Msg: `The MCC roll-up mode "rows" is not screen or lines.`},
		},
		{"29.97 as 2997/100", mcc.WriteOptions{FrameRate: model.FrameRate{Num: 2997, Den: 100}}.Validate(), nil},
		{
			"48 fps", mcc.WriteOptions{FrameRate: model.FrameRate{Num: 48, Den: 1}}.Validate(),
			&model.InvalidArgumentError{Option: "WriteOptions.MCC.FrameRate", Msg: rates + "48."},
		},
		{
			"drop-frame mode", mcc.WriteOptions{DropFrame: model.DropFrameMode(3)}.Validate(),
			&model.InvalidArgumentError{Option: "WriteOptions.MCC.DropFrame", Msg: "The MCC drop-frame mode 3 is unknown."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantErr == nil {
				assert.NoError(t, tt.err)
				return
			}
			var got *model.InvalidArgumentError
			require.ErrorAs(t, tt.err, &got)
			assert.Equal(t, tt.wantErr, got)
			assert.ErrorIs(t, tt.err, model.ErrInvalidArgument)
		})
	}
}
