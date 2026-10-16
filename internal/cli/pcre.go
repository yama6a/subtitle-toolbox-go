package cli

import (
	"regexp"
	"strconv"
	"strings"
)

// pcrePattern is a FROM of --replace-regex after the conversion of convertPCRE.
type pcrePattern struct {
	// pattern is the bare regexp2 pattern, with the flags m, s and x as an inline group.
	pattern    string
	ignoreCase bool
}

// convertPCRE turns a PHP PCRE pattern with delimiters and flags, such as /\.{4,}/i, into a regexp2 pattern.
// ok is false for a pattern that PHP rejects, and for an empty pattern such as //m, which ReplaceText rejects.
// badFlag holds the first flag that regexp2 lacks, or "".
func convertPCRE(from string) (p pcrePattern, badFlag string, ok bool) {
	s := strings.TrimLeft(from, phpSpace)
	if s == "" || isAlnum(s[0]) || s[0] == '\\' {
		return pcrePattern{}, "", false
	}
	body, flags, ok := splitDelimited(s)
	if !ok || body == "" {
		return pcrePattern{}, "", false
	}
	inline, ignoreCase, badFlag := readFlags(flags)
	if badFlag != "" {
		return pcrePattern{}, badFlag, false
	}
	if inline != "" {
		body = "(?" + inline + ")" + body
	}
	return pcrePattern{pattern: body, ignoreCase: ignoreCase}, "", true
}

// readFlags returns the flags m, s and x for an inline group, whether i is set, and the first other flag. PHP
// skips u, a space, LF and CR.
func readFlags(flags string) (inline string, ignoreCase bool, badFlag string) {
	for _, f := range flags {
		switch f {
		case 'i':
			ignoreCase = true
		case 'm', 's', 'x':
			if !strings.ContainsRune(inline, f) {
				inline += string(f)
			}
		case 'u', ' ', '\n', '\r':
		default:
			return "", false, string(f)
		}
	}
	return inline, ignoreCase, ""
}

// splitDelimited returns the pattern between the delimiter at s[0] and its closing delimiter, and the flags after
// it. ok is false without a closing delimiter.
func splitDelimited(s string) (body, flags string, ok bool) {
	open := s[0]
	closing, nests := closingDelimiter(open)
	var b strings.Builder
	depth := 1
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			i++
			b.WriteString(unescapeEscape(s[i]))
			continue
		case c == closing:
			if depth--; depth == 0 {
				return b.String(), s[i+1:], true
			}
		case nests && c == open:
			depth++
		}
		b.WriteByte(c)
	}
	return "", "", false
}

// unescapeEscape returns the escape \c inside the pattern. PHP passes the "\" on to PCRE, and regexp2 reads a
// "\" before ASCII punctuation as PCRE does. Only \_ fails in regexp2, so it becomes _, which PCRE reads the same.
func unescapeEscape(c byte) string {
	if c == '_' {
		return "_"
	}
	return "\\" + string(c)
}

// closingDelimiter returns the closing delimiter of open. The bracket delimiters close with their partner and
// nest.
func closingDelimiter(open byte) (closing byte, nests bool) {
	switch open {
	case '(':
		return ')', true
	case '[':
		return ']', true
	case '{':
		return '}', true
	case '<':
		return '>', true
	default:
		return open, false
	}
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// convertReplacement turns a PHP TO into a regexp2 replacement. The references \N, \NN, $N, $NN, ${N} and ${NN}
// become ${N}, or nothing for a group that the pattern lacks, as in PHP. \\ and \$ give \ and $. Every other $
// becomes $$, so the regexp2 substitutions such as $_ and $& stay text.
func convertReplacement(to string, hasGroup func(n int) bool) string {
	var b strings.Builder
	for i := 0; i < len(to); i++ {
		c := to[i]
		if c == '\\' && i+1 < len(to) && (to[i+1] == '\\' || to[i+1] == '$') {
			i++
			c = to[i]
		} else if n, size, ok := backReference(to[i:]); ok {
			if hasGroup(n) {
				b.WriteString("${" + strconv.Itoa(n) + "}")
			}
			i += size - 1
			continue
		}
		if c == '$' {
			b.WriteString("$$")
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// phpBackReference is a back reference of a PHP TO, as preg_get_backref of php_pcre.c reads it.
var phpBackReference = regexp.MustCompile(`^(?:[\\$]([0-9]{1,2})|\$\{([0-9]{1,2})\})`)

// backReference returns the group number of the back reference at the start of s and its length in bytes.
func backReference(s string) (n, size int, ok bool) {
	m := phpBackReference.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	n, _ = strconv.Atoi(m[1] + m[2])
	return n, len(m[0]), true
}
