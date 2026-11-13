package stbox_test

import (
	"testing"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func TestAWSTranscribeWriteGoldens(t *testing.T) {
	t.Parallel()

	testCloudWriteGoldens(t, "aws-transcribe", stbox.AWSTranscribe, "library_speakers_language_id.json")
}

func TestAWSTranscribeRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "aws-transcribe")
}
