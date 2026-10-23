package markupcore

import "strings"

// PrimaryLanguage returns the part of a BCP 47 code before the first "-" or "_", in lower case.
func PrimaryLanguage(code string) string {
	if i := strings.IndexAny(code, "-_"); i >= 0 {
		code = code[:i]
	}
	return ASCIILower(code)
}

// IsTurkic reports whether the primary subtag of code is tr or az, the languages with the Turkic case rules for i.
func IsTurkic(code string) bool {
	primary := PrimaryLanguage(code)
	return primary == "tr" || primary == "az"
}
