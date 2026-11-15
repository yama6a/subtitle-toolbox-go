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

const modulePath = "github.com/yama6a/subtitle-toolbox-go/v5"

type layer int

// The layers of the table in CONTRIBUTING.md. A package imports only packages of a lower layer.
const (
	layerBase      layer = iota // 0: model, model/markup and the helpers in internal folders
	layerFormat                 // 1
	layerContainer              // 2
	layerTop                    // 3: the root and every package under edit/ and inspect/
	layerCLI                    // 4
)

// profiles lists the format packages that may import another format package, because their format is a profile of it.
var profiles = map[string]string{"format/itt": "format/ttml", "format/hls": "format/webvtt"}

// engineUsers lists the packages below a service folder that may import engine packages of the same service.
var engineUsers = map[string][]string{"edit/ocr/auto": {"edit/ocr/glyph", "edit/ocr/tesseract"}}

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
		return []string{rel + " is outside the folders of the layer table in CONTRIBUTING.md"}
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
	switch l {
	case layerTop:
		return topImportAllowed(rel, dep)
	case layerFormat, layerContainer:
		return false
	case layerBase, layerCLI:
		return true
	}
	return false
}

// topImportAllowed reports whether a layer 3 package may import another layer 3 package.
// A package may import a helper below internal/ of its own group, or its own service and the helpers of that service.
func topImportAllowed(rel, dep string) bool {
	if rel == "." || dep == "." {
		return false
	}
	group, _, _ := strings.Cut(rel, "/")
	depGroup, _, _ := strings.Cut(dep, "/")
	if group != depGroup {
		return false
	}
	if strings.HasPrefix(dep, group+"/internal/") {
		return true
	}
	service := servicePath(rel)
	if rel == service || servicePath(dep) != service {
		return false
	}
	return dep == service || strings.HasPrefix(dep, service+"/internal/") || slices.Contains(engineUsers[rel], dep)
}

// servicePath returns the first 2 path elements of rel, such as edit/ocr.
func servicePath(rel string) string {
	parts := strings.SplitN(rel, "/", 3)
	if len(parts) < 2 {
		return rel
	}
	return parts[0] + "/" + parts[1]
}

func layerOf(rel string) (layer, bool) {
	parent := path.Dir(rel)
	switch {
	case rel == "internal/cli" || rel == "cmd/stbox":
		return layerCLI, true
	case rel == "." || strings.HasPrefix(rel, "edit/") || strings.HasPrefix(rel, "inspect/"):
		return layerTop, true
	case rel == "model" || strings.HasPrefix(rel, "model/") || strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "format/internal/") || isContainerInternal(rel):
		return layerBase, true
	case parent == "format":
		return layerFormat, true
	case parent == "container":
		return layerContainer, true
	}
	return 0, false
}

func isContainerInternal(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) >= 4 && parts[0] == "container" && parts[2] == "internal"
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
			name:    "format imports layer 0 and an allowed module",
			rel:     "format/srt",
			imports: []string{modulePath + "/model", modulePath + "/model/markup", modulePath + "/format/internal/textblock", "golang.org/x/text/encoding"},
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
			name:    "base helper imports model and another helper",
			rel:     "internal/parsing",
			imports: []string{modulePath + "/model", modulePath + "/internal/textenc"},
		},
		{
			name:    "model imports model/markup",
			rel:     "model",
			imports: []string{modulePath + "/model/markup"},
		},
		{
			name:    "base helper imports a format",
			rel:     "internal/parsing",
			imports: []string{modulePath + "/format/srt"},
			want:    []string{"internal/parsing imports format/srt, which is not in a lower layer"},
		},
		{
			name:    "itt imports ttml",
			rel:     "format/itt",
			imports: []string{modulePath + "/format/ttml"},
		},
		{
			name:    "hls imports webvtt",
			rel:     "format/hls",
			imports: []string{modulePath + "/format/webvtt"},
		},
		{
			name:    "format imports another format",
			rel:     "format/webvtt",
			imports: []string{modulePath + "/format/srt"},
			want:    []string{"format/webvtt imports format/srt, which is not in a lower layer"},
		},
		{
			name:    "container imports a format",
			rel:     "container/mkv",
			imports: []string{modulePath + "/format/srt"},
		},
		{
			name:    "container imports another container",
			rel:     "container/mkv",
			imports: []string{modulePath + "/container/mp4"},
			want:    []string{"container/mkv imports container/mp4, which is not in a lower layer"},
		},
		{
			name:    "CLI imports everything",
			rel:     "cmd/stbox",
			imports: []string{modulePath, modulePath + "/internal/cli", modulePath + "/inspect/stats", modulePath + "/format/srt"},
		},
		{
			name:    "root imports a service",
			rel:     ".",
			imports: []string{modulePath + "/inspect/stats"},
			want:    []string{". imports inspect/stats, which is not in a lower layer"},
		},
		{
			name:    "service imports the root",
			rel:     "inspect/stats",
			imports: []string{modulePath},
			want:    []string{"inspect/stats imports ., which is not in a lower layer"},
		},
		{
			name:    "service imports another service",
			rel:     "edit/fixing",
			imports: []string{modulePath + "/edit/speakers"},
			want:    []string{"edit/fixing imports edit/speakers, which is not in a lower layer"},
		},
		{
			name:    "edit imports inspect",
			rel:     "edit/resegment",
			imports: []string{modulePath + "/inspect/stats"},
			want:    []string{"edit/resegment imports inspect/stats, which is not in a lower layer"},
		},
		{
			name:    "service imports a group helper",
			rel:     "edit/speakers",
			imports: []string{modulePath + "/edit/internal/speakerlabel"},
		},
		{
			name:    "inspect service imports its group helper",
			rel:     "inspect/stats",
			imports: []string{modulePath + "/inspect/internal/readingspeed"},
		},
		{
			name:    "group helper imports a service",
			rel:     "edit/internal/speakerlabel",
			imports: []string{modulePath + "/edit/speakers"},
			want:    []string{"edit/internal/speakerlabel imports edit/speakers, which is not in a lower layer"},
		},
		{
			name:    "engine imports its service and its HTTP helper",
			rel:     "edit/translation/deepl",
			imports: []string{modulePath + "/edit/translation", modulePath + "/edit/translation/internal/httppost"},
		},
		{
			name:    "engine imports another engine",
			rel:     "edit/ocr/glyph",
			imports: []string{modulePath + "/edit/ocr/tesseract"},
			want:    []string{"edit/ocr/glyph imports edit/ocr/tesseract, which is not in a lower layer"},
		},
		{
			name:    "service imports its engine",
			rel:     "edit/ocr",
			imports: []string{modulePath + "/edit/ocr/glyph"},
			want:    []string{"edit/ocr imports edit/ocr/glyph, which is not in a lower layer"},
		},
		{
			name:    "chooser imports the engines",
			rel:     "edit/ocr/auto",
			imports: []string{modulePath + "/edit/ocr", modulePath + "/edit/ocr/glyph"},
		},
		{
			name:    "test helper imports a service",
			rel:     "internal/translationtest",
			imports: []string{modulePath + "/edit/translation"},
			want:    []string{"internal/translationtest imports edit/translation, which is not in a lower layer"},
		},
		{
			name:    "test module outside tests",
			rel:     "model",
			imports: []string{"github.com/stretchr/testify/assert"},
			want:    []string{"model imports github.com/stretchr/testify/assert, which is not an allowed module"},
		},
		{
			name: "package without a layer",
			rel:  "misc",
			want: []string{"misc is outside the folders of the layer table in CONTRIBUTING.md"},
		},
		{
			name: "format pattern covers one folder only",
			rel:  "format/srt/extra",
			want: []string{"format/srt/extra is outside the folders of the layer table in CONTRIBUTING.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, checkImports(tt.rel, tt.imports))
		})
	}
}
