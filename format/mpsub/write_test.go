package mpsub_test

import (
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	goNote  = "NOTE=Created with subtitle-toolbox-go (https://github.com/yama6a/subtitle-toolbox-go)\n"
	header  = "TITLE=\nAUTHOR=\nTYPE=VIDEO\nFORMAT=TIME\n" + goNote
	header2 = "TITLE=\nAUTHOR=\nTYPE=VIDEO\nFORMAT=25\n" + goNote
)

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

// Ports testFramesAreRoundedFromAbsoluteTimesSoErrorsDoNotAddUp, testEscapedCoreMarkupTextIsWrittenAsPlainText
// and testOverlappingCuesWriteANegativeWaitThatParsesBack of tests/Formatters/MpSubFormatterTest.php and
// tests/Parsers/MpSubParserTest.php of subtitle-toolbox-php, then adds rows of its own.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fps  float64
		cues []model.Cue
		want string
	}{
		{"no cues", 0, nil, header},
		{
			"the example of the issue", 0,
			[]model.Cue{cue(time.Second, 4*time.Second, "First"), cue(2500*ms, 5*time.Second, "Second")},
			header + "\n1 3\nFirst\n\n-1.5 2.5\nSecond\n",
		},
		{
			"milliseconds", 0,
			[]model.Cue{cue(0, 456*ms, "A"), cue(4766*ms, 41222*ms, "B")},
			header + "\n0 0.456\nA\n\n4.31 36.456\nB\n",
		},
		{"whole seconds have no fraction", 0, []model.Cue{cue(2*time.Second, 5*time.Second, "A")}, header + "\n2 3\nA\n"},
		{
			"the wait counts from the end of the cue before", 0,
			[]model.Cue{cue(time.Second, 2*time.Second, "A"), cue(5*time.Second, 6*time.Second, "B")},
			header + "\n1 1\nA\n\n3 1\nB\n",
		},
		{"the largest time", 0, []model.Cue{cue(0, timecode.MaxValidTime, "A")}, header + "\n0 359999999.999\nA\n"},
		{"a time below 0 is 0", 0, []model.Cue{cue(-2*time.Second, -time.Second, "A")}, header + "\n0 0\nA\n"},
		{
			"a start below 0 and an end above", 0,
			[]model.Cue{cue(-2*time.Second, 3*time.Second, "A")},
			header + "\n0 3\nA\n",
		},
		{
			"several lines", 0,
			[]model.Cue{cue(0, time.Second, "One", "Two", "Three")},
			header + "\n0 1\nOne\nTwo\nThree\n",
		},
		{
			"markup goes and entities decode", 0,
			[]model.Cue{cue(time.Second, 2*time.Second, "<i>I &lt;3 bread &amp; jam</i>", `<font color="#ff0000">Red</font>`)},
			header + "\n1 1\nI <3 bread & jam\nRed\n",
		},
		{
			"a ruby annotation is text", 0,
			[]model.Cue{cue(0, time.Second, "<ruby>Tokyo<rt>to</rt></ruby>")},
			header + "\n0 1\nTokyo (to)\n",
		},
		{
			"empty lines and tags without text go", 0,
			[]model.Cue{cue(0, time.Second, "<b></b>", "A", " ", "<v Anna>")},
			header + "\n0 1\nA\n",
		},
		{
			"a cue without visible text is not written and does not move the previous end", 0,
			[]model.Cue{cue(time.Second, 2*time.Second, "A"), cue(3*time.Second, 4*time.Second, "<i></i>"), cue(5*time.Second, 6*time.Second, "C")},
			header + "\n1 1\nA\n\n3 1\nC\n",
		},
		{"a cue without lines is not written", 0, []model.Cue{cue(0, time.Second)}, header},
		{
			"frames at 25 fps", 25,
			[]model.Cue{cue(time.Second, 3*time.Second, "A"), cue(3*time.Second, 4500*ms, "B")},
			header2 + "\n25 50\nA\n\n0 38\nB\n",
		},
		{
			"frames round half away from zero", 25,
			[]model.Cue{cue(20*ms, 60*ms, "A")},
			header2 + "\n1 1\nA\n",
		},
		{
			"frames round from absolute times so that errors do not add up", 25,
			[]model.Cue{cue(0, 60*ms, "Cue 0"), cue(60*ms, 120*ms, "Cue 1"), cue(120*ms, 180*ms, "Cue 2"), cue(180*ms, 240*ms, "Cue 3")},
			header2 + "\n0 2\nCue 0\n\n0 1\nCue 1\n\n0 2\nCue 2\n\n0 1\nCue 3\n",
		},
		{
			"a negative wait in frames", 25,
			[]model.Cue{cue(time.Second, 4*time.Second, "A"), cue(2500*ms, 5*time.Second, "B")},
			header2 + "\n25 75\nA\n\n-37 62\nB\n",
		},
		{
			"an exact half frame can round down, as on PHP 8.5", 25,
			[]model.Cue{cue(0, 64820*ms, "A")},
			header2 + "\n0 1620\nA\n",
		},
		{"30 fps", 30, []model.Cue{cue(time.Second, 2*time.Second, "A")}, "TITLE=\nAUTHOR=\nTYPE=VIDEO\nFORMAT=30\n" + goNote + "\n30 30\nA\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := write(t, subtitle(tt.cues...), model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{FrameRate: tt.fps})
			assert.Equal(t, tt.want, got)
		})
	}
}

// Ports testHeaderWithoutMetadataAndFormatDataIsUnchanged and testHeaderHoldsMetadataAndFormatData of
// tests/Formatters/MpSubFormatterTest.php of subtitle-toolbox-php. The default NOTE names this project.
func TestWriteHeader(t *testing.T) {
	t.Parallel()

	t.Run("without metadata and format data", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.SetMetadata(model.MetadataLanguage, "en")
		assert.Equal(t, "\xEF\xBB\xBF"+header, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{}))
	})
	t.Run("metadata and format data", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.SetMetadata(model.MetadataTitle, "Yesterday")
		s.SetMetadata(model.MetadataAuthor, "Jane Doe")
		s.SetMetadata(model.MetadataLanguage, "en")
		require.NoError(t, s.SetFormatData("mpsub", model.NewData(
			model.Field{Key: "NOTE", Value: "Draft"}, model.Field{Key: "FILE", Value: "123,abc"},
			model.Field{Key: "FORMAT", Value: "30"}, model.Field{Key: "TYPE", Value: "AUDIO"},
			model.Field{Key: "TITLE", Value: "stored"}, model.Field{Key: "AUTHOR", Value: "stored"},
			model.Field{Key: "SIZE", Value: "5"},
		)))
		want := "\xEF\xBB\xBFTITLE=Yesterday\nAUTHOR=Jane Doe\nTYPE=AUDIO\nFILE=123,abc\nSIZE=5\nFORMAT=TIME\nNOTE=Draft\n"
		assert.Equal(t, want, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{}))
	})
	t.Run("stored values that are empty stay", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		require.NoError(t, s.SetFormatData("mpsub", model.NewData(model.Field{Key: "TYPE", Value: ""}, model.Field{Key: "NOTE", Value: ""})))
		assert.Equal(t, "TITLE=\nAUTHOR=\nTYPE=\nFORMAT=TIME\nNOTE=\n", writeBare(t, s))
	})
}

func TestWriteHeaderAlwaysReadsBack(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, time.Second, "x"))
	s.SetMetadata(model.MetadataTitle, "A\nB")
	s.SetMetadata(model.MetadataAuthor, "C\r\nD\rE")
	require.NoError(t, s.SetFormatData("mpsub", model.NewData(
		model.Field{Key: "NOTE", Value: "n1\nn2"},
		model.Field{Key: "TYPE", Value: "\nVIDEO"},
		model.Field{Key: "FILE", Value: "f\r1"},
		model.Field{Key: "lower", Value: "x"},
		model.Field{Key: "", Value: "x"},
		model.Field{Key: "A1", Value: "x"},
		model.Field{Key: "A=B", Value: "x"},
		model.Field{Key: "A\nB", Value: "x"},
		model.Field{Key: "\u00c4B", Value: "x"},
		model.Field{Key: "OK", Value: "kept"},
	)))
	got := writeBare(t, s)
	assert.Equal(t, "TITLE=A B\nAUTHOR=C  D E\nTYPE= VIDEO\nFILE=f 1\nOK=kept\nFORMAT=TIME\nNOTE=n1 n2\n\n0 1\nx\n", got)

	back, warnings := mustRead(t, got, false)
	assert.Empty(t, warnings)
	assert.Equal(t, map[string]string{"title": "A B", "author": "C  D E"}, back.AllMetadata())
	assert.Equal(t, map[string]any{"TYPE": "VIDEO", "FILE": "f 1", "OK": "kept", "NOTE": "n1 n2"}, fileData(back))
}

// A header value that holds a space and a # reads back without the comment, because the reader drops it.
func TestWriteHeaderValueWithACommentIsLossy(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, time.Second, "x"))
	s.SetMetadata(model.MetadataTitle, "Part #2")
	back, _ := mustRead(t, writeBare(t, s), false)
	assert.Equal(t, map[string]string{"title": "Part"}, back.AllMetadata())
}

func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, time.Second, "Hello", "World"))
	body := header + "\n0 1\nHello\nWorld\n"
	assert.Equal(t, "\xEF\xBB\xBF"+body, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{}))
	assert.Equal(t, body, write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{}))
	assert.Equal(t, "\xEF\xBB\xBF"+body, write(t, s, model.CommonWriteOptions{BOM: model.BOMAdd}, mpsub.WriteOptions{}))
	crlf := write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, mpsub.WriteOptions{})
	assert.Equal(t, "TITLE=\r\nAUTHOR=\r\nTYPE=VIDEO\r\nFORMAT=TIME\r\n"+goNote[:len(goNote)-1]+"\r\n\r\n0 1\r\nHello\r\nWorld\r\n", crlf)
}

// The largest time must read back with the reader, at any frame rate. A frame is coarser than a millisecond below
// 1000 frames per second, so the frame of 99999:59:59.999 can round up to 100000 hours.
func TestWriteStaysBelowTheTimeLimit(t *testing.T) {
	t.Parallel()

	const limit = timecode.MaxTime
	tests := []struct {
		name string
		fps  float64
		end  time.Duration
		want string
	}{
		{"seconds, 99999:59:59.999", 0, limit - 1*ms, "\n0 359999999.999\n"},
		{"25 fps, 99999:59:59.999 clamps", 25, limit - 1*ms, "\n0 8999999999\n"},
		{"25 fps, 99999:59:59.995 clamps", 25, limit - 5*ms, "\n0 8999999999\n"},
		{"25 fps, 99999:59:59.980 clamps", 25, limit - 20*ms, "\n0 8999999999\n"},
		{"25 fps, 99999:59:59.900 stays", 25, limit - 100*ms, "\n0 8999999998\n"},
		{"1 fps, 99999:59:59.999 clamps", 1, limit - 1*ms, "\n0 359999999\n"},
		{"1 fps, 99999:59:59.400 stays", 1, limit - 600*ms, "\n0 359999999\n"},
		{"1 fps, 99999:59:58.400 stays", 1, limit - 1600*ms, "\n0 359999998\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := write(t, subtitle(cue(0, tt.end, "x")), model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{FrameRate: tt.fps})
			assert.Contains(t, got, tt.want)
			_, _, err := read(got, false)
			require.NoError(t, err)
		})
	}

	for _, fps := range []float64{1, 2, 5, 12, 24, 25, 29, 30, 48, 50, 60, 90, 100, 120, 240, 999, 1000, 1001, 2000, 4000, 48000, 1e6} {
		t.Run("round trip at "+strconv.FormatFloat(fps, 'f', 0, 64)+" fps", func(t *testing.T) {
			t.Parallel()

			for _, end := range []time.Duration{limit - 1*ms, limit - 2*ms, limit - 499*ms, limit - 500*ms, limit - 501*ms} {
				got := write(t, subtitle(cue(end-ms, end, "x")), model.CommonWriteOptions{}, mpsub.WriteOptions{FrameRate: fps})
				s, _, err := read(got, false)
				require.NoError(t, err, "end %v in %q", end, got)
				require.Equal(t, 1, s.Len())
				assert.LessOrEqual(t, s.Cues()[0].End, limit-ms)
			}
		})
	}
}

// A frame beyond the int64 range writes the digits of the rounded float, as MicroDVD does.
func TestWriteFrameBeyondTheIntRange(t *testing.T) {
	t.Parallel()

	got := write(t, subtitle(cue(0, 359999999*time.Second, "x")), model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{FrameRate: 1e12})
	assert.Contains(t, got, "\n0 359999999000000004096\nx\n")
}

func TestFramesRoundTripWithinHalfAFrame(t *testing.T) {
	t.Parallel()

	for _, fps := range []float64{1, 24, 25, 30, 60} {
		t.Run(strconv.FormatFloat(fps, 'f', 0, 64), func(t *testing.T) {
			t.Parallel()

			cues := make([]model.Cue, 0, 200)
			for i := range 200 {
				start := time.Duration(i*1234+17) * ms
				cues = append(cues, cue(start, start+time.Duration(i%7*311+5)*ms, "x"))
			}
			s := subtitle(cues...)
			back, _ := mustRead(t, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{FrameRate: fps}), false)
			require.Equal(t, s.Len(), back.Len())
			half := time.Duration(float64(time.Second) / fps / 2)
			for i, c := range s.All() {
				assert.InDelta(t, float64(c.Start), float64(back.Cues()[i].Start), float64(half+ms), "start %d", i)
				assert.InDelta(t, float64(c.End), float64(back.Cues()[i].End), float64(half+ms), "end %d", i)
			}
		})
	}
}

// Ports testFrameRateZeroThrows and testFractionalFrameRateThrows of tests/Formatters/MpSubFormatterTest.php of
// subtitle-toolbox-php. The zero value of FrameRate writes seconds, so zero is valid in Go.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fps  float64
		want string
	}{
		{"zero writes seconds", 0, ""},
		{"negative zero writes seconds", math.Copysign(0, -1), ""},
		{"25", 25, ""},
		{"1", 1, ""},
		{"a large whole rate", 1e15, ""},
		{"29.97", 29.97, "The MPSub frame rate must be a positive integer, got 29.97."},
		{"0.5", 0.5, "The MPSub frame rate must be a positive integer, got 0.5."},
		{"negative", -1, "The MPSub frame rate must be a positive integer, got -1."},
		{"negative and fractional", -25.5, "The MPSub frame rate must be a positive integer, got -25.5."},
		{"NaN", math.NaN(), "The MPSub frame rate must be a positive integer, got NAN."},
		{"infinity", math.Inf(1), "The MPSub frame rate must be a positive integer, got INF."},
		{"negative infinity", math.Inf(-1), "The MPSub frame rate must be a positive integer, got -INF."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := mpsub.WriteOptions{FrameRate: tt.fps}.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var arg *model.InvalidArgumentError
			require.ErrorAs(t, err, &arg)
			assert.Equal(t, "WriteOptions.MPSub.FrameRate", arg.Option)
			assert.Equal(t, tt.want, arg.Msg)
		})
	}
}

func TestWriteChecksTheOptions(t *testing.T) {
	t.Parallel()

	var out failingWriter
	err := mpsub.Write(out, subtitle(cue(0, time.Second, "x")), model.CommonWriteOptions{}, mpsub.WriteOptions{FrameRate: 29.97})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := mpsub.Write(failingWriter{}, subtitle(cue(0, time.Second, "x")), model.CommonWriteOptions{}, mpsub.WriteOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}
