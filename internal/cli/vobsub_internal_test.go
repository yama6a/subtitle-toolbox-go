package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// vobSubDir returns a directory with the pair two-tracks-pal and a MicroDVD file frames.sub from testdata/cli.
func vobSubDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	for _, name := range []string{"two-tracks-pal.idx", "two-tracks-pal.sub"} {
		copyFile(t, testfiles.Path(t, "vobsub/"+name), filepath.Join(dir, name))
	}
	return dir
}

func TestVobSubInputRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		stdin string
		want  result
	}{
		{
			"standard input is a usage error before any read",
			[]string{"convert", "-", "--from", "vobsub", "--to", "srt"},
			"# VobSub index file",
			usageResult("convert", "VobSub needs the path of the .idx file. Standard input does not work."),
		},
		{
			"no output may overwrite the .sub file",
			[]string{"convert", "two-tracks-pal.idx", "--to", "microdvd", "-o", "two-tracks-pal.sub"},
			"",
			usageResult("convert", "The output two-tracks-pal.sub is a file that the command reads. Pass another output file or directory."),
		},
		{
			"no output may overwrite the .idx file of a .sub input",
			[]string{"convert", "two-tracks-pal.sub", "--from", "vobsub", "-o", "two-tracks-pal.idx"},
			"",
			usageResult("convert", "The output two-tracks-pal.idx is a file that the command reads. Pass another output file or directory."),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: vobSubDir(t), stdin: tt.stdin}.run(t, tt.words...))
		})
	}
}

func TestVobSubDirectoryInputIsTheIndexFile(t *testing.T) {
	t.Parallel()

	dir := vobSubDir(t)
	for from, to := range map[string]string{
		"one-packet-pal.sub": "d/MOVIE.SUB", "one-packet-pal.idx": "d/MOVIE.idx",
		"two-tracks-pal.sub": "d/Lone.SUB", "two-tracks-pal.idx": "d/lone.idx",
	} {
		copyFile(t, testfiles.Path(t, "vobsub/"+from), filepath.Join(dir, to))
	}
	copyFile(t, filepath.Join(dir, "frames.sub"), filepath.Join(dir, "d", "frames.sub"))
	r := &runner{c: &console{dir: dir}}

	got, err := r.expand([]string{"d"})
	require.NoError(t, err)
	assert.Equal(t, []string{"d/Lone.SUB", "d/MOVIE.idx", "d/frames.sub", "d/lone.idx"}, got, "Lone.SUB has no .idx file that opens it")
}
