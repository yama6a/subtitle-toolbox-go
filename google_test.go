package stbox_test

import (
	"testing"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func TestGoogleSpeechWriteGoldens(t *testing.T) {
	t.Parallel()

	testCloudWriteGoldens(t, "google-speech", stbox.GoogleSpeech, "restaurant_v2_diarization.json")
}

func TestGoogleSpeechRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "google-speech")
}
