package stbox_test

import (
	"testing"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func TestDeepgramWriteGoldens(t *testing.T) {
	t.Parallel()

	testCloudWriteGoldens(t, "deepgram", stbox.Deepgram, "pool_utterances_diarize.json")
}

func TestDeepgramRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "deepgram")
}
