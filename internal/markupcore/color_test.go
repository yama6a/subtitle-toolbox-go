package markupcore_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func TestFontColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		attrs  string
		want   string
		wantOK bool
	}{
		{"double quotes", ` color="#FF0000"`, "#FF0000", true},
		{"single quotes", ` color='#FF0000'`, "#FF0000", true},
		{"no quotes in a whole tag", "<font color=red>", "red", true},
		{"upper case and spaces kept", ` face="Arial" COLOR = " #ff0000 "`, " #ff0000 ", true},
		{"entity kept", ` color='&quot;x'`, "&quot;x", true},
		{"other attribute after", `<font color="#ff0000" face=x>`, "#ff0000", true},
		{"empty value", `<font color="">`, "", true},
		{"no value", "<font color=>", "", false},
		{"no color", ` face="Arial"`, "", false},
		{"other attribute name", ` bgcolor="#ff0000"`, "", false},
		{"tag with single quotes", "<font color='#FF0000'>", "#FF0000", true},
		{"tag without quotes", "<font color=#ff0000>", "#ff0000", true},
		{"tag in upper case", `<FONT COLOR="#FF0000">`, "#FF0000", true},
		{"tag with spaces around the equal sign", `<font color = "#ff0000" >`, "#ff0000", true},
		{"tag with other attributes", `<font face="Arial" color="#ff0000">`, "#ff0000", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.FontColor(tt.attrs)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColorHexWithAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		color  string
		want   string
		wantOK bool
	}{
		{"#FF000080", "#ff000080", true},
		{"rgba(255, 0, 0, 0.5)", "#ff000080", true},
		{"rgba(255,0,0,0.5)", "#ff000080", true},
		{"rgba(255,0,0,50%)", "#ff000080", true},
		{"RGBA(255,0,0,128)", "#ff000080", true},
		{"rgba(255,0,0,1)", "#ff000001", true},
		{"rgba(255,0,0,.5)", "#ff000080", true},
		{"rgba(255,0,0,255)", "#ff0000ff", true},
		{"rgba(0,0,0,0)", "#00000000", true},
		{"rgb(1,2,3,4)", "#01020304", true},
		{"rgba(1,2,3)", "#010203", true},
		{"#ff0000zz", "#ff0000", true},
		{"#ff000080f", "#ff0000", true},
		{"red", "#ff0000", true},
		{"RED", "#ff0000", true},
		{"#FFF", "#ffffff", true},
		{" &#X23;FF0 ", "#ffff00", true},
		{"rgba(255,0,0,1.5)", "", false},
		{"rgba(255,0,0,5.)", "", false},
		{"rgba(255,0,0,100.2%)", "", false},
		{"rgba(255,0,0,256)", "", false},
		{"rgba(1,2,3,9223372036854775808)", "", false},
		{"rgb(256,0,0)", "", false},
		{"rgb(1000,0,0)", "", false},
		{"#ff00", "", false},
		{"transparent", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.ColorHexWithAlpha(tt.color)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestColorHex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		color  string
		want   string
		wantOK bool
	}{
		{"#FF0", "#ffff00", true},
		{"yellow", "#ffff00", true},
		{"Yellow", "#ffff00", true},
		{" Yellow ", "#ffff00", true},
		{"#FFFF0080", "#ffff00", true},
		{"&#35;ff0", "#ffff00", true},
		{"rgb(255, 255, 0)", "#ffff00", true},
		{"rgb( 255 , 128 , 0 )", "#ff8000", true},
		{"RGBA(255,128,0,0.5)", "#ff8000", true},
		{"aqua", "#00ffff", true},
		{"rgb(256,0,0)", "", false},
		{"transparent", "", false},
		{"#ff00", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.ColorHex(tt.color)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTTMLColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		color  string
		want   string
		wantOK bool
	}{
		{"rgba(0,0,0,0)", "rgba(0,0,0,0)", true},
		{"rgba(0, 0, 0, 0)", "rgba(0, 0, 0, 0)", true},
		{"rgb(255, 0, 0)", "rgb(255, 0, 0)", true},
		{"#FFFF0080", "#FFFF0080", true},
		{"transparent", "transparent", true},
		{"Transparent", "transparent", true},
		{"Yellow", "yellow", true},
		{"Teal", "teal", true},
		{"rgba(255, 0, 0, 0.5)", "#ff000080", true},
		{"RGB(0,0,255)", "#0000ff", true},
		{"#FF0", "#ffff00", true},
		{"grey", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.TTMLColor(tt.color)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamedColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		htmlOnly bool
		want     string
		wantOK   bool
	}{
		{"silver", false, "#c0c0c0", true},
		{"Aqua", true, "#00ffff", true},
		{"cyan", false, "#00ffff", true},
		{"cyan", true, "", false},
		{"magenta", true, "", false},
		{"grey", false, "", false},
		{"transparent", false, "", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.name, tt.htmlOnly), func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.NamedColor(tt.name, tt.htmlOnly)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWebVTTClassColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		classes        string
		wantColor      string
		wantBackground string
	}{
		{".bg_black.yellow", "#ffff00", "#000000"},
		{".yellow.white.bg_red.bg_lime", "#ffff00", "#ff0000"},
		{"yellow", "#ffff00", ""},
		{".loud", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.classes, func(t *testing.T) {
			t.Parallel()

			color, background := markupcore.WebVTTClassColors(tt.classes)
			assert.Equal(t, tt.wantColor, color)
			assert.Equal(t, tt.wantBackground, background)
		})
	}
}

func TestWebVTTColorsToFont(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{"color class", "<c.yellow>Hi</c>", `<font color="#ffff00">Hi</font>`},
		{"background class only", "<c.bg_black>Hi</c>", "<c.bg_black>Hi</c>"},
		{"text and background class", "<c.bg_black.yellow>Hi</c>", `<font color="#ffff00">Hi</font>`},
		{
			"nested and annotated tags",
			"<c.yellow>a <c.loud>b</c> c</c> <c.red x>d</c ></c> <C.red>e</c>",
			`<font color="#ffff00">a <c.loud>b</c> c</font> <font color="#ff0000">d</font></c> <C.red>e</c>`,
		},
		{"no c tag", "<i>a</i>", "<i>a</i>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.WebVTTColorsToFont(tt.text))
		})
	}
}

func TestFontToWebVTTColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		text        string
		want        string
		wantDropped []string
	}{
		{"color with a class", `<font color="#ffff00">Hi</font>`, "<c.yellow>Hi</c>", nil},
		{
			"colors without a class and without a value",
			`<font color="#123456">a</font> <FONT COLOR=yellow>b</FONT > <font face=x>c</font> <font color=" ">d</font></font><font color=red><font color=x>e</font></font>`,
			"a <c.yellow>b</c> c d<c.red>e</c>",
			[]string{"#123456", "x"},
		},
		{"no font tag", "plain", "plain", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, dropped := markupcore.FontToWebVTTColors(tt.text)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantDropped, dropped)
		})
	}
}

func TestWebVTTColorClassesRoundTrip(t *testing.T) {
	t.Parallel()

	for _, class := range []string{"white", "lime", "cyan", "red", "yellow", "magenta", "blue", "black"} {
		t.Run(class, func(t *testing.T) {
			t.Parallel()

			text := "<c." + class + ">Line</c>"
			got, dropped := markupcore.FontToWebVTTColors(markupcore.WebVTTColorsToFont(text))
			assert.Equal(t, text, got)
			assert.Empty(t, dropped)
		})
	}
}

func TestBGRToRGB(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "FF8000", markupcore.BGRToRGB("0080FF"))
	assert.Equal(t, "0080ff", markupcore.RGBToBGR("ff8000"))
	assert.Equal(t, "123456", markupcore.RGBToBGR(markupcore.BGRToRGB("123456")))
	assert.Equal(t, "53412", markupcore.BGRToRGB("12345"))
}
