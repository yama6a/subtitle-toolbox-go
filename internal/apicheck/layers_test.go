package apicheck

import (
	"errors"
	"go/build"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modulePath = "github.com/yama6a/subtitle-toolbox-go"

type layer int

// The layers of the table in CONTRIBUTING.md. A package imports only packages of a lower layer.
const (
	layerLeaf      layer = iota // 0
	layerMarkup                 // 1
	layerModel                  // 2
	layerModelUser              // 3
	layerFormat                 // 4
	layerContainer              // 4b
	layerFacade                 // 5
	layerCLI                    // 6
)

// layers holds the layer of each package. A key that ends in "/*" covers each package one folder below it.
var layers = map[string]layer{
	"internal/timecode":    layerLeaf,
	"internal/phpkey":      layerLeaf,
	"internal/grapheme":    layerLeaf,
	"internal/ssatag":      layerLeaf,
	"internal/textenc":     layerLeaf,
	"internal/xmlraw":      layerLeaf,
	"internal/codepage":    layerLeaf,
	"internal/cea608":      layerLeaf,
	"internal/cea708":      layerLeaf,
	"internal/ebml":        layerLeaf,
	"internal/orderedjson": layerLeaf,
	"internal/markupcore":  layerLeaf,
	"internal/linewrap":    layerLeaf,
	"internal/testfiles":   layerLeaf,
	"internal/golden":      layerLeaf,
	"internal/extfallback": layerLeaf,
	"internal/paragraphs":  layerLeaf,
	"markup":               layerMarkup,
	"model":                layerModel,
	"internal/blocks":      layerModelUser,
	"internal/subrip":      layerModelUser,
	"internal/vtt":         layerModelUser,
	"internal/position":    layerModelUser,
	"internal/parsing":     layerModelUser,
	"internal/writing":     layerModelUser,
	"internal/caption":     layerModelUser,
	"internal/ttmlwrite":   layerModelUser,
	"internal/wordgroup":   layerModelUser,
	"format/*":             layerFormat,
	"container/*":          layerContainer,
	".":                    layerFacade,
	"stats":                layerFacade,
	"stream":               layerFacade,
	"hearingimpaired":      layerFacade,
	"fixing":               layerFacade,
	"profanity":            layerFacade,
	"refsync":              layerFacade,
	"diff":                 layerFacade,
	"dual":                 layerFacade,
	"internal/cli":         layerCLI,
	"cmd/stbox":            layerCLI,
}

// allowedModules are the modules that CLAUDE.md allows outside tests.
var allowedModules = []string{
	"golang.org/x/text",
	"github.com/rivo/uniseg",
	"github.com/Eyevinn/mp4ff",
	"github.com/asticode/go-astits",
	"github.com/dlclark/regexp2/v2",
}

func TestPackageLayers(t *testing.T) {
	t.Parallel()

	var checked int
	err := filepath.WalkDir(moduleRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(moduleRoot, p)
		require.NoError(t, err)
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if rel != "." && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
			return filepath.SkipDir
		}
		pkg, err := build.ImportDir(p, 0)
		var noGo *build.NoGoError
		if errors.As(err, &noGo) || (err == nil && len(pkg.GoFiles) == 0) {
			return nil
		}
		require.NoError(t, err)
		checked++
		for _, problem := range checkImports(rel, pkg.Imports) {
			t.Error(problem)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Positive(t, checked)
}

// checkImports returns one line for each import of the package at rel that the layer table forbids.
func checkImports(rel string, imports []string) []string {
	own, ok := layerOf(rel)
	if !ok {
		return []string{rel + " has no layer. Add it to the layer table in CONTRIBUTING.md and in layers_test.go"}
	}
	var problems []string
	for _, imp := range imports {
		depRel, inModule := strings.CutPrefix(imp, modulePath)
		if !inModule {
			if !isStandard(imp) && !isAllowedModule(imp) {
				problems = append(problems, rel+" imports "+imp+", which is not an allowed module")
			}
			continue
		}
		depRel = strings.TrimPrefix(depRel, "/")
		if depRel == "" {
			depRel = "."
		}
		dep, ok := layerOf(depRel)
		if !ok {
			continue
		}
		if dep < own || (dep == own && (own == layerLeaf || own == layerModelUser || own == layerCLI)) {
			continue
		}
		problems = append(problems, rel+" imports "+depRel+", which is not in a lower layer")
	}
	return problems
}

func layerOf(rel string) (layer, bool) {
	if l, ok := layers[rel]; ok {
		return l, true
	}
	if rel == "." {
		return 0, false
	}
	l, ok := layers[path.Dir(rel)+"/*"]
	return l, ok
}

func isStandard(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".")
}

func isAllowedModule(imp string) bool {
	for _, m := range allowedModules {
		if imp == m || strings.HasPrefix(imp, m+"/") {
			return true
		}
	}
	return false
}

func TestCheckImports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rel     string
		imports []string
		want    []string
	}{
		{
			name:    "root imports formats, model and the standard library",
			rel:     ".",
			imports: []string{"io", modulePath + "/format/srt", modulePath + "/model", modulePath + "/internal/textenc"},
		},
		{
			name:    "format imports layers 0 to 3 and an allowed module",
			rel:     "format/srt",
			imports: []string{modulePath + "/model", modulePath + "/markup", modulePath + "/internal/blocks", "golang.org/x/text/encoding"},
		},
		{
			name:    "format imports the allowed regexp2 v2 module",
			rel:     "format/srt",
			imports: []string{"github.com/dlclark/regexp2/v2"},
		},
		{
			name:    "format imports the regexp2 v1 module",
			rel:     "format/srt",
			imports: []string{"github.com/dlclark/regexp2"},
			want:    []string{"format/srt imports github.com/dlclark/regexp2, which is not an allowed module"},
		},
		{
			name:    "leaf helper imports another leaf helper",
			rel:     "internal/textenc",
			imports: []string{modulePath + "/internal/codepage"},
		},
		{
			name:    "helper on model types imports another helper",
			rel:     "internal/subrip",
			imports: []string{modulePath + "/internal/blocks", modulePath + "/internal/parsing", modulePath + "/model"},
		},
		{
			name:    "CLI imports everything",
			rel:     "cmd/stbox",
			imports: []string{modulePath, modulePath + "/internal/cli", modulePath + "/stats", modulePath + "/format/srt"},
		},
		{
			name:    "model imports the root",
			rel:     "model",
			imports: []string{modulePath},
			want:    []string{"model imports ., which is not in a lower layer"},
		},
		{
			name:    "markup imports model",
			rel:     "markup",
			imports: []string{modulePath + "/model"},
			want:    []string{"markup imports model, which is not in a lower layer"},
		},
		{
			name:    "format imports another format",
			rel:     "format/webvtt",
			imports: []string{modulePath + "/format/srt"},
			want:    []string{"format/webvtt imports format/srt, which is not in a lower layer"},
		},
		{
			name:    "service imports the root",
			rel:     "stats",
			imports: []string{modulePath},
			want:    []string{"stats imports ., which is not in a lower layer"},
		},
		{
			name:    "root imports a service",
			rel:     ".",
			imports: []string{modulePath + "/stats"},
			want:    []string{". imports stats, which is not in a lower layer"},
		},
		{
			name:    "helper on model types imports a format",
			rel:     "internal/blocks",
			imports: []string{modulePath + "/format/srt"},
			want:    []string{"internal/blocks imports format/srt, which is not in a lower layer"},
		},
		{
			name:    "container imports a format",
			rel:     "container/mkv",
			imports: []string{modulePath + "/format/srt"},
		},
		{
			name:    "test module outside tests",
			rel:     "model",
			imports: []string{"github.com/stretchr/testify/assert"},
			want:    []string{"model imports github.com/stretchr/testify/assert, which is not an allowed module"},
		},
		{
			name: "package without a layer",
			rel:  "internal/misc",
			want: []string{"internal/misc has no layer. Add it to the layer table in CONTRIBUTING.md and in layers_test.go"},
		},
		{
			name: "format pattern covers one folder only",
			rel:  "format/srt/extra",
			want: []string{"format/srt/extra has no layer. Add it to the layer table in CONTRIBUTING.md and in layers_test.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, checkImports(tt.rel, tt.imports))
		})
	}
}
