package itt_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	ttmlNS      = `xmlns="http://www.w3.org/ns/ttml"`
	parameterNS = "http://www.w3.org/ns/ttml#parameter"
	oneCue      = `<body><div><p begin="00:00:01:12" end="00:00:04:00">Hello</p></div></body></tt>`
)

func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		file    string
		lenient bool
		// want is the itt format data as key, value pairs in order, nil for none.
		want     []string
		start    time.Duration
		end      time.Duration
		cues     int
		warnings int
	}{
		{
			name: "SMPTE times at 23.976 fps",
			content: `<tt ` + ttmlNS + ` xmlns:ttp="` + parameterNS + `" ttp:timeBase="smpte" ttp:frameRate="24"` +
				` ttp:frameRateMultiplier=" 999 1000 ">` + oneCue,
			want:  []string{"timeBase", "smpte", "frameRate", "24", "frameRateMultiplier", "999 1000"},
			start: 1502 * time.Millisecond, end: 4004 * time.Millisecond, cues: 1,
		},
		{
			name: "other prefixes",
			content: `<tt ` + ttmlNS + ` xmlns:p="` + parameterNS + `" xmlns:x="urn:example" p:timeBase="smpte" p:frameRate="25"` +
				` x:frameRate="50">` + oneCue,
			want:  []string{"timeBase", "smpte", "frameRate", "25"},
			start: 1480 * time.Millisecond, end: 4 * time.Second, cues: 1,
		},
		{
			name:    "no parameters",
			content: `<tt ` + ttmlNS + `>` + oneCue,
			start:   1400 * time.Millisecond, end: 4 * time.Second, cues: 1,
		},
		{
			name:    "undeclared ttp prefix",
			content: `<tt ` + ttmlNS + ` ttp:frameRate="25" ttp:dropMode="nonDrop">` + oneCue,
			want:    []string{"frameRate", "25", "dropMode", "nonDrop"},
			start:   1480 * time.Millisecond, end: 4 * time.Second, cues: 1,
		},
		{
			name: "a parameter in 2 namespaces keeps the first",
			content: `<tt ` + ttmlNS + ` xmlns:ttp="` + parameterNS + `" xmlns:d="http://www.w3.org/2006/10/ttaf1#parameter"` +
				` ttp:frameRate="25" d:frameRate="30">` + oneCue,
			want:  []string{"frameRate", "25"},
			start: 1480 * time.Millisecond, end: 4 * time.Second, cues: 1,
		},
		{
			name:    "lenient mode repairs bad times",
			file:    "lenient/bad_begin.ttml",
			lenient: true,
			cues:    3, warnings: 2,
		},
		{
			name: "forced cues",
			file: "forced/forced_signs_2398.itt",
			want: []string{"timeBase", "smpte", "frameRate", "24", "frameRateMultiplier", "999 1000", "dropMode", "nonDrop"},
			cues: 6, start: 2002 * time.Millisecond, end: 4505 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := []byte(tt.content)
			if tt.file != "" {
				content = testfiles.Read(t, tt.file)
			}
			common := model.CommonReadOptions{Lenient: tt.lenient}
			s, warnings, err := itt.Read(bytes.NewReader(content), common, itt.ReadOptions{})
			require.NoError(t, err)

			data, ok := s.FormatData("itt")
			assert.Equal(t, tt.want != nil, ok)
			for i := 0; i < len(tt.want); i += 2 {
				got, _ := data.String(tt.want[i])
				assert.Equal(t, tt.want[i+1], got, tt.want[i])
			}
			assert.Equal(t, len(tt.want)/2, data.Len())
			require.Equal(t, tt.cues, s.Len())
			assert.Len(t, warnings, tt.warnings)
			if tt.start != 0 {
				first, _ := s.Cue(0)
				assert.Equal(t, tt.start, first.Start)
				assert.Equal(t, tt.end, first.End)
			}

			plain, plainWarnings, err := ttml.Read(bytes.NewReader(content), common, ttml.ReadOptions{})
			require.NoError(t, err)
			s.DeleteFormatData("itt")
			assert.Equal(t, plain, s)
			assert.Equal(t, plainWarnings, warnings)
		})
	}
}

func TestReadFollowsTheTTMLReaderForBadUTF8(t *testing.T) {
	t.Parallel()

	content := []byte(`<tt ` + ttmlNS + ` xmlns:ttp="` + parameterNS + `" ttp:frameRate="25"><body><div><p begin="1s" end="2s">a` +
		"\xff" + `b</p></div></body></tt>`)
	for _, lenient := range []bool{false, true} {
		common := model.CommonReadOptions{Lenient: lenient}
		_, wantWarnings, wantErr := ttml.Read(bytes.NewReader(content), common, ttml.ReadOptions{})
		s, warnings, err := itt.Read(bytes.NewReader(content), common, itt.ReadOptions{})
		if wantErr != nil {
			require.EqualError(t, err, wantErr.Error())
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, wantWarnings, warnings)
		cue, _ := s.Cue(0)
		assert.Equal(t, "a�b", cue.Text())
	}
}

func readFile(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	s, _, err := itt.Read(bytes.NewReader(testfiles.Read(t, name)), model.CommonReadOptions{}, itt.ReadOptions{})
	require.NoError(t, err)
	return s
}
