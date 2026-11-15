package webvtt_test

import (
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func taggedText(c model.Cue) string { return strings.Join(c.TaggedLines(), "\n") }
