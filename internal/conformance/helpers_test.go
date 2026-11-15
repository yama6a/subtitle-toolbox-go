// Package conformance runs the readers and writers of every format over testdata through the stbox facade.
package conformance_test

import (
	"bytes"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

// parseBytes reads content with stbox.Parse.
func parseBytes(content []byte, opts stbox.ReadOptions) (*stbox.Subtitle, stbox.ReadInfo, error) {
	return stbox.Parse(bytes.NewReader(content), int64(len(content)), opts)
}
