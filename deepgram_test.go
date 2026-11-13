package stbox_test

import (
	"testing"
)

func TestDeepgramRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "deepgram")
}
