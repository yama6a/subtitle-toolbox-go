package repocheck

import (
	"errors"
	"go/build"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modulePath = "github.com/yama6a/subtitle-toolbox-go/v4"

type layer int

// The layers of the table in CONTRIBUTING.md. A package imports only packages of a lower layer.
const (
	layerLeaf      layer = iota // 0: helpers that import no model type
	layerModel                  // 1: model and markup
	layerModelUser              // 2: helpers that work on model types
	layerFormat                 // 3
	layerContainer              // 4
	layerService                // 5: the root, the services and the packages below a service folder
	layerCLI                    // 6
)

// layers holds the layer of each package. A key that ends in "/*" covers each package one folder below it.
var layers = map[string]layer{
	"internal/timecode":                  layerLeaf,
	"internal/intkey":                    layerLeaf,
	"format/internal/formatdata":         layerModelUser,
	"internal/grapheme":                  layerLeaf,
	"internal/ssatag":                    layerLeaf,
	"internal/textenc":                   layerLeaf,
	"format/internal/xmlraw":             layerLeaf,
	"format/internal/codepage":           layerLeaf,
	"internal/caption/cea608":            layerLeaf,
	"internal/caption/cea708":            layerLeaf,
	"container/mkv/internal/ebml":        layerLeaf,
	"internal/orderedjson":               layerLeaf,
	"internal/markupcore":                layerLeaf,
	"internal/speakerlabel":              layerLeaf,
	"container/mpegts/internal/teletext": layerLeaf,
	"internal/linewrap":                  layerLeaf,
	"internal/testfiles":                 layerLeaf,
	"internal/translationtest":           layerLeaf,
	"internal/extfallback":               layerLeaf,
	"format/internal/paragraphs":         layerLeaf,
	"markup":                             layerModel,
	"model":                              layerModel,
	"internal/textblock":                 layerModelUser,
	"internal/subrip":                    layerModelUser,
	"internal/vtt":                       layerModelUser,
	"internal/position":                  layerModelUser,
	"internal/parsing":                   layerModelUser,
	"internal/writing":                   layerModelUser,
	"internal/caption":                   layerModelUser,
	"format/internal/ttmldoc":            layerModelUser,
	"format/internal/wordgroup":          layerModelUser,
	"internal/bitmap":                    layerModelUser,
	"internal/bitmap/dvdsub":             layerModelUser,
	"internal/bitmap/displayset":         layerModelUser,
	"container/mpegts/internal/arib":     layerModelUser,
	"internal/bitmap/dvbsub":             layerModelUser,
	"internal/readingspeed":              layerModelUser,
	"format/*":                           layerFormat,
	"container/*":                        layerContainer,
	".":                                  layerService,
	"stats":                              layerService,
	"stream":                             layerService,
	"hearingimpaired":                    layerService,
	"speakers":                           layerService,
	"fixing":                             layerService,
	"profanity":                          layerService,
	"refsync":                            layerService,
	"diff":                               layerService,
	"dual":                               layerService,
	"karaoke":                            layerService,
	"validation":                         layerService,
	"timing":                             layerService,
	"imaging":                            layerService,
	"ocr":                                layerService,
	"ocr/*":                              layerService,
	"ocr/auto":                           layerService,
	"translation":                        layerService,
	"translation/internal/httppost":      layerService,
	"translation/*":                      layerService,
	"hls":                                layerService,
	"resegment":                          layerService,
	"internal/cli":                       layerCLI,
	"cmd/stbox":                          layerCLI,
}

// profiles lists the format packages that may import another format package, because their format is a profile of it.
var profiles = map[string]string{"format/itt": "format/ttml"}

// engineUsers lists the packages below a service folder that may import engine packages of the same service.
var engineUsers = map[string][]string{"ocr/auto": {"ocr/glyph", "ocr/tesseract"}}

// allowedModules are the modules that CLAUDE.md allows outside tests.
var allowedModules = []string{
	"golang.org/x/text",
	"github.com/rivo/uniseg",
	"github.com/Eyevinn/mp4ff",
	"github.com/asticode/go-astits",
	"github.com/dlclark/regexp2/v2",
	"github.com/yama6a/go-glyph-ocr",
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
		if dep < own || profiles[rel] == depRel {
			continue
		}
		if dep == own && sameLayerAllowed(own, rel, depRel) {
			continue
		}
		problems = append(problems, rel+" imports "+depRel+", which is not in a lower layer")
	}
	return problems
}

func sameLayerAllowed(l layer, rel, dep string) bool {
	if l == layerService {
		return serviceImportAllowed(rel, dep)
	}
	return l != layerFormat && l != layerContainer
}

// serviceImportAllowed reports whether a layer 5 package may import another layer 5 package.
// Only a package below a service folder may do so, and only for its own service.
func serviceImportAllowed(rel, dep string) bool {
	if rel == "." || dep == "." {
		return false
	}
	service, below, _ := strings.Cut(rel, "/")
	depService, _, _ := strings.Cut(dep, "/")
	if below == "" || service != depService {
		return false
	}
	return dep == service || strings.HasPrefix(dep, service+"/internal/") || slices.Contains(engineUsers[rel], dep)
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
			imports: []string{modulePath + "/model", modulePath + "/markup", modulePath + "/internal/textblock", "golang.org/x/text/encoding"},
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
			imports: []string{modulePath + "/format/internal/codepage"},
		},
		{
			name:    "helper on model types imports another helper",
			rel:     "internal/subrip",
			imports: []string{modulePath + "/internal/textblock", modulePath + "/internal/parsing", modulePath + "/model"},
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
			name:    "model imports markup",
			rel:     "model",
			imports: []string{modulePath + "/markup"},
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
			name:    "engine and test helper import ocr",
			rel:     "ocr/ocrtest",
			imports: []string{modulePath + "/ocr", modulePath + "/model"},
		},
		{
			name:    "chooser imports ocr and an engine",
			rel:     "ocr/auto",
			imports: []string{modulePath + "/ocr", modulePath + "/ocr/glyph"},
		},
		{
			name:    "ocr imports its test helper",
			rel:     "ocr",
			imports: []string{modulePath + "/ocr/ocrtest"},
			want:    []string{"ocr imports ocr/ocrtest, which is not in a lower layer"},
		},
		{
			name:    "engine imports another engine",
			rel:     "ocr/glyph",
			imports: []string{modulePath + "/ocr/tesseract"},
			want:    []string{"ocr/glyph imports ocr/tesseract, which is not in a lower layer"},
		},
		{
			name:    "engine imports the service, its HTTP helper and model",
			rel:     "translation/deepl",
			imports: []string{modulePath + "/translation", modulePath + "/translation/internal/httppost", modulePath + "/model"},
		},
		{
			name:    "HTTP helper imports an engine",
			rel:     "translation/internal/httppost",
			imports: []string{modulePath + "/translation/deepl"},
			want:    []string{"translation/internal/httppost imports translation/deepl, which is not in a lower layer"},
		},
		{
			name:    "engine imports another engine of translation",
			rel:     "translation/deepl",
			imports: []string{modulePath + "/translation/google"},
			want:    []string{"translation/deepl imports translation/google, which is not in a lower layer"},
		},
		{
			name:    "ocr imports an engine",
			rel:     "ocr",
			imports: []string{modulePath + "/ocr/glyph"},
			want:    []string{"ocr imports ocr/glyph, which is not in a lower layer"},
		},
		{
			name:    "translation imports ocr",
			rel:     "translation",
			imports: []string{modulePath + "/ocr"},
			want:    []string{"translation imports ocr, which is not in a lower layer"},
		},
		{
			name:    "test helper imports translation",
			rel:     "internal/translationtest",
			imports: []string{modulePath + "/translation"},
			want:    []string{"internal/translationtest imports translation, which is not in a lower layer"},
		},
		{
			name:    "root imports a service",
			rel:     ".",
			imports: []string{modulePath + "/stats"},
			want:    []string{". imports stats, which is not in a lower layer"},
		},
		{
			name:    "helper on model types imports a format",
			rel:     "internal/textblock",
			imports: []string{modulePath + "/format/srt"},
			want:    []string{"internal/textblock imports format/srt, which is not in a lower layer"},
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
