package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertPCRE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		from       string
		pattern    string
		ignoreCase bool
		badFlag    string
		ok         bool
	}{
		{"slashes", `/\.{4,}/`, `\.{4,}`, false, "", true},
		{"leading white space", " \t/a/", "a", false, "", true},
		{"hash delimiter", "#a/b#", "a/b", false, "", true},
		{"brace delimiter", "{x+}", "x+", false, "", true},
		{"nested braces", "{a{2}}", "a{2}", false, "", true},
		{"parentheses nest", "((a)|b)", "(a)|b", false, "", true},
		{"square brackets", "[a]", "a", false, "", true},
		{"angle brackets", "<a\\>b>", "a>b", false, "", true},
		{"escaped delimiter", `/a\/b/`, "a/b", false, "", true},
		{"escaped meta delimiter stays", `+a\+b+`, `a\+b`, false, "", true},
		{"escaped parenthesis stays", `(a\(b)`, `a\(b`, false, "", true},
		{"other escapes stay", `/\d\//`, `\d/`, false, "", true},
		{"tilde", "~colou?r~", "colou?r", false, "", true},
		{"flag i", "/col(ou)r/i", "col(ou)r", true, "", true},
		{"flags m s x", "/a/xsm", "(?xsm)a", false, "", true},
		{"repeated flags", "/a/mm", "(?m)a", false, "", true},
		{"flag u dropped", "/a/u", "a", false, "", true},
		{"flag U", "/a/U", "", false, "U", false},
		{"flag after a good one", "/a/iD", "", false, "D", false},
		{"white space flag", "/a/ ", "", false, " ", false},
		{"no closing delimiter", "/abc", "", false, "", false},
		{"escaped closing delimiter only", `/abc\/`, "", false, "", false},
		{"unclosed nesting", "{a{2}", "", false, "", false},
		{"letter delimiter", "abca", "", false, "", false},
		{"digit delimiter", "1a1", "", false, "", false},
		{"backslash delimiter", `\a\`, "", false, "", false},
		{"empty", "", "", false, "", false},
		{"only white space", "  ", "", false, "", false},
		{"empty pattern", "//", "", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, badFlag, ok := convertPCRE(tt.from)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.badFlag, badFlag)
			assert.Equal(t, pcrePattern{pattern: tt.pattern, ignoreCase: tt.ignoreCase}, p)
		})
	}
}

func TestConvertReplacement(t *testing.T) {
	t.Parallel()

	tests := []struct{ to, want string }{
		{`\1`, "${1}"},
		{`[\0]`, "[${0}]"},
		{`\12`, "${12}"},
		{`\123`, "${12}3"},
		{`$1 ${2} $$`, `$1 ${2} $$`},
		{`\\1`, `\${1}`},
		{`a\b\`, `a\b\`},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.to, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, convertReplacement(tt.to))
		})
	}
}

func TestRegexReplacementRejectsWhatRegexp2Lacks(t *testing.T) {
	t.Parallel()

	for _, from := range []string{`/(/`, `/a\Kb/`, `/\h+/`, `/\o{101}/`, `/(?|a|b)/`, `/(*SKIP)a/`, "//"} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			_, err := regexReplacement(from)
			assert.Equal(t, usage("The option --replace has the invalid regular expression "+quote(from)+"."), err)
		})
	}
	_, err := regexReplacement("/a/U")
	assert.Equal(t, usage(`The option --replace has the invalid regular expression "/a/U". The flag U is not supported.`), err)
	for _, from := range []string{`/[[:alpha:]]/`, `/\R/`, `/a++/`, `_a\_b_`, `/(?<=a)b/i`} {
		_, err := regexReplacement(from)
		assert.NoError(t, err, from)
	}
}
