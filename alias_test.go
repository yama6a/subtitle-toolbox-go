package stbox

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modelDir = "model"

// reexportedTypes names the model types whose constants the root re-exports.
var reexportedTypes = map[string]bool{
	"Format": true, "Alignment": true, "LineEnding": true, "BOM": true, "WarningAction": true,
	"CaseMode": true, "DialogueDashStyle": true,
}

type constDecl struct {
	name  string
	typ   string
	value ast.Expr
}

func TestEveryModelConstantIsReexported(t *testing.T) {
	t.Parallel()

	root := packageValues(t, ".")
	var checked int
	for _, c := range modelConsts(t) {
		if !reexportedTypes[c.typ] {
			continue
		}
		checked++
		assert.Equal(t, "model."+c.name, root[c.name], "the root package has no constant %s = model.%s", c.name, c.name)
	}
	assert.Positive(t, checked)
}

func TestEveryModelSentinelIsReexported(t *testing.T) {
	t.Parallel()

	root := packageValues(t, ".")
	modelValues := packageValues(t, modelDir)
	var checked int
	for name := range modelValues {
		if !strings.HasPrefix(name, "Err") {
			continue
		}
		checked++
		assert.Equal(t, "model."+name, root[name], "the root package has no variable %s = model.%s", name, name)
	}
	assert.Positive(t, checked)
}

// parseDir parses the non-test Go files of dir.
func parseDir(t *testing.T, dir string) []*ast.File {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		files = append(files, f)
	}
	require.NotEmpty(t, files, "no Go file in %s", dir)
	return files
}

// modelConsts returns the exported constants of package model in source order.
// A constant without a type and a value takes both from the spec above it, as in Go.
func modelConsts(t *testing.T) []constDecl {
	t.Helper()

	var out []constDecl
	for _, f := range parseDir(t, modelDir) {
		for _, decl := range f.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.CONST {
				continue
			}
			var typ string
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.ValueSpec)
				require.True(t, ok)
				if s.Type != nil {
					typ = typeName(s.Type)
				} else if len(s.Values) > 0 {
					typ = ""
				}
				for i, id := range s.Names {
					if !id.IsExported() {
						continue
					}
					c := constDecl{name: id.Name, typ: typ}
					if i < len(s.Values) {
						c.value = s.Values[i]
					}
					out = append(out, c)
				}
			}
		}
	}
	return out
}

// packageValues maps each exported constant and variable of dir that is set to a
// selector, such as SubRip = model.SubRip, to the text of that selector.
func packageValues(t *testing.T, dir string) map[string]string {
	t.Helper()

	out := map[string]string{}
	for _, f := range parseDir(t, dir) {
		for _, decl := range f.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || (d.Tok != token.CONST && d.Tok != token.VAR) {
				continue
			}
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.ValueSpec)
				require.True(t, ok)
				for i, id := range s.Names {
					if !id.IsExported() {
						continue
					}
					out[id.Name] = ""
					if i >= len(s.Values) {
						continue
					}
					if sel, ok := s.Values[i].(*ast.SelectorExpr); ok {
						if pkg, ok := sel.X.(*ast.Ident); ok {
							out[id.Name] = pkg.Name + "." + sel.Sel.Name
						}
					}
				}
			}
		}
	}
	return out
}

func typeName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// formatConsts maps the name of each Format constant of package model to its value.
func formatConsts(t *testing.T) map[string]Format {
	t.Helper()

	out := map[string]Format{}
	for _, c := range modelConsts(t) {
		if c.typ != "Format" {
			continue
		}
		lit, ok := c.value.(*ast.BasicLit)
		require.True(t, ok, "model.%s has no string literal value", c.name)
		value, err := strconv.Unquote(lit.Value)
		require.NoError(t, err)
		out[c.name] = Format(value)
	}
	return out
}
