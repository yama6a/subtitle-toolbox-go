package stbox

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFile struct{ tracks []Track }

func (f fakeFile) Tracks() []Track { return f.tracks }

func (f fakeFile) Extract(int, ReadOptions) (*Subtitle, []ParseWarning, error) {
	s := &Subtitle{}
	s.AddCue(Cue{Start: 0, End: 1000000000, Lines: []string{"Hello"}})
	return s, []ParseWarning{{Message: "repaired", Line: NoLine, Block: NoBlock, Action: Repaired}}, nil
}

func fakeTracks(numbers ...int) []Track {
	tracks := make([]Track, len(numbers))
	for i, n := range numbers {
		tracks[i] = Track{Container: Matroska, Number: n, CodecID: "S_TEXT/UTF8", Format: SubRip}
	}
	return tracks
}

func TestReadTrack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tracks     []Track
		opts       ReadOptions
		wantTrack  int
		wantErr    string
		wantTracks int
		wantType   any
	}{
		{name: "the only track", tracks: fakeTracks(4), wantTrack: 4},
		{name: "the second of 3 tracks", tracks: fakeTracks(1, 2, 3), opts: ReadOptions{Track: 2}, wantTrack: 2},
		{
			name: "no track", wantType: &TrackError{Container: Matroska},
			wantErr: "stbox: unsupported format: The MKV or WebM file has no subtitle track.",
		},
		{
			name: "6 tracks", tracks: fakeTracks(1, 2, 3, 4, 5, 6), wantTracks: 6, wantType: &TrackError{},
			wantErr: "stbox: unsupported format: The MKV or WebM file has 6 subtitle tracks.",
		},
		{
			name: "a track that does not exist", tracks: fakeTracks(1, 2), opts: ReadOptions{Track: 7},
			wantType: &InvalidArgumentError{},
			wantErr:  "stbox: invalid argument: ReadOptions.Track: The file has no subtitle track with the number 7.",
		},
		{
			name: "a format", tracks: fakeTracks(1), opts: ReadOptions{Format: SubRip},
			wantErr: "stbox: unsupported format: movie.mkv is an MKV or WebM file. Read it by track.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := containerFuncs{container: Matroska, open: func(io.ReaderAt, int64) (containerFile, error) {
				return fakeFile{tt.tracks}, nil
			}}
			s, info, err := readTrack(row, bytes.NewReader(nil), 0, "movie.mkv", tt.opts)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, s)
				assert.Zero(t, info)
				var te *TrackError
				var ie *InvalidArgumentError
				switch tt.wantType.(type) {
				case *TrackError:
					require.ErrorAs(t, err, &te)
					assert.Len(t, te.Tracks, tt.wantTracks)
					assert.Equal(t, Matroska, te.Container)
					require.ErrorIs(t, err, ErrUnsupportedFormat)
					require.NotErrorIs(t, err, ErrUnknownFormat)
				case *InvalidArgumentError:
					require.ErrorAs(t, err, &ie)
					assert.Equal(t, "ReadOptions.Track", ie.Option)
				default:
					require.ErrorIs(t, err, ErrUnsupportedFormat)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 1, s.Len())
			assert.Equal(t, ReadInfo{
				Format: SubRip, Container: Matroska, Track: tt.wantTrack,
				Warnings: []ParseWarning{{Message: "repaired", Line: NoLine, Block: NoBlock, Action: Repaired}},
			}, info)
		})
	}
}

func TestTrackInputErrors(t *testing.T) {
	t.Parallel()

	srt := []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	ts := make([]byte, 377)
	ts[0], ts[188], ts[376] = 0x47, 0x47, 0x47
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.ts")
	require.NoError(t, os.WriteFile(path, ts, 0o600))
	srtPath := filepath.Join(dir, "a.srt")
	require.NoError(t, os.WriteFile(srtPath, srt, 0o600))
	const noTrackSize = "stbox: invalid argument: The size must be 0 or more, got -1."
	const notContainer = "stbox: unsupported format: The content is not an MKV, WebM, MP4 or MPEG-TS file."
	const onlyContainers = "stbox: invalid argument: ReadOptions.Track: Only MKV, WebM, MP4 and MPEG-TS files have tracks."
	const noReader = " is an MPEG-TS file. This version cannot read MPEG-TS files."

	tests := []struct {
		name string
		call func() error
		want string
		is   error
	}{
		{"Parse with a track on SubRip", func() error {
			_, _, err := Parse(bytes.NewReader(srt), int64(len(srt)), ReadOptions{Track: 3})
			return err
		}, onlyContainers, ErrInvalidArgument},
		{"Load with a track on SubRip", func() error {
			_, _, err := Load(srtPath, ReadOptions{Track: 3})
			return err
		}, onlyContainers, ErrInvalidArgument},
		{"Tracks of SubRip", func() error {
			_, err := Tracks(bytes.NewReader(srt), int64(len(srt)))
			return err
		}, notContainer, ErrUnsupportedFormat},
		{"a negative track", func() error {
			_, _, err := Parse(bytes.NewReader(srt), int64(len(srt)), ReadOptions{Track: -1})
			return err
		}, "stbox: invalid argument: ReadOptions.Track: The track number must be 0 or more, got -1.", ErrInvalidArgument},
		{"Parse of an empty reader with a size", func() error {
			_, _, err := Parse(bytes.NewReader(nil), 5, ReadOptions{Track: 1})
			return err
		}, onlyContainers, ErrInvalidArgument},
		{"Parse with a negative size", func() error {
			_, _, err := Parse(bytes.NewReader(srt), -1, ReadOptions{})
			return err
		}, noTrackSize, ErrInvalidArgument},
		{"Tracks with a negative size", func() error {
			_, err := Tracks(bytes.NewReader(srt), -1)
			return err
		}, noTrackSize, ErrInvalidArgument},
		{"Load of MPEG-TS without a reader", func() error {
			_, _, err := Load(path, ReadOptions{})
			return err
		}, "stbox: unsupported format: " + path + noReader, ErrUnsupportedFormat},
		{"Parse of MPEG-TS without a reader, with a format", func() error {
			_, _, err := Parse(bytes.NewReader(ts), int64(len(ts)), ReadOptions{Format: SubRip})
			return err
		}, "stbox: unsupported format: The content" + noReader, ErrUnsupportedFormat},
		{"Parse of MPEG-TS without a reader", func() error {
			_, _, err := Parse(bytes.NewReader(ts), int64(len(ts)), ReadOptions{})
			return err
		}, "stbox: unsupported format: The content" + noReader, ErrUnsupportedFormat},
		{"Tracks of MPEG-TS without a reader", func() error {
			_, err := Tracks(bytes.NewReader(ts), int64(len(ts)))
			return err
		}, "stbox: unsupported format: The content" + noReader, ErrUnsupportedFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call()
			require.EqualError(t, err, tt.want)
			require.ErrorIs(t, err, tt.is)
			assert.NotErrorIs(t, err, ErrUnknownFormat)
		})
	}
}
