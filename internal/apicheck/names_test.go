// Package apicheck tests the exported API of the module against the API rules in CONTRIBUTING.md.
package apicheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const moduleRoot = "../.."

const (
	ruleLookupPrefix = "N5"
	ruleBannedWord   = "N4"
	ruleOptionDoc    = "N20"
)

var ruleText = map[string]string{
	ruleLookupPrefix: "starts with Get or Find",
	ruleBannedWord:   "holds Colour, Fps or a time unit",
	ruleOptionDoc:    "is an option field without a doc comment",
}

// exceptions maps "<package dir>:<name>:<rule>" to the reason why the name breaks the rule.
// The package dir is relative to the module root, "." for the root package.
var exceptions = map[string]string{
	"format/csv:TimeSeconds:N4":      "names a CSV time format, not the unit of a value",
	"format/csv:TimeMilliseconds:N4": "names a CSV time format, not the unit of a value",
}

var bannedWords = []string{"colour", "fps", "secs", "seconds", "millis"}

type finding struct {
	pos  token.Position
	key  string
	rule string
}

func TestExportedNames(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	packages := map[string][]*ast.File{}
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(moduleRoot, path)
		require.NoError(t, err)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		require.NoError(t, err)
		dir := filepath.ToSlash(filepath.Dir(rel))
		packages[dir] = append(packages[dir], f)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, packages, "no Go file found under %s", moduleRoot)

	used := map[string]bool{}
	for _, dir := range slices.Sorted(maps.Keys(packages)) {
		for _, fd := range checkPackage(fset, dir, packages[dir]) {
			if _, ok := exceptions[fd.key]; ok {
				used[fd.key] = true
				continue
			}
			t.Errorf("%s: %s %s", fd.pos, fd.key, ruleText[fd.rule])
		}
	}
	for key := range exceptions {
		assert.True(t, used[key], "exception %q matches no name, remove it", key)
	}
}

// skipDir leaves out what is not API: internal/ at any depth, cmd/, and the
// directories that the go command ignores.
func skipDir(rel, name string) bool {
	if rel == "." {
		return false
	}
	return rel == "cmd" || name == "internal" || name == "testdata" || name == "vendor" ||
		strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func checkPackage(fset *token.FileSet, dir string, files []*ast.File) []finding {
	options := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok && d.Recv != nil && d.Name.Name == "Validate" {
				options[receiverType(d.Recv.List[0].Type)] = true
			}
		}
	}

	var out []finding
	add := func(pos token.Pos, name, rule string) {
		out = append(out, finding{pos: fset.Position(pos), key: dir + ":" + name + ":" + rule, rule: rule})
	}
	checkName := func(id *ast.Ident, qualified string) {
		if !id.IsExported() {
			return
		}
		if hasLookupPrefix(id.Name) {
			add(id.Pos(), qualified, ruleLookupPrefix)
		}
		if hasBannedWord(id.Name) {
			add(id.Pos(), qualified, ruleBannedWord)
		}
	}

	var checkType func(prefix string, expr ast.Expr, options bool)
	checkType = func(prefix string, expr ast.Expr, options bool) {
		switch t := expr.(type) {
		case *ast.StructType:
			for _, field := range t.Fields.List {
				for _, id := range field.Names {
					if !id.IsExported() {
						continue
					}
					checkName(id, prefix+"."+id.Name)
					if options && strings.TrimSpace(field.Doc.Text()) == "" {
						add(id.Pos(), prefix+"."+id.Name, ruleOptionDoc)
					}
					checkType(prefix+"."+id.Name, field.Type, options)
				}
			}
		case *ast.InterfaceType:
			for _, method := range t.Methods.List {
				for _, id := range method.Names {
					checkName(id, prefix+"."+id.Name)
				}
			}
		case *ast.StarExpr:
			checkType(prefix, t.X, options)
		case *ast.ArrayType:
			checkType(prefix, t.Elt, options)
		case *ast.MapType:
			checkType(prefix, t.Key, options)
			checkType(prefix, t.Value, options)
		case *ast.ChanType:
			checkType(prefix, t.Value, options)
		}
	}

	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					checkName(d.Name, d.Name.Name)
					continue
				}
				recv := receiverType(d.Recv.List[0].Type)
				if ast.IsExported(recv) {
					checkName(d.Name, recv+"."+d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						checkName(s.Name, s.Name.Name)
						if s.Assign == token.NoPos {
							isOptions := strings.HasSuffix(s.Name.Name, "Options") || options[s.Name.Name]
							checkType(s.Name.Name, s.Type, isOptions)
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							checkName(id, id.Name)
						}
					}
				}
			}
		}
	}
	return out
}

func hasLookupPrefix(name string) bool {
	for _, prefix := range []string{"Get", "Find"} {
		rest, ok := strings.CutPrefix(name, prefix)
		if ok && rest != "" && (unicode.IsUpper(rune(rest[0])) || unicode.IsDigit(rune(rest[0])) || rest[0] == '_') {
			return true
		}
	}
	return false
}

// hasBannedWord counts a word only where a word of the name starts, so that
// "ParseCSV" does not hold "secs".
func hasBannedWord(name string) bool {
	for i, r := range name {
		if i > 0 && !unicode.IsUpper(r) {
			continue
		}
		rest := strings.ToLower(name[i:])
		for _, w := range bannedWords {
			if strings.HasPrefix(rest, w) {
				return true
			}
		}
	}
	return false
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func TestCheckPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "clean declarations",
			src: `
type Format string
const SubRip Format = "srt"
var ErrInvalidArgument error
func FormatFromPath(path string) (Format, bool) { return "", false }
type Subtitle struct{ cues []int }
func (s *Subtitle) CueIndexAt(t int) (int, bool) { return 0, false }
func ParseCSV() {}
func UseCSV() {}`,
		},
		{
			name: "Get and Find prefixes",
			src: `
func GetFormat() {}
func FindFormat() {}
func Get2() {}
type S struct{}
func (s *S) GetCode() int { return 0 }`,
			want: []string{
				"x:GetFormat:N5",
				"x:FindFormat:N5",
				"x:Get2:N5",
				"x:S.GetCode:N5",
			},
		},
		{
			name: "prefix needs a word boundary",
			src: `
func Getaway() {}
type Findings struct{}
func getFormat() {}
type Data struct{}
func (d *Data) Get(key string) (any, bool) { return nil, false }
func (d Data) Find() {}`,
		},
		{
			name: "generic receiver",
			src: `
type List[T any] struct{}
func (l *List[T]) GetFirst() {}
type Pair[K, V any] struct{}
func (p Pair[K, V]) FindKey() {}`,
			want: []string{
				"x:List.GetFirst:N5",
				"x:Pair.FindKey:N5",
			},
		},
		{
			name: "units and spellings in every kind of name",
			src: `
const DefaultFps = 25
var Colour int
type DurationMillis int
const Millisecond = 1
type Cue struct {
	StartSecs int
	FPS       float64
	inSeconds int
}
type Engine interface {
	TimeoutSeconds() int
}
func (c Cue) EndMillis() int { return 0 }`,
			want: []string{
				"x:DefaultFps:N4",
				"x:Colour:N4",
				"x:DurationMillis:N4",
				"x:Millisecond:N4",
				"x:Cue.StartSecs:N4",
				"x:Cue.FPS:N4",
				"x:Engine.TimeoutSeconds:N4",
				"x:Cue.EndMillis:N4",
			},
		},
		{
			name: "unexported types are not API",
			src: `
type cueJSON struct {
	StartMillis int
}
func (c cueJSON) GetStart() int { return 0 }
type writeOptions struct {
	BOM bool
}
type engine interface {
	TimeoutSeconds() int
}`,
		},
		{
			name: "fields of nested anonymous structs",
			src: `
type Report struct {
	Totals struct {
		WordCount  int
		LengthSecs int
	}
	Lines []struct {
		EndMillis int
	}
	Last *struct {
		FPS int
	}
	ByWord map[string]struct {
		Colour int
	}
}`,
			want: []string{
				"x:Report.Totals.LengthSecs:N4",
				"x:Report.Lines.EndMillis:N4",
				"x:Report.Last.FPS:N4",
				"x:Report.ByWord.Colour:N4",
			},
		},
		{
			name: "option fields need a doc comment",
			src: `
type Common struct{}
type WriteOptions struct {
	// OmitHeader leaves out the header block.
	OmitHeader bool
	BOM        bool
	CRLF       bool // a trailing comment is no doc comment
	Common
	internal int
	// Paragraphs sets the paragraph rules.
	Paragraphs []struct {
		Gap int
	}
}
type Options struct {
	//nolint:revive // a directive alone is no doc comment
	MaxLinesPerCue int
}
type Report struct {
	Count int
}
type ReadOptions = WriteOptions`,
			want: []string{
				"x:WriteOptions.BOM:N20",
				"x:WriteOptions.CRLF:N20",
				"x:WriteOptions.Paragraphs.Gap:N20",
				"x:Options.MaxLinesPerCue:N20",
			},
		},
		{
			name: "a struct with a Validate method is an options struct",
			src: `
type CueLimits struct {
	// MaxLinesPerCue is the most lines a cue may have. Zero means no limit.
	MaxLinesPerCue int
	MaxCharsPerLine int
}
func (l CueLimits) Validate() error { return nil }`,
			want: []string{"x:CueLimits.MaxCharsPerLine:N20"},
		},
		{
			name: "one name can break several rules",
			src: `
type ReadOptions struct {
	GetFps int
}`,
			want: []string{
				"x:ReadOptions.GetFps:N5",
				"x:ReadOptions.GetFps:N4",
				"x:ReadOptions.GetFps:N20",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "x.go", "package x\n"+tt.src, parser.ParseComments)
			require.NoError(t, err)

			var got []string
			for _, fd := range checkPackage(fset, "x", []*ast.File{f}) {
				got = append(got, fd.key)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
