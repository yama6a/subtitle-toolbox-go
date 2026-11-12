package stbox_test

import (
	"bytes"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
)

// parseBytes reads content with stbox.Parse.
func parseBytes(content []byte, opts stbox.ReadOptions) (*stbox.Subtitle, stbox.ReadInfo, error) {
	return stbox.Parse(bytes.NewReader(content), int64(len(content)), opts)
}
