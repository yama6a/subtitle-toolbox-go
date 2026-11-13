package auto_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/auto"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/glyph"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/tesseract"
)

func TestChoose(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("the fake program is a shell script")
	}
	fake := testfiles.Path(t, "ocr/fake-tesseract/tesseract")
	missing := testfiles.Path(t, "ocr/no-such-tesseract")
	tests := []struct {
		name    string
		opts    auto.Options
		want    ocr.EngineName
		wantErr string
	}{
		{"default picks Tesseract when it is installed", auto.Options{TesseractProgram: fake}, ocr.EngineTesseract, ""},
		{"default falls back to glyph", auto.Options{TesseractProgram: missing}, ocr.EngineGlyph, ""},
		{"explicit glyph skips the check", auto.Options{Engine: ocr.EngineGlyph, TesseractProgram: fake}, ocr.EngineGlyph, ""},
		{"explicit Tesseract that runs", auto.Options{Engine: ocr.EngineTesseract, TesseractProgram: fake}, ocr.EngineTesseract, ""},
		{
			"explicit Tesseract that is missing",
			auto.Options{Engine: ocr.EngineTesseract, TesseractProgram: missing},
			"",
			"Cannot run OCR with Tesseract: the program \"" + missing + "\" is missing. Install Tesseract with: ",
		},
		{
			"unknown engine",
			auto.Options{Engine: "easyocr"},
			"",
			`stbox: invalid argument: auto.Options.Engine: The OCR engine must be tesseract or glyph, got "easyocr".`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := auto.Choose(t.Context(), tt.opts)
			if tt.wantErr != "" {
				require.ErrorIs(t, err, model.ErrInvalidArgument)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			engine, err := auto.New(t.Context(), tt.opts)
			require.NoError(t, err)
			if tt.want == ocr.EngineTesseract {
				assert.IsType(t, &tesseract.Engine{}, engine)
			} else {
				assert.IsType(t, &glyph.Engine{}, engine)
			}
		})
	}
}

func ExampleChoose() {
	name, err := auto.Choose(context.Background(), auto.Options{Engine: ocr.EngineGlyph})
	fmt.Println(name, err)
	// Output:
	// glyph <nil>
}
