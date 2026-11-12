package stbox_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// TestPACEncodingNamesTheCodePage covers decision D1: Encoding picks the PAC code page, and PAC.CodePage wins.
func TestPACEncodingNamesTheCodePage(t *testing.T) {
	t.Parallel()

	greek := testfiles.Read(t, "pac/real/weather_greek.pac")
	firstLine := func(t *testing.T, content []byte, opts stbox.ReadOptions) string {
		t.Helper()

		s, info, err := stbox.Parse(bytes.NewReader(content), opts)
		require.NoError(t, err)
		assert.Equal(t, stbox.PAC, info.Format)
		assert.Empty(t, info.SourceEncoding)
		return s.Cues()[0].Lines[0]
	}
	detected := firstLine(t, greek, stbox.ReadOptions{})
	cyrillic := firstLine(t, greek, stbox.ReadOptions{PAC: pac.ReadOptions{CodePage: pac.Cyrillic}})
	require.NotEqual(t, detected, cyrillic)

	tests := []struct {
		name     string
		opts     stbox.ReadOptions
		want     string
		option   string
		errorMsg string
	}{
		{name: "Greek name", opts: stbox.ReadOptions{Format: stbox.PAC, Encoding: "ISO-8859-7"}, want: detected},
		{name: "alias without case", opts: stbox.ReadOptions{Encoding: "windows-1253"}, want: detected},
		{name: "Cyrillic name", opts: stbox.ReadOptions{Encoding: "ISO-8859-5"}, want: cyrillic},
		{name: "field wins over the encoding", opts: stbox.ReadOptions{Encoding: "ISO-8859-5", PAC: pac.ReadOptions{CodePage: pac.Greek}}, want: detected},
		{
			name: "encoding without a PAC code page", opts: stbox.ReadOptions{Encoding: "KOI8-R"},
			option: "ReadOptions.Encoding", errorMsg: `PAC files have no code page for the encoding "KOI8-R".`,
		},
		{
			name: "spelling that textenc does not know", opts: stbox.ReadOptions{Encoding: "TIS-620"},
			option: "ReadOptions.Encoding", errorMsg: `The encoding "TIS-620" is unknown.`,
		},
		{
			name: "unknown code page field", opts: stbox.ReadOptions{PAC: pac.ReadOptions{CodePage: "klingon"}},
			option: "ReadOptions.PAC.CodePage", errorMsg: `The PAC code page "klingon" is unknown.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.option == "" {
				assert.Equal(t, tt.want, firstLine(t, greek, tt.opts))
				return
			}
			_, _, err := stbox.Parse(bytes.NewReader(greek), tt.opts)
			var ierr *stbox.InvalidArgumentError
			require.ErrorAs(t, err, &ierr)
			assert.Equal(t, tt.option, ierr.Option)
			assert.Equal(t, tt.errorMsg, ierr.Msg)
		})
	}
}

func TestPACLoadByExtensionAndContent(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"show.pac", "show.fpc", "show.bin"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(writeTemp(t, file, testfiles.Read(t, "pac/real/bakery_latin.pac")), stbox.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, stbox.PAC, info.Format)
			assert.Positive(t, s.Len())
		})
	}
}
