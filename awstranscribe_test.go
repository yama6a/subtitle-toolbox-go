package stbox_test

import (
	"testing"
)

func TestAWSTranscribeRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "aws-transcribe")
}
