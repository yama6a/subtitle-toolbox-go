package stbox_test

import (
	"testing"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func TestAssemblyAIWriteGoldens(t *testing.T) {
	t.Parallel()

	testCloudWriteGoldens(t, "assemblyai", stbox.AssemblyAI, "hike_utterances_speakers.json")
}

func TestAssemblyAIRealFiles(t *testing.T) {
	t.Parallel()

	testCloudRealFiles(t, "assemblyai")
}
