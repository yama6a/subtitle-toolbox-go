package stbox

import (
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatsFollowTheRowOrder(t *testing.T) {
	t.Parallel()

	want := []Format{
		ASS, CSV, JSON, LRC, MicroDVD, MPSub, SAMI, SBV, SubRip,
		SubViewer, TTML, TSV, PlainText, MPL2, TMPlayer, QuickTimeText, WebVTT,
	}
	assert.Equal(t, want, Formats())
}

func TestEveryFormatConstantHasARow(t *testing.T) {
	t.Parallel()

	consts := formatConsts(t)
	values := slices.Sorted(maps.Values(consts))
	assert.Equal(t, values, slices.Sorted(slices.Values(Formats())))
}

func TestModelFactsListTheRegistryRows(t *testing.T) {
	t.Parallel()

	consts := formatConsts(t)
	names := factsRowNames(t)
	facts := make([]Format, 0, len(names))
	for _, name := range names {
		f, ok := consts[name]
		require.True(t, ok, "the facts row %s names no Format constant", name)
		facts = append(facts, f)
	}
	assert.Equal(t, Formats(), facts)
}

func TestFactsMatchTheFunctions(t *testing.T) {
	t.Parallel()

	for _, row := range registry {
		t.Run(string(row.format), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, row.read != nil, row.format.CanRead(), "CanRead")
			assert.Equal(t, row.write != nil, row.format.CanWrite(), "CanWrite")
			if row.read != nil {
				assert.Equal(t, detected(row.format), row.format.IsAutoDetected(), "IsAutoDetected")
			}
		})
	}
}

func detected(f Format) bool {
	return slices.ContainsFunc(jsonChecks, func(c jsonCheck) bool { return c.format == f }) ||
		slices.ContainsFunc(signatures, func(s signature) bool { return s.format == f })
}

func TestDetectionListsFollowTheOrder(t *testing.T) {
	t.Parallel()

	jsonOrder := []Format{JSON}
	signatureOrder := []Format{
		WebVTT, TTML, SAMI, ASS, MPSub, MicroDVD, QuickTimeText, SubRip, SBV, SubViewer, LRC, MPL2, TMPlayer,
	}
	jsonFormats := make([]Format, 0, len(jsonChecks))
	signatureFormats := make([]Format, 0, len(signatures))
	for _, c := range jsonChecks {
		jsonFormats = append(jsonFormats, c.format)
	}
	for _, s := range signatures {
		signatureFormats = append(signatureFormats, s.format)
	}
	assert.True(t, isSubsequence(jsonFormats, jsonOrder), "JSON checks %v do not follow %v", jsonFormats, jsonOrder)
	assert.True(t, isSubsequence(signatureFormats, signatureOrder),
		"signatures %v do not follow %v", signatureFormats, signatureOrder)
	for _, f := range slices.Concat(jsonFormats, signatureFormats) {
		assert.True(t, f.IsAutoDetected(), "%s is in a detection list but not auto-detected", f)
		assert.True(t, f.CanRead(), "%s is in a detection list but cannot be read", f)
	}
}

func TestIsSubsequence(t *testing.T) {
	t.Parallel()

	order := []Format{WebVTT, TTML, SubRip}
	tests := []struct {
		name string
		list []Format
		want bool
	}{
		{"empty", nil, true},
		{"all", []Format{WebVTT, TTML, SubRip}, true},
		{"gap", []Format{WebVTT, SubRip}, true},
		{"wrong order", []Format{SubRip, WebVTT}, false},
		{"twice", []Format{WebVTT, WebVTT}, false},
		{"not in the order", []Format{ASS}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isSubsequence(tt.list, order))
		})
	}
}

func TestFormatFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path   string
		want   Format
		wantOK bool
	}{
		{"movie.sub", MicroDVD, true},
		{"movie.SUB", MicroDVD, true},
		{"notes.txt", PlainText, true},
		{"clip.xml", TTML, true},
		{"cues.json", JSON, true},
		{"old.ssa", ASS, true},
		{"show.dfxp", TTML, true},
		{"show.smi", SAMI, true},
		{"show.sami", SAMI, true},
		{"show.vtt", WebVTT, true},
		{"movie.en.srt", SubRip, true},
		{"dir.d/movie.srt", SubRip, true},
		{"subtitles/movie.Srt", SubRip, true},
		{"movie", "", false},
		{"dir.srt/movie", "", false},
		{"movie.", "", false},
		{"movie.mkv", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			got, ok := FormatFromPath(tt.path)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatsReturnsACopy(t *testing.T) {
	t.Parallel()

	Formats()[0] = "changed"
	assert.Equal(t, ASS, Formats()[0])
}

// factsRowNames returns the Format constant name of each row of the model facts table, in row order.
func factsRowNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, f := range parseDir(t, modelDir) {
		for _, decl := range f.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.ValueSpec)
				if !ok || len(s.Names) != 1 || s.Names[0].Name != "formatFacts" {
					continue
				}
				table, ok := s.Values[0].(*ast.CompositeLit)
				require.True(t, ok)
				for _, elt := range table.Elts {
					names = append(names, rowFormat(t, elt))
				}
			}
		}
	}
	require.NotEmpty(t, names, "package model has no formatFacts table")
	return names
}

func rowFormat(t *testing.T, elt ast.Expr) string {
	t.Helper()

	row, ok := elt.(*ast.CompositeLit)
	require.True(t, ok)
	for _, field := range row.Elts {
		kv, ok := field.(*ast.KeyValueExpr)
		require.True(t, ok, "a facts row has a field without a key")
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "format" {
			value, ok := kv.Value.(*ast.Ident)
			require.True(t, ok, "a facts row has a format that is no constant name")
			return value.Name
		}
	}
	require.Fail(t, "a facts row has no format field")
	return ""
}

func isSubsequence(list, order []Format) bool {
	i := 0
	for _, f := range list {
		j := slices.Index(order[i:], f)
		if j < 0 {
			return false
		}
		i += j + 1
	}
	return true
}
