package model_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestOptionEnumStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value fmt.Stringer
		want  string
	}{
		{model.LF, "lf"},
		{model.CRLF, "crlf"},
		{model.LineEnding(2), "LineEnding(2)"},
		{model.BOMDefault, "default"},
		{model.BOMAdd, "add"},
		{model.BOMRemove, "remove"},
		{model.BOM(3), "BOM(3)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.value.String())
		})
	}
}

func TestCommonWriteOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		opts       model.CommonWriteOptions
		path       string
		wantOption string
		wantMsg    string
	}{
		{name: "valid", opts: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}},
		{
			name: "bad line ending", opts: model.CommonWriteOptions{LineEnding: 2},
			wantOption: "CommonWriteOptions.LineEnding", wantMsg: "The line ending 2 is unknown.",
		},
		{
			name: "bad BOM", opts: model.CommonWriteOptions{BOM: 3},
			wantOption: "CommonWriteOptions.BOM", wantMsg: "The BOM rule 3 is unknown.",
		},
		{
			name: "ValidateAt with a path", opts: model.CommonWriteOptions{BOM: 3}, path: "WriteOptions",
			wantOption: "WriteOptions.BOM", wantMsg: "The BOM rule 3 is unknown.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.path != "" {
				err = tt.opts.ValidateAt(tt.path)
			}
			if tt.wantMsg == "" {
				require.NoError(t, err)
				return
			}
			var e *model.InvalidArgumentError
			require.ErrorAs(t, err, &e)
			assert.Equal(t, tt.wantOption, e.Option)
			assert.Equal(t, tt.wantMsg, e.Msg)
		})
	}
}
