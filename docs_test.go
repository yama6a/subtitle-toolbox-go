package stbox_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

var (
	fencePattern   = regexp.MustCompile("(?s)```.*?```")
	codeSpan       = regexp.MustCompile("`[^`\n]*`")
	linkPattern    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	headingPattern = regexp.MustCompile(`(?m)^#{1,6} +(.+?) *$`)
	markerPattern  = regexp.MustCompile(`^<!-- example: (\S+) -->$`)
)

func markdownFiles(t *testing.T) []string {
	t.Helper()
	files := []string{"README.md"}
	err := filepath.WalkDir("docs", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, filepath.ToSlash(p))
		}
		return nil
	})
	require.NoError(t, err)
	slices.Sort(files)
	return files
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(b)
}

// anchors returns the heading anchors that GitHub makes. A repeated heading gets the suffix -1, -2 and so on.
func anchors(text string) map[string]bool {
	out := map[string]bool{}
	seen := map[string]int{}
	for _, m := range headingPattern.FindAllStringSubmatch(fencePattern.ReplaceAllString(text, ""), -1) {
		var b strings.Builder
		for _, r := range strings.ToLower(strings.ReplaceAll(m[1], "`", "")) {
			switch {
			case r == ' ' || r == '-':
				b.WriteByte('-')
			case r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
				b.WriteRune(r)
			}
		}
		slug := b.String()
		if n := seen[slug]; n > 0 {
			out[slug+"-"+strconv.Itoa(n)] = true
		} else {
			out[slug] = true
		}
		seen[slug]++
	}
	return out
}

func TestDocsAnchors(t *testing.T) {
	t.Parallel()
	got := anchors("# Formats\n## Options\n## Read `x`\n## Options\n## Options\n")
	assert.Equal(t, map[string]bool{"formats": true, "options": true, "options-1": true, "options-2": true, "read-x": true}, got)
}

func TestDocsLinks(t *testing.T) {
	t.Parallel()
	for _, file := range markdownFiles(t) {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			text := codeSpan.ReplaceAllString(fencePattern.ReplaceAllString(readFile(t, file), ""), "")
			for _, m := range linkPattern.FindAllStringSubmatch(text, -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				name, fragment, _ := strings.Cut(target, "#")
				name = path.Join(path.Dir(file), name)
				if target[0] == '#' {
					name = file
				}
				info, err := os.Stat(filepath.FromSlash(name))
				require.NoError(t, err, "%s links to %s", file, target)
				if fragment != "" && !info.IsDir() && strings.HasSuffix(name, ".md") {
					assert.True(t, anchors(readFile(t, filepath.FromSlash(name)))[fragment],
						"%s links to %s, which has no such heading", file, target)
				}
			}
		})
	}
}

func TestDocsIndex(t *testing.T) {
	t.Parallel()
	index := readFile(t, "docs/README.md")
	var rows []string
	for _, line := range strings.Split(index, "\n") {
		if strings.HasPrefix(line, "|") {
			rows = append(rows, line)
		}
	}
	for _, file := range markdownFiles(t) {
		page, ok := strings.CutPrefix(file, "docs/")
		if !ok || page == "README.md" {
			continue
		}
		assert.True(t, slices.ContainsFunc(rows, func(row string) bool {
			return strings.Contains(row, "]("+page+")")
		}), "docs/README.md has no table row that links to %s", page)
	}
}

func TestReadmeSections(t *testing.T) {
	t.Parallel()
	readme := readFile(t, "README.md")
	got := regexp.MustCompile(`(?m)^#{1,2} .+$`).FindAllString(fencePattern.ReplaceAllString(readme, ""), -1)
	assert.Equal(t, []string{
		"# subtitle-toolbox-go",
		"## Install",
		"## Usage",
		"## Requirements",
		"## Formats",
		"## Command line tool",
		"## API",
		"## Design",
		"## Compatibility",
		"## Documentation",
		"## Development",
		"## Releases",
		"## Contributing",
		"## License",
		"## Credits",
	}, got)
	assert.NotContains(t, readme, "shields.io", "the README has no badges")
}

func TestDocsASCII(t *testing.T) {
	t.Parallel()
	for _, file := range markdownFiles(t) {
		for i, line := range strings.Split(readFile(t, file), "\n") {
			for _, r := range line {
				if r > 127 {
					assert.Fail(t, "non-ASCII character", "%s line %d holds %q", file, i+1, r)
					break
				}
			}
		}
	}
}

func TestReadmeFormats(t *testing.T) {
	t.Parallel()
	type row struct{ name, extensions, read, write string }
	var want []row
	for _, f := range stbox.Formats() {
		if !f.CanRead() && !f.CanWrite() {
			continue
		}
		var exts []string
		for _, e := range f.Extensions() {
			exts = append(exts, "`."+e+"`")
		}
		want = append(want, row{"`" + string(f) + "`", strings.Join(exts, ", "), yesNo(f.CanRead()), yesNo(f.CanWrite())})
	}

	section := readFile(t, "README.md")
	_, section, ok := strings.Cut(section, "\n## Formats\n")
	require.True(t, ok)
	section, _, _ = strings.Cut(section, "\n## ")
	var got []row
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if !strings.HasPrefix(line, "|") || len(cells) != 7 || strings.TrimSpace(cells[0]) == "Format" || strings.HasPrefix(strings.TrimSpace(cells[0]), ":") {
			continue
		}
		got = append(got, row{strings.TrimSpace(cells[2]), strings.TrimSpace(cells[3]), strings.TrimSpace(cells[4]), strings.TrimSpace(cells[5])})
	}
	assert.Equal(t, want, got)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

type exampleFunc struct {
	dir  string
	body string
}

func examples(t *testing.T) map[string][]exampleFunc {
	t.Helper()
	out := map[string][]exampleFunc{}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Example") {
				continue
			}
			var comments []*ast.CommentGroup
			for _, c := range file.Comments {
				if c.Pos() >= fn.Body.Pos() && c.End() <= fn.Body.End() {
					comments = append(comments, c)
				}
			}
			var buf bytes.Buffer
			cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
			if err := cfg.Fprint(&buf, fset, &printer.CommentedNode{Node: fn.Body, Comments: comments}); err != nil {
				return fmt.Errorf("print %s in %s: %w", fn.Name.Name, p, err)
			}
			out[fn.Name.Name] = append(out[fn.Name.Name], exampleFunc{dir: path.Dir(filepath.ToSlash(p)), body: unbrace(buf.String())})
		}
		return nil
	})
	require.NoError(t, err)
	return out
}

func unbrace(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	lines = lines[1 : len(lines)-1]
	for i, line := range lines {
		lines[i] = strings.TrimRight(strings.TrimPrefix(line, "\t"), " \t")
	}
	return strings.Join(lines, "\n")
}

// TestDocsExamples checks that each go block is the body of the Example function that the line above it names.
// The marker is "<!-- example: Name -->", or "<!-- example: dir/Name -->" when several packages have a function Name.
func TestDocsExamples(t *testing.T) {
	t.Parallel()
	all := examples(t)
	for _, file := range markdownFiles(t) {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			lines := strings.Split(readFile(t, file), "\n")
			for i := 0; i < len(lines); i++ {
				if !strings.HasPrefix(lines[i], "```go") {
					continue
				}
				start := i + 1
				end := slices.IndexFunc(lines[start:], func(l string) bool { return strings.HasPrefix(l, "```") })
				require.GreaterOrEqual(t, end, 0, "%s: the go block at line %d has no closing fence", file, start)
				i = start + end
				block := strings.Join(lines[start:i], "\n")
				at := file + " line " + strconv.Itoa(start)
				if start < 2 || !markerPattern.MatchString(lines[start-2]) {
					assert.Fail(t, "go block without a marker", "%s: put <!-- example: Name --> on the line above the fence", at)
					continue
				}
				ref := markerPattern.FindStringSubmatch(lines[start-2])[1]
				dir, name := path.Split(ref)
				dir = strings.TrimSuffix(dir, "/")
				var matches []exampleFunc
				for _, f := range all[name] {
					if dir == "" || f.dir == dir {
						matches = append(matches, f)
					}
				}
				if !assert.Len(t, matches, 1, "%s: %s must name exactly one Example function, add a dir/ prefix when several packages have it", at, ref) {
					continue
				}
				assert.Equal(t, matches[0].body, strings.TrimRight(block, " \t\n"), "%s differs from %s", at, ref)
			}
		})
	}
}
