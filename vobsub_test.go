package stbox_test

import (
	"bytes"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// pairFS returns a file system with the VobSub pair two-tracks-pal under the names of idx and sub.
func pairFS(t *testing.T, idx, sub string) fstest.MapFS {
	t.Helper()

	return fstest.MapFS{
		idx: {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")},
		sub: {Data: testfiles.Read(t, "vobsub/two-tracks-pal.sub")},
	}
}

func TestLoadFSFindsThePairOfAVobSubFile(t *testing.T) {
	t.Parallel()

	windows1252 := []byte("# VobSub index file, v7\nsize: 720x576\npalette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, " +
		"000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000\nid: Fr\xe9, index: 0\ntimestamp: 00:00:01:000, filepos: 000000000\n")
	sub := testfiles.Read(t, "vobsub/one-packet-pal.sub")
	tests := []struct {
		name         string
		fsys         fs.FS
		path         string
		opts         stbox.ReadOptions
		wantCues     int
		wantLanguage string
		wantErr      error
		wantMessage  string
	}{
		{"an .idx name looks for .sub", pairFS(t, "movie.idx", "movie.sub"), "movie.idx", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"an .idx name looks for .SUB", pairFS(t, "movie.idx", "movie.SUB"), "movie.idx", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"upper-case extensions", pairFS(t, "MOVIE.IDX", "MOVIE.SUB"), "MOVIE.IDX", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"a .sub name with the format looks for .idx", pairFS(t, "movie.idx", "movie.sub"), "movie.sub", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a .sub name with the format looks for .IDX", pairFS(t, "movie.IDX", "movie.sub"), "movie.sub", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a name without an extension gets a dot", pairFS(t, "movie.idx", "movie."), "movie.", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a name without a dot", pairFS(t, "movie.idx", "movie"), "movie", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"the track option", pairFS(t, "movie.idx", "movie.sub"), "movie.idx", stbox.ReadOptions{VobSub: vobsub.ReadOptions{Language: "de"}}, 2, "de", nil, ""},
		{"the encoding decodes the .idx file", fstest.MapFS{"a.idx": {Data: windows1252}, "a.sub": {Data: sub}}, "a.idx", stbox.ReadOptions{Encoding: "Windows-1252"}, 1, "Fr\u00e9", nil, ""},
		{
			"a missing .sub file",
			fstest.MapFS{"movie.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")}},
			"movie.idx",
			stbox.ReadOptions{},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.sub next to movie.idx: file does not exist",
		},
		{
			"a missing .idx file",
			fstest.MapFS{"movie.sub": {Data: sub}},
			"movie.sub",
			stbox.ReadOptions{Format: stbox.VobSub},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.idx next to movie.sub: file does not exist",
		},
		{
			"a directory is no partner",
			fstest.MapFS{"movie.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")}, "movie.sub/x": {}},
			"movie.idx",
			stbox.ReadOptions{},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.sub next to movie.idx: file does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.LoadFS(tt.fsys, tt.path, tt.opts)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.EqualError(t, err, tt.wantMessage)
				return
			}
			require.NoError(t, err)
			language, _ := s.Metadata("language")
			assert.Equal(t, [4]any{tt.wantCues, tt.wantLanguage, stbox.VobSub, ""}, [4]any{s.Len(), language, info.Format, info.SourceEncoding})
		})
	}
}

func TestLoadVobSubFromFiles(t *testing.T) {
	t.Parallel()

	idx := testfiles.Path(t, "vobsub/two-tracks-pal.idx")
	sub := testfiles.Path(t, "vobsub/two-tracks-pal.sub")
	for _, tt := range []struct {
		name, path string
		opts       stbox.ReadOptions
	}{
		{"the .idx file", idx, stbox.ReadOptions{}},
		{"the .sub file with the format", sub, stbox.ReadOptions{Format: stbox.VobSub}},
		{"the .idx file with a wrong IDX option", idx, stbox.ReadOptions{VobSub: vobsub.ReadOptions{IDX: "wrong"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(tt.path, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, [3]any{5, stbox.VobSub, stbox.Container("")}, [3]any{s.Len(), info.Format, info.Container})
		})
	}
}

func TestLoadOfAMicroDVDSubFileNeverLooksForAnIndex(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"a.sub": {Data: testfiles.Read(t, "microdvd/valid.sub")},
		"a.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")},
	}
	_, info, err := stbox.LoadFS(fsys, "a.sub", stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.MicroDVD, info.Format)
}

func TestParseVobSub(t *testing.T) {
	t.Parallel()

	idx := testfiles.Read(t, "vobsub/two-tracks-pal.idx")
	sub := testfiles.Read(t, "vobsub/two-tracks-pal.sub")
	read := func(opts stbox.ReadOptions) (int, error) {
		s, _, err := stbox.Parse(bytes.NewReader(sub), int64(len(sub)), opts)
		if err != nil {
			return 0, err
		}
		return s.Len(), nil
	}

	n, err := read(stbox.ReadOptions{Format: stbox.VobSub, VobSub: vobsub.ReadOptions{IDX: string(idx), Index: new(1)}})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	_, err = read(stbox.ReadOptions{Format: stbox.VobSub})
	var aerr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.VobSub.IDX", aerr.Option)

	_, err = read(stbox.ReadOptions{Format: stbox.VobSub, VobSub: vobsub.ReadOptions{IDX: string(idx), Index: new(-1)}})
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.VobSub.Index", aerr.Option)
}
