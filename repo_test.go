package stbox_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mainModule  = "github.com/yama6a/subtitle-toolbox-go"
	sourcesFile = "SOURCES.md"
	testdataDir = "testdata"
	fuzzDir     = "fuzz"
	noticesFile = "THIRD_PARTY_NOTICES.md"
	goSection   = "Go"
)

func TestEveryFixtureHasASourcesRow(t *testing.T) {
	t.Parallel()

	missing, err := uncredited(os.DirFS("."))
	require.NoError(t, err)
	assert.Empty(t, missing, "Add a row for these files to a SOURCES.md in their folder or a parent folder.")
}

func TestEverySourcesRowNamesAFile(t *testing.T) {
	t.Parallel()

	unmatched, err := unmatchedRows(os.DirFS("."))
	require.NoError(t, err)
	assert.Empty(t, unmatched, "These SOURCES.md rows name no file.")
}

func TestEveryLinkedModuleHasANotice(t *testing.T) {
	t.Parallel()

	modules, err := linkedModules(t.Context())
	require.NoError(t, err)
	notices, err := os.ReadFile(noticesFile)
	require.NoError(t, err)

	headings := map[string]bool{}
	for line := range strings.Lines(string(notices)) {
		headings[strings.TrimRight(line, "\r\n")] = true
	}
	for _, module := range append([]string{goSection}, modules...) {
		assert.True(t, headings["## "+module], "%s has no section %q for module %s", noticesFile, "## "+module, module)
	}
}

func TestUncredited(t *testing.T) {
	t.Parallel()

	file := func(content string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(content)} }
	tests := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{
			name: "file named in its own folder",
			files: fstest.MapFS{
				"testdata/srt/a.srt":      file("x"),
				"testdata/srt/SOURCES.md": file("| File | Source | License |\n|:--- |:--- |:--- |\n| `a.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "file named in a parent folder by its relative path",
			files: fstest.MapFS{
				"testdata/srt/real/a.srt": file("x"),
				"testdata/srt/SOURCES.md": file("| `real/a.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "file named in the testdata folder",
			files: fstest.MapFS{
				"testdata/srt/real/a.srt": file("x"),
				"testdata/SOURCES.md":     file("| `srt/real/a.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "file named by a glob",
			files: fstest.MapFS{
				"testdata/expected/a.to.srt": file("x"),
				"testdata/expected/b.to.vtt": file("x"),
				"testdata/SOURCES.md":        file("| `expected/*.to.*` | generated | MIT |\n"),
			},
		},
		{
			name: "files named by a comma list",
			files: fstest.MapFS{
				"testdata/a.srt":      file("x"),
				"testdata/b.srt":      file("x"),
				"testdata/c.srt":      file("x"),
				"testdata/SOURCES.md": file("| `a.srt`, `b.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/c.srt"},
		},
		{
			name: "glob does not cross a folder",
			files: fstest.MapFS{
				"testdata/expected/deep/a.srt": file("x"),
				"testdata/SOURCES.md":          file("| `expected/*` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/expected/deep/a.srt"},
		},
		{
			name: "file with no row",
			files: fstest.MapFS{
				"testdata/srt/a.srt":      file("x"),
				"testdata/srt/b.srt":      file("x"),
				"testdata/srt/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/srt/b.srt"},
		},
		{
			name: "row in a sibling folder does not count",
			files: fstest.MapFS{
				"testdata/srt/a.srt":      file("x"),
				"testdata/vtt/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/srt/a.srt"},
		},
		{
			name: "fuzz corpus is exempt",
			files: fstest.MapFS{
				"testdata/fuzz/FuzzParse/0123456789abcdef": file("x"),
				"format/srt/testdata/fuzz/FuzzRead/seed":   file("x"),
			},
		},
		{
			name: "a folder named fuzz below another folder is not exempt",
			files: fstest.MapFS{
				"testdata/srt/fuzz/a.srt": file("x"),
			},
			want: []string{"testdata/srt/fuzz/a.srt"},
		},
		{
			name: "SOURCES.md needs no row",
			files: fstest.MapFS{
				"testdata/SOURCES.md": file(""),
			},
		},
		{
			name: "files outside a testdata folder are ignored",
			files: fstest.MapFS{
				"format/srt/reader.go": file("x"),
				"docs/a.srt":           file("x"),
			},
		},
		{
			name: "testdata folder below a package folder",
			files: fstest.MapFS{
				"format/srt/testdata/a.srt":      file("x"),
				"format/srt/testdata/b.srt":      file("x"),
				"format/srt/testdata/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"format/srt/testdata/b.srt"},
		},
		{
			name: "a SOURCES.md above the testdata folder does not count",
			files: fstest.MapFS{
				"format/srt/testdata/a.srt": file("x"),
				"format/srt/SOURCES.md":     file("| `testdata/a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"format/srt/testdata/a.srt"},
		},
		{
			name: "missing files are sorted",
			files: fstest.MapFS{
				"testdata/z.srt": file("x"),
				"testdata/a.srt": file("x"),
				"testdata/m.srt": file("x"),
			},
			want: []string{"testdata/a.srt", "testdata/m.srt", "testdata/z.srt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := uncredited(tt.files)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUnmatchedRows(t *testing.T) {
	t.Parallel()

	file := func(content string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(content)} }
	tests := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{
			name: "row names a file",
			files: fstest.MapFS{
				"testdata/a.srt":      file("x"),
				"testdata/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "row names nothing",
			files: fstest.MapFS{
				"testdata/a.srt":      file("x"),
				"testdata/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n| `gone.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/SOURCES.md: gone.srt"},
		},
		{
			name: "glob matches a file in a subfolder",
			files: fstest.MapFS{
				"testdata/expected/a.to.srt": file("x"),
				"testdata/SOURCES.md":        file("| `expected/*.to.*` | generated | MIT |\n"),
			},
		},
		{
			name: "glob matches nothing",
			files: fstest.MapFS{
				"testdata/expected/a.srt": file("x"),
				"testdata/SOURCES.md":     file("| `expected/*.to.*` | generated | MIT |\n"),
			},
			want: []string{"testdata/SOURCES.md: expected/*.to.*"},
		},
		{
			name: "one missing name in a comma list",
			files: fstest.MapFS{
				"testdata/a.srt":      file("x"),
				"testdata/SOURCES.md": file("| `a.srt`, `b.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/SOURCES.md: b.srt"},
		},
		{
			name: "header and separator rows are skipped",
			files: fstest.MapFS{
				"testdata/a.srt":      file("x"),
				"testdata/SOURCES.md": file("| File | Source | License |\n|:--- |:--- |:--- |\n| `a.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "a name relative to the SOURCES.md folder",
			files: fstest.MapFS{
				"testdata/srt/a.srt":      file("x"),
				"testdata/vtt/SOURCES.md": file("| `a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/vtt/SOURCES.md: a.srt"},
		},
		{
			name: "a SOURCES.md outside a testdata folder is ignored",
			files: fstest.MapFS{
				"docs/SOURCES.md": file("| `gone.srt` | somewhere | MIT |\n"),
			},
		},
		{
			name: "a name inside a nested testdata folder",
			files: fstest.MapFS{
				"testdata/x/testdata/a.srt": file("x"),
				"testdata/SOURCES.md":       file("| `x/testdata/a.srt` | somewhere | MIT |\n"),
			},
			want: []string{"testdata/SOURCES.md: x/testdata/a.srt"},
		},
		{
			name: "rows are sorted by file, then by order",
			files: fstest.MapFS{
				"testdata/z/SOURCES.md": file("| `b` | s | MIT |\n| `a` | s | MIT |\n"),
				"testdata/a/SOURCES.md": file("| `c` | s | MIT |\n"),
			},
			want: []string{"testdata/a/SOURCES.md: c", "testdata/z/SOURCES.md: a", "testdata/z/SOURCES.md: b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := unmatchedRows(tt.files)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSourcesNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"empty file", "", nil},
		{"header row", "| File | Source | License |\n", nil},
		{"separator row", "|:--- |:--- |:--- |\n|---|---|---|\n| :-: | x | y |\n", nil},
		{"plain name", "| a.srt | s | MIT |\n", []string{"a.srt"}},
		{"name in backticks", "| `a.srt` | s | MIT |\n", []string{"a.srt"}},
		{"comma list", "| `a.srt`, `b/*.vtt` ,c.sub | s | MIT |\n", []string{"a.srt", "b/*.vtt", "c.sub"}},
		{"line without a leading bar", "text | a.srt | MIT |\n", nil},
		{"line with one bar", "| a.srt\n", nil},
		{"empty first cell", "|  | s | MIT |\n", nil},
		{"text around the table", "# Heading\n\nSome text.\n\n| `a.srt` | s | MIT |\n\n- list\n", []string{"a.srt"}},
		{"windows line ends", "| File | Source |\r\n| `a.srt` | s |\r\n", []string{"a.srt"}},
		{
			name:    "full table",
			content: "| File | Source | License |\n|:--- |:--- |:--- |\n| `a.srt` | x | MIT |\n| `b.srt`, `c.srt` | y | ISC |\n",
			want:    []string{"a.srt", "b.srt", "c.srt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sourcesNames([]byte(tt.content)))
		})
	}
}

// uncredited returns the files under any testdata folder of fsys that no SOURCES.md row names.
func uncredited(fsys fs.FS) ([]string, error) {
	var missing []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || d.Name() == sourcesFile {
			return nil
		}
		folder, ok := testdataFolder(p)
		if !ok {
			return nil
		}
		if isFuzzCorpus(p, folder) {
			return nil
		}
		credited, err := isCredited(fsys, p, folder)
		if err != nil {
			return err
		}
		if !credited {
			missing = append(missing, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk testdata folders: %w", err)
	}
	slices.Sort(missing)
	return missing, nil
}

// unmatchedRows returns "<SOURCES.md path>: <name>" for each row name that matches no file.
func unmatchedRows(fsys fs.FS) ([]string, error) {
	var unmatched []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != sourcesFile {
			return nil
		}
		if _, ok := testdataFolder(p); !ok {
			return nil
		}
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err //nolint:wrapcheck // the fs.PathError names the file and unmatchedRows wraps once

		}
		folder := path.Dir(p)
		files, err := filesBelow(fsys, folder)
		if err != nil {
			return err
		}
		for _, name := range sourcesNames(content) {
			if !slices.ContainsFunc(files, func(file string) bool { return nameMatches(name, file) }) {
				unmatched = append(unmatched, p+": "+name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk testdata folders: %w", err)
	}
	slices.Sort(unmatched)
	return unmatched, nil
}

// sourcesNames returns the names in the first cell of each table row of one SOURCES.md.
func sourcesNames(content []byte) []string {
	var names []string
	for line := range strings.Lines(string(content)) {
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cell, _, found := strings.Cut(line[1:], "|")
		cell = strings.TrimSpace(cell)
		if !found || cell == "File" || strings.Trim(cell, ":- ") == "" {
			continue
		}
		for part := range strings.SplitSeq(cell, ",") {
			if name := strings.Trim(part, " `"); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// linkedModules returns the module paths that `go list -deps ./cmd/stbox` reports, without the main module.
func linkedModules(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}{{end}}", "./cmd/stbox").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("go list: %w: %s", err, bytes.TrimSpace(exitErr.Stderr))
		}
		return nil, fmt.Errorf("go list: %w", err)
	}
	var modules []string
	for line := range strings.Lines(string(out)) {
		if module := strings.TrimSpace(line); module != "" && module != mainModule {
			modules = append(modules, module)
		}
	}
	slices.Sort(modules)
	return slices.Compact(modules), nil
}

func testdataFolder(p string) (string, bool) {
	for folder := path.Dir(p); folder != "."; folder = path.Dir(folder) {
		if path.Base(folder) == testdataDir {
			return folder, true
		}
	}
	return "", false
}

func isFuzzCorpus(p, folder string) bool {
	return strings.HasPrefix(p, folder+"/"+fuzzDir+"/")
}

func isCredited(fsys fs.FS, p, folder string) (bool, error) {
	for dir := path.Dir(p); ; dir = path.Dir(dir) {
		content, err := fs.ReadFile(fsys, path.Join(dir, sourcesFile))
		switch {
		case err == nil:
			relative := strings.TrimPrefix(p, dir+"/")
			for _, name := range sourcesNames(content) {
				if nameMatches(name, relative) {
					return true, nil
				}
			}
		case !errors.Is(err, fs.ErrNotExist):
			return false, err //nolint:wrapcheck // the fs.PathError names the file and uncredited wraps once

		}
		if dir == folder {
			return false, nil
		}
	}
}

func nameMatches(name, relative string) bool {
	if name == relative {
		return true
	}
	ok, err := path.Match(name, relative)
	return err == nil && ok
}

func filesBelow(fsys fs.FS, folder string) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != folder && d.Name() == testdataDir {
			return fs.SkipDir
		}
		if d.Type().IsRegular() && d.Name() != sourcesFile {
			files = append(files, strings.TrimPrefix(p, folder+"/"))
		}
		return nil
	})
	return files, err //nolint:wrapcheck // the fs.PathError names the file and unmatchedRows wraps once
}
