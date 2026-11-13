package scc_test

import (
	"fmt"
	"math/bits"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const header = "Scenarist_SCC V1.0\n\n"

// frameTime returns the time of a frame at 29.97 fps in whole milliseconds.
func frameTime(frame int64) time.Duration {
	return model.FrameRate2997.Duration(frame).Round(time.Millisecond)
}

// text packs ASCII text two characters per word with odd parity and pads an odd character with 80.
func text(s string) string {
	bytes := make([]byte, 0, len(s)+1)
	for _, c := range []byte(s) {
		if bits.OnesCount8(c)%2 == 0 {
			c |= 0x80
		}
		bytes = append(bytes, c)
	}
	if len(bytes)%2 == 1 {
		bytes = append(bytes, 0x80)
	}
	words := make([]string, 0, len(bytes)/2)
	for i := 0; i < len(bytes); i += 2 {
		words = append(words, fmt.Sprintf("%02x%02x", bytes[i], bytes[i+1]))
	}
	return strings.Join(words, " ")
}

func read(t *testing.T, content string, common model.CommonReadOptions, opts scc.ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return scc.Read(strings.NewReader(content), common, opts)
}

func mustRead(t *testing.T, content string, common model.CommonReadOptions, opts scc.ReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := read(t, content, common, opts)
	require.NoError(t, err)
	return s, warnings
}
