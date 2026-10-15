package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoFormatIsBinary(t *testing.T) {
	t.Parallel()

	for _, row := range formatFacts {
		assert.False(t, row.binary, string(row.format))
	}
}
