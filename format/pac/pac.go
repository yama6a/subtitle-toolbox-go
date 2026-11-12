// Package pac reads and writes PAC, the binary subtitle file of the Screen Electronics systems, and PAC Unicode
// (.fpc), its form with UTF-8 text.
//
// A file holds a header of 20 bytes, one record per subtitle and an end block that starts with FFh. A record holds
// the subtitle number, the time codes, the vertical position and the text lines. The file does not store its code
// page, so the reader takes it from the story record or detects it.
//
// PAC has no public spec. The code follows Pac.cs of Subtitle Edit, see THIRD_PARTY_NOTICES.md.
package pac

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const formatDataKey = string(model.PAC)

// CodePage is a code page of PAC text. Its value is the codePage field of the PAC format data.
type CodePage string

// The code pages. The double-byte code pages store their lines as double-byte text after a W16 code, and their
// single-byte text is Latin. Unicode is PAC Unicode, which stores each line as UTF-8.
const (
	// CodePageAuto takes the code page from the story record language, else detects it.
	CodePageAuto       CodePage = ""
	Latin              CodePage = "latin"
	LatinCzech         CodePage = "latin-czech"
	LatinTurkish       CodePage = "latin-turkish"
	Greek              CodePage = "greek"
	Cyrillic           CodePage = "cyrillic"
	Arabic             CodePage = "arabic"
	Hebrew             CodePage = "hebrew"
	Thai               CodePage = "thai"
	ChineseTraditional CodePage = "chinese-traditional"
	ChineseSimplified  CodePage = "chinese-simplified"
	Korean             CodePage = "korean"
	Japanese           CodePage = "japanese"
	Unicode            CodePage = "unicode"
)

func (c CodePage) known() bool {
	switch c {
	case Latin, LatinCzech, LatinTurkish, Greek, Cyrillic, Arabic, Hebrew, Thai,
		ChineseTraditional, ChineseSimplified, Korean, Japanese, Unicode:
		return true
	case CodePageAuto:
		return false
	}
	return false
}

// encodingCodePages maps the canonical names of package textenc to their code page.
//
//nolint:gochecknoglobals // lookup table of the encoding names
var encodingCodePages = map[string]CodePage{
	"ISO-8859-1": Latin, "Windows-1252": Latin,
	"ISO-8859-2": LatinCzech, "Windows-1250": LatinCzech,
	"ISO-8859-9": LatinTurkish, "Windows-1254": LatinTurkish,
	"ISO-8859-7": Greek, "Windows-1253": Greek,
	"ISO-8859-5": Cyrillic, "Windows-1251": Cyrillic,
	"ISO-8859-6": Arabic, "Windows-1256": Arabic,
	"ISO-8859-8": Hebrew, "Windows-1255": Hebrew,
	"Windows-874": Thai, "ISO-8859-11": Thai,
	"Big5": ChineseTraditional,
	"GBK":  ChineseSimplified, "GB18030": ChineseSimplified,
	"CP949": Korean, "EUC-KR": Korean,
	"Shift_JIS": Japanese,
	"UTF-8":     Unicode,
}

// CodePageFromEncoding returns the code page of an encoding name, such as "ISO-8859-7" for Greek. It accepts the
// names and aliases of stbox.Encodings, compared without case. It returns false for a name that is unknown or
// that no PAC code page uses, such as KOI8-R.
func CodePageFromEncoding(name string) (CodePage, bool) {
	e, err := textenc.Lookup(name)
	if err != nil {
		return CodePageAuto, false
	}
	c, ok := encodingCodePages[e.Name]
	return c, ok
}

// ReadOptions holds the settings of the PAC reader.
type ReadOptions struct {
	// CodePage is the code page of the text. CodePageAuto takes the code page from the story record language, else
	// detects it. stbox.Parse and stbox.Load set it from ReadOptions.Encoding when it is CodePageAuto, and Read
	// never sees Encoding.
	CodePage CodePage
}

// Validate returns an error that matches model.ErrInvalidArgument for a code page that does not exist.
func (o ReadOptions) Validate() error {
	if o.CodePage != CodePageAuto && !o.CodePage.known() {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.PAC.CodePage",
			Msg:    "The PAC code page " + strconv.Quote(string(o.CodePage)) + " is unknown.",
		}
	}
	return nil
}

// WriteOptions holds the settings of the PAC writer. PAC has none yet.
type WriteOptions struct{}

// WriteReport holds the changes that the PAC writer made to fit the subtitle into PAC. PAC has none yet.
type WriteReport struct{}
