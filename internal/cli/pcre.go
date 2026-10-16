package cli

import "strings"

// pcrePattern is a FROM of --replace-regex after the conversion of convertPCRE.
type pcrePattern struct {
	// pattern is the bare regexp2 pattern, with the flags m, s and x as an inline group.
	pattern    string
	ignoreCase bool
}

// convertPCRE turns a PHP PCRE pattern with delimiters and flags, such as /\.{4,}/i, into a regexp2 pattern.
// ok is false for a pattern that PHP rejects too. badFlag holds the first flag that regexp2 lacks, or "".
func convertPCRE(from string) (p pcrePattern, badFlag string, ok bool) {
	s := strings.TrimLeft(from, phpSpace)
	if s == "" || isAlnum(s[0]) || s[0] == '\\' {
		return pcrePattern{}, "", false
	}
	body, flags, ok := splitDelimited(s)
	if !ok {
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

// readFlags returns the flags m, s and x for an inline group, whether i is set, and the first other flag but u.
func readFlags(flags string) (inline string, ignoreCase bool, badFlag string) {
	for _, f := range flags {
		switch f {
		case 'i':
			ignoreCase = true
		case 'm', 's', 'x':
			if !strings.ContainsRune(inline, f) {
				inline += string(f)
			}
		case 'u':
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
			b.WriteString(unescapeDelimiter(s[i], open, closing))
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

// unescapeDelimiter returns the escape of c inside the pattern. The "\" before a delimiter only escapes it for
// PHP, so it drops, unless c means something else in a pattern without it: +a\+b+ still matches a plus sign.
func unescapeDelimiter(c, open, closing byte) string {
	if (c == open || c == closing) && strings.IndexByte(`.+*?|^$()[]{}#-`, c) < 0 {
		return string(c)
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

// convertReplacement turns the PHP back references \N and \NN of a TO into the regexp2 form ${N}.
func convertReplacement(to string) string {
	var b strings.Builder
	for i := 0; i < len(to); i++ {
		if to[i] == '\\' && i+1 < len(to) && isDigit(to[i+1]) {
			end := i + 2
			if end < len(to) && isDigit(to[end]) {
				end++
			}
			b.WriteString("${" + to[i+1:end] + "}")
			i = end - 1
			continue
		}
		b.WriteByte(to[i])
	}
	return b.String()
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
