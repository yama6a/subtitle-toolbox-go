package stbox_test

import (
	"testing"
)

func TestGoogleSpeechRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "google-speech")
}
