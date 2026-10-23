package stringlist_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/stringlist"
)

func TestAny(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []any{"a", ""}, stringlist.Any([]string{"a", ""}))
	assert.Equal(t, []any{}, stringlist.Any(nil))
}
