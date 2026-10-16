package cli

import (
	"testing"

	"github.com/dlclark/regexp2/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"angle brackets", `<a\>b>`, `a\>b`, false, "", true},
		{"escaped slash", `/a\/b/`, `a\/b`, false, "", true},
		{"escaped plus", `+a\+b+`, `a\+b`, false, "", true},
		{"escaped parenthesis", `(a\(b)`, `a\(b`, false, "", true},
		{"escaped comma", `,a{1\,2}b,`, `a{1\,2}b`, false, "", true},
		{"escaped exclamation mark", `!a(?\!b)!`, `a(?\!b)`, false, "", true},
		{"escaped underscore delimiter", `_a\_b_`, "a_b", false, "", true},
		{"other escapes stay", `/\d\//`, `\d\/`, false, "", true},
		{"tilde", "~colou?r~", "colou?r", false, "", true},
		{"flag i", "/col(ou)r/i", "col(ou)r", true, "", true},
		{"flags m s x", "/a/xsm", "(?xsm)a", false, "", true},
		{"repeated flags", "/a/mm", "(?m)a", false, "", true},
		{"flag u dropped", "/a/u", "a", false, "", true},
		{"space LF and CR in the flags", "/a/i x\n\r", "(?x)a", true, "", true},
		{"flag U", "/a/U", "", false, "U", false},
		{"flag after a good one", "/a/iD", "", false, "D", false},
		{"tab flag", "/a/\t", "", false, "\t", false},
		{"no closing delimiter", "/abc", "", false, "", false},
		{"escaped closing delimiter only", `/abc\/`, "", false, "", false},
		{"unclosed nesting", "{a{2}", "", false, "", false},
		{"letter delimiter", "abca", "", false, "", false},
		{"digit delimiter", "1a1", "", false, "", false},
		{"backslash delimiter", `\a\`, "", false, "", false},
		{"empty", "", "", false, "", false},
		{"only white space", "  ", "", false, "", false},
		{"empty pattern", "//", "", false, "", false},
		{"empty pattern with flag m", "//m", "", false, "", false},
		{"empty pattern with flag s", "//s", "", false, "", false},
		{"empty pattern with flag x", "//x", "", false, "", false},
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

// The PHP column holds what preg_replace of PHP 8.5 writes for each TO when /(a)/ matches "a".
func TestConvertReplacementGivesThePHPResult(t *testing.T) {
	t.Parallel()

	tests := []struct{ to, want, php string }{
		{`\1`, "${1}", "a"},
		{`[\0]`, "[${0}]", "[a]"},
		{`\11`, "", ""},
		{`\123`, "3", "3"},
		{`$1`, "${1}", "a"},
		{`$1a`, "${1}a", "aa"},
		{`$11`, "", ""},
		{`${1}x`, "${1}x", "ax"},
		{`${12`, "$${12", "${12"},
		{`$$`, "$$$$", "$$"},
		{`$_`, "$$_", "$_"},
		{`$&`, "$$&", "$&"},
		{`${name}`, "$${name}", "${name}"},
		{`$`, "$$", "$"},
		{`\\1`, `\1`, `\1`},
		{`\\\1`, `\${1}`, `\a`},
		{`\$1`, "$$1", "$1"},
		{`\x`, `\x`, `\x`},
		{`\`, `\`, `\`},
		{"", "", ""},
	}
	re := regexp2.MustCompile("(a)", regexp2.None)
	for _, tt := range tests {
		t.Run(tt.to, func(t *testing.T) {
			t.Parallel()

			got := convertReplacement(tt.to, func(n int) bool { return n <= 1 })
			assert.Equal(t, tt.want, got)
			out, err := re.Replace("a", got, -1, -1)
			require.NoError(t, err)
			assert.Equal(t, tt.php, out)
		})
	}
}

func TestRegexReplacementRejectsWhatRegexp2Lacks(t *testing.T) {
	t.Parallel()

	for _, from := range []string{`/(/`, `/a\Kb/`, `/\h+/`, `/\o{101}/`, `/(?|a|b)/`, `/(*SKIP)a/`, "//", "//m", `!a(?\!b)!`} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			_, _, err := regexReplacement(from)
			assert.Equal(t, usage("The option --replace has the invalid regular expression "+quote(from)+"."), err)
		})
	}
	_, _, err := regexReplacement("/a/U")
	assert.Equal(t, usage(`The option --replace has the invalid regular expression "/a/U". The flag U is not supported.`), err)
	for _, from := range []string{`/[[:alpha:]]/`, `/\R/`, `/a++/`, `_a\_b_`, `/(?<=a)b/i`} {
		_, _, err := regexReplacement(from)
		require.NoError(t, err, from)
	}
	for _, c := range "!\"#$%&'()*+,-./:;<=>?@[]^`{|}~" {
		_, _, err := regexReplacement(`/\` + string(c) + `/`)
		require.NoError(t, err, string(c))
	}
	_, hasGroup, err := regexReplacement(`/(a)(?<b>c)/`)
	require.NoError(t, err)
	assert.Equal(t, []bool{true, true, true, false}, []bool{hasGroup(0), hasGroup(1), hasGroup(2), hasGroup(3)})
}
