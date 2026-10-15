package model

import "strconv"

// DialogueDashStyle is the dash, and the space after it, that starts a dialogue line.
type DialogueDashStyle uint8

// The dialogue dash styles. DialogueDashHyphenSpace is the zero value.
const (
	// DialogueDashHyphenSpace is a hyphen-minus and a space, "- ".
	DialogueDashHyphenSpace DialogueDashStyle = iota
	// DialogueDashHyphen is a hyphen-minus, "-".
	DialogueDashHyphen
	// DialogueDashUnicodeHyphen is U+2010.
	DialogueDashUnicodeHyphen
	// DialogueDashUnicodeHyphenSpace is U+2010 and a space.
	DialogueDashUnicodeHyphenSpace
	// DialogueDashEnDash is U+2013.
	DialogueDashEnDash
	// DialogueDashEnDashSpace is U+2013 and a space.
	DialogueDashEnDashSpace
	// DialogueDashEmDash is U+2014.
	DialogueDashEmDash
	// DialogueDashEmDashSpace is U+2014 and a space.
	DialogueDashEmDashSpace
)

// Text returns the dash and its space, or "" for a value outside the constants.
func (d DialogueDashStyle) Text() string {
	texts := [...]string{"- ", "-", "\u2010", "\u2010 ", "\u2013", "\u2013 ", "\u2014", "\u2014 "}
	if int(d) < len(texts) {
		return texts[d]
	}
	return ""
}

// String returns the constant name, such as "DialogueDashHyphenSpace".
func (d DialogueDashStyle) String() string {
	names := [...]string{
		"DialogueDashHyphenSpace", "DialogueDashHyphen", "DialogueDashUnicodeHyphen", "DialogueDashUnicodeHyphenSpace",
		"DialogueDashEnDash", "DialogueDashEnDashSpace", "DialogueDashEmDash", "DialogueDashEmDashSpace",
	}
	if int(d) < len(names) {
		return names[d]
	}
	return "DialogueDashStyle(" + strconv.Itoa(int(d)) + ")"
}
