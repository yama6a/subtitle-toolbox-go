package vobsub_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// box returns rows of the pixel value v.
func box(width, height int, v byte) [][]byte {
	rows := make([][]byte, height)
	for i := range rows {
		rows[i] = make([]byte, width)
		for j := range rows[i] {
			rows[i][j] = v
		}
	}
	return rows
}

// shown is a picture of 8x4 pixels at 10, 20 that starts after the delay of its first sequence.
func shown(sequences ...sequence) picture {
	return picture{rows: box(8, 4, 1), x: 10, y: 20, colors: [4]int{0, 3, 5, 9}, contrasts: [4]int{0, 15, 8, 4}, sequences: sequences}
}

func start0() sequence { return sequence{params: true, commands: []byte{start}} }

func TestReadBuiltUnits(t *testing.T) {
	t.Parallel()

	box8 := func(start, end time.Duration, forced bool) cue { return cue{start, end, forced, 10, 20, 8, 4} }
	tests := []struct {
		name    string
		packets []packet
		times   []time.Duration
		want    []cue
	}{
		{
			"start delay of 114 ms and stop delay of 1138 ms",
			[]packet{{unit: shown(sequence{delay: 10, params: true, commands: []byte{start}}, sequence{delay: 100, commands: []byte{stop}}).unit()}},
			[]time.Duration{time.Second},
			[]cue{box8(1114*ms, 2138*ms, false)},
		},
		{
			"the forced start command sets forced",
			[]packet{{unit: shown(sequence{params: true, commands: []byte{forcedStart}}).unit()}},
			[]time.Duration{time.Second},
			[]cue{box8(time.Second, 6*time.Second, true)},
		},
		{
			"a unit without a stop ends at the next unit or after the last cue duration",
			[]packet{{unit: shown(start0()).unit()}, {unit: shown(start0()).unit()}},
			[]time.Duration{time.Second, 3 * time.Second},
			[]cue{box8(time.Second, 3*time.Second, false), box8(3*time.Second, 8*time.Second, false)},
		},
		{
			"a unit without a picture ends the cue before it and adds no cue",
			[]packet{{unit: shown(start0()).unit()}, {unit: shown(sequence{commands: []byte{start}}).unit()}},
			[]time.Duration{time.Second, 2 * time.Second},
			[]cue{box8(time.Second, 2*time.Second, false)},
		},
		{
			"CHG_COLCON is skipped by its size",
			[]packet{{unit: shown(sequence{delay: 10, params: true, commands: []byte{colorChange, 0, 4, 0xAA, 0xBB, start}}).unit()}},
			[]time.Duration{time.Second},
			[]cue{box8(1114*ms, 6114*ms, false)},
		},
		{
			"a unit over 4 packets with packets of another track between",
			[]packet{{unit: shown(start0()).unit(), chunk: 7, other: true}, {unit: shown(start0()).unit()}},
			[]time.Duration{time.Second, 3 * time.Second},
			[]cue{box8(time.Second, 3*time.Second, false), box8(3*time.Second, 8*time.Second, false)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sub, positions := program(tt.packets...)
			s := mustRead(t, sub, idxFor(0, tt.times, positions))
			assert.Equal(t, tt.want, cuesOf(s))
		})
	}
}

func TestReadPaintsThePixels(t *testing.T) {
	t.Parallel()

	long := make([]byte, 300)
	for i := range long {
		switch {
		case i < 2:
			long[i] = 1
		case i < 12:
			long[i] = 2
		case i < 52:
			long[i] = 3
		default:
			long[i] = 1
		}
	}
	custom := "custom colors: ON, tridx: 1000, colors: 000000, ff0000, 00ff00, 0000ff\n"
	rows := [][]byte{{1, 1, 2}, {3, 3, 0}, {2, 2, 1}}
	tests := []struct {
		name string
		head string
		pic  picture
		want map[[2]int]uint32
	}{
		{
			"palette colours with the contrast as alpha, on an odd height",
			"palette: " + palette + "\n",
			picture{rows: rows, colors: [4]int{0, 3, 5, 9}, contrasts: [4]int{0, 15, 8, 4}},
			map[[2]int]uint32{{0, 0}: 0x300000FF, {2, 0}: 0x50000088, {0, 1}: 0x90000044, {2, 1}: 0, {1, 2}: 0x50000088, {2, 2}: 0x300000FF},
		},
		{
			"custom colours replace the palette and the contrast",
			custom,
			picture{rows: rows, colors: [4]int{0, 3, 5, 9}, contrasts: [4]int{0, 15, 8, 4}},
			map[[2]int]uint32{{0, 0}: 0xFF0000FF, {2, 0}: 0x00FF00FF, {0, 1}: 0x0000FFFF, {2, 1}: 0},
		},
		{
			"the bottom field first",
			"palette: " + palette + "\n",
			picture{rows: rows, colors: [4]int{0, 3, 5, 9}, contrasts: [4]int{0, 15, 8, 4}, bottomFirst: true},
			map[[2]int]uint32{{0, 0}: 0x300000FF, {0, 1}: 0x90000044, {1, 2}: 0x50000088},
		},
		{
			"codes of 1 to 4 nibbles and a last run to the end of the row",
			"palette: " + palette + "\n",
			picture{rows: [][]byte{long, long}, colors: [4]int{0, 3, 5, 9}, contrasts: [4]int{15, 15, 15, 15}},
			map[[2]int]uint32{{1, 0}: 0x300000FF, {2, 0}: 0x500000FF, {11, 0}: 0x500000FF, {12, 0}: 0x900000FF, {51, 1}: 0x900000FF, {52, 1}: 0x300000FF, {299, 1}: 0x300000FF},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.pic.sequences = []sequence{start0()}
			sub, positions := program(packet{unit: tt.pic.unit()})
			idx := index(tt.head, "id: en, index: 0", "timestamp: 00:00:01:000, filepos: "+hex9(positions[0]))
			c, ok := mustRead(t, sub, idx).Cue(0)
			require.True(t, ok)
			for at, want := range tt.want {
				assert.Equal(t, want, pixel(t, c, at[0], at[1]), "pixel %v", at)
			}
		})
	}
}

func TestReadSelectsTheTrack(t *testing.T) {
	t.Parallel()

	packets := make([]packet, 0, 4)
	for track := range 4 {
		pic := shown(start0())
		pic.x = 10 * track
		packets = append(packets, packet{unit: pic.unit(), track: track})
	}
	sub, pos := program(packets...)
	ts := func(sec int, track int) string {
		return "timestamp: 00:00:0" + strconv.Itoa(sec) + ":000, filepos: " + hex9(pos[track])
	}
	idx := index("palette: "+palette+"\n",
		"id: en, index: 0", ts(1, 0),
		"id: de, index: 1", ts(2, 1),
		"id: DE, index: 2", ts(3, 2),
		"id: , index: 3", ts(4, 3))
	tests := []struct {
		name     string
		opts     vobsub.ReadOptions
		language string
		x        int
		start    time.Duration
		wantErr  string
	}{
		{name: "no filter reads the first track", language: "en", x: 0, start: time.Second},
		{name: "index 1", opts: vobsub.ReadOptions{Index: new(1)}, language: "de", x: 10, start: 2 * time.Second},
		{name: "index 0 is a filter", opts: vobsub.ReadOptions{Index: new(0)}, language: "en", x: 0, start: time.Second},
		{name: "language without case, first match", opts: vobsub.ReadOptions{Language: "dE"}, language: "de", x: 10, start: 2 * time.Second},
		{name: "index and language", opts: vobsub.ReadOptions{Index: new(2), Language: "de"}, language: "DE", x: 20, start: 3 * time.Second},
		{name: "an empty id is a language", opts: vobsub.ReadOptions{Index: new(3)}, language: "", x: 30, start: 4 * time.Second},
		{name: "no track with the index", opts: vobsub.ReadOptions{Index: new(9)}, wantErr: "vobsub: The .idx content has no track with index 9."},
		{name: "no track with the language", opts: vobsub.ReadOptions{Language: "fr"}, wantErr: `vobsub: The .idx content has no track with language "fr".`},
		{
			name: "no track with both", opts: vobsub.ReadOptions{Index: new(0), Language: "de"},
			wantErr: `vobsub: The .idx content has no track with index 0 and language "de".`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(t, sub, idx, tt.opts)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			lang, ok := s.Metadata("language")
			assert.True(t, ok)
			assert.Equal(t, tt.language, lang)
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, [2]any{tt.start, tt.x}, [2]any{c.Start, c.Image.X})
		})
	}
}

func TestReadOptionsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts vobsub.ReadOptions
		want model.InvalidArgumentError
	}{
		{"no IDX", vobsub.ReadOptions{}, model.InvalidArgumentError{Option: "ReadOptions.VobSub.IDX", Msg: "VobSub needs the .idx content. Set ReadOptions.VobSub.IDX."}},
		{
			"negative index",
			vobsub.ReadOptions{IDX: "x", Index: new(-1)},
			model.InvalidArgumentError{Option: "ReadOptions.VobSub.Index", Msg: "The track index must be 0 or more, got -1."},
		},
		{
			"blank language",
			vobsub.ReadOptions{IDX: "x", Language: " \t"},
			model.InvalidArgumentError{Option: "ReadOptions.VobSub.Language", Msg: "The language must not be empty."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := vobsub.Read(strings.NewReader(""), model.CommonReadOptions{}, tt.opts)
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			assert.Equal(t, tt.want, *aerr)
		})
	}
}

func TestReadReportsBrokenUnits(t *testing.T) {
	t.Parallel()

	good := shown(start0()).unit()
	goodSub, _ := program(packet{unit: good})
	cutSub, _ := program(packet{unit: good, chunk: 7})
	zeros := make([]byte, 64)
	area := []byte{0x05, 0x00, 0x00, 0x07, 0x00, 0x00, 0x01}
	tests := []struct {
		name string
		sub  []byte
		pos  int
		want string
	}{
		{
			"filepos past the content", goodSub, len(goodSub) + 0x100,
			"The filepos 000000" + strconv.FormatInt(int64(len(goodSub)+0x100), 16) + " of timestamp 1.000 is outside the .sub content of " + strconv.Itoa(len(goodSub)) + " bytes.",
		},
		{
			"no start code", zeros, 0,
			"The .sub content has no complete subtitle packet at filepos 000000000. It ends or has no MPEG start code at byte 0.",
		},
		{
			"unit cut off at the end of the content", cutSub[:31], 0,
			"The .sub content has no complete subtitle packet at filepos 000000000. It ends or has no MPEG start code at byte 31.",
		},
		{"pack header without the MPEG-2 marker", slicesConcat([]byte("\x00\x00\x01\xBA"), zeros), 0, "The .sub content has no MPEG-2 pack header at byte 0."},
		{
			"program end before the unit is complete", slicesConcat([]byte("\x00\x00\x01\xB9"), zeros), 0,
			"The .sub content ends at byte 0 before the subtitle packet at filepos 000000000 is complete.",
		},
		{"packet longer than the content", slicesConcat([]byte(packHeader), []byte("\x00\x00\x01\xBD\xFF\xFF"), zeros), 0, "The .sub packet at byte 14 is longer than the content."},
		{
			"control sequence offset below 4", pesOf(slicesConcat(u16(10, 2), make([]byte, 6))), 0,
			"The subtitle packet of 10 bytes has no valid control sequence offset.",
		},
		{"command past the end of the unit", pesOf(cutCommand), 0, "The subtitle packet ends inside command 03."},
		{
			"bitmap past the end of the unit", pesOf(rawUnit(slicesConcat(area, []byte{0x06, 0xFF, 0xFF, 0xFF, 0xFF, start})...)), 0,
			"The bitmap of the subtitle packet ends before line 0 is complete.",
		},
		{
			"image over the size limit", pesOf(rawUnit(0x05, 0x00, 0x0F, 0xFF, 0x00, 0x0F, 0xFF, 0x06, 0, 0, 0, 0, start)), 0,
			"Cannot read the subtitle packet: an image of 4096x4096 pixels is larger than the limit of 7680 pixels per side and 8294400 pixels in total.",
		},
	}
	for _, tt := range tests {
		idx := index("palette: "+palette+"\n", "id: en, index: 0", "timestamp: 00:00:01:000, filepos: "+hex9(tt.pos))
		t.Run(tt.name+" strict", func(t *testing.T) {
			t.Parallel()

			_, _, err := read(t, tt.sub, idx, vobsub.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{Format: model.VobSub, Msg: tt.want}, *perr)
		})
		t.Run(tt.name+" lenient", func(t *testing.T) {
			t.Parallel()

			s, warnings, err := vobsub.Read(bytes.NewReader(tt.sub), model.CommonReadOptions{Lenient: true}, vobsub.ReadOptions{IDX: string(idx)})
			require.NoError(t, err)
			assert.Zero(t, s.Len())
			assert.Equal(t, []model.ParseWarning{{Message: tt.want, Line: model.NoLine, Block: 0, Action: model.Skipped}}, warnings)
		})
	}
}

// cutCommand is a unit whose SET_COLOR command has 1 of its 2 argument bytes.
var cutCommand = slicesConcat(u16(10, 4), u16(0, 0), []byte{0x03, 0x01})

// pesOf returns the .sub content of a unit in one packet.
func pesOf(unit []byte) []byte {
	sub, _ := program(packet{unit: unit})
	return sub
}

func TestLenientReadSkipsABrokenUnitAndReadsTheNext(t *testing.T) {
	t.Parallel()

	good := shown(start0()).unit()
	broken := cutCommand
	tests := []struct {
		name    string
		packets []packet
		want    []cue
		block   int
	}{
		{"broken first", []packet{{unit: broken}, {unit: good}}, []cue{{2 * time.Second, 7 * time.Second, false, 10, 20, 8, 4}}, 0},
		{"broken last", []packet{{unit: good}, {unit: broken}}, []cue{{time.Second, 6 * time.Second, false, 10, 20, 8, 4}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sub, positions := program(tt.packets...)
			idx := idxFor(0, []time.Duration{time.Second, 2 * time.Second}, positions)
			s, warnings, err := vobsub.Read(bytes.NewReader(sub), model.CommonReadOptions{Lenient: true}, vobsub.ReadOptions{IDX: string(idx)})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cuesOf(s))
			require.Len(t, warnings, 1)
			assert.Equal(t, tt.block, warnings[0].Block)
		})
	}
}
