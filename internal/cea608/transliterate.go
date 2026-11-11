package cea608

// transliterations lists, for each replacement, the characters that CEA-608 lacks and that it stands in for.
var transliterations = []struct{ to, from string }{ //nolint:gochecknoglobals // constant table
	{"A", "ĀĂĄǍ"},
	{"a", "āăąǎ"},
	{"C", "ĆĈĊČ"},
	{"c", "ćĉċč"},
	{"D", "ÐĎĐ"},
	{"d", "ðďđ"},
	{"E", "ĒĔĖĘĚ"},
	{"e", "ēĕėęě"},
	{"G", "ĜĞĠĢ"},
	{"g", "ĝğġģ"},
	{"H", "ĤĦ"},
	{"h", "ĥħ"},
	{"I", "ĨĪĬĮİ"},
	{"i", "ĩīĭįı"},
	{"J", "Ĵ"},
	{"j", "ĵ"},
	{"K", "Ķ"},
	{"k", "ķ"},
	{"L", "ĹĻĽĿŁ"},
	{"l", "ĺļľŀł"},
	{"N", "ŃŅŇ"},
	{"n", "ńņň"},
	{"O", "ŌŎŐ"},
	{"o", "ōŏő"},
	{"R", "ŔŖŘ"},
	{"r", "ŕŗř"},
	{"S", "ŚŜŞŠȘ"},
	{"s", "śŝşšș"},
	{"T", "ŢŤŦȚ"},
	{"t", "ţťŧț"},
	{"U", "ŨŪŬŮŰŲ"},
	{"u", "ũūŭůűų"},
	{"W", "Ŵ"},
	{"w", "ŵ"},
	{"Y", "ÝŶŸ"},
	{"y", "ýÿŷ"},
	{"Z", "ŹŻŽ"},
	{"z", "źżž"},
	{"AE", "Æ"},
	{"ae", "æ"},
	{"OE", "Œ"},
	{"oe", "œ"},
	{"IJ", "Ĳ"},
	{"ij", "ĳ"},
	{"TH", "Þ"},
	{"th", "þ"},
	{"-", "\u2010\u2011\u2012\u2013\u2015\u2212"},
	{"'", "`\u00b4\u201a\u2032"},
	{`"`, "\u201e\u2033"},
	{"<", "\u2039"},
	{">", "\u203a"},
	{"...", "\u2026"},
	{".", "\u00b7"},
	{"x", "\u00d7"},
	{"EUR", "\u20ac"},
}

var transliterationOf = buildTransliterations() //nolint:gochecknoglobals // lookup built once from the table above

func buildTransliterations() map[rune]string {
	m := map[rune]string{}
	for _, t := range transliterations {
		for _, r := range t.from {
			m[r] = t.to
		}
	}
	return m
}

// Transliterate returns the characters that CEA-608 has for a character it lacks, such as "S" for U+0160.
func Transliterate(r rune) (string, bool) {
	s, ok := transliterationOf[r]
	return s, ok
}
