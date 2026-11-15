package formatdata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/formatdata"
)

func TestStrings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []any{"a", ""}, formatdata.Strings([]string{"a", ""}))
	assert.Equal(t, []any{}, formatdata.Strings(nil))
}
