# Core markup

A SubRip line `<i>Fish & Chips</i>` reads as `<i>Fish &amp; Chips</i>`. An ASS line `{\i1}Hi{\i0}` reads as `<i>Hi</i>`. Every cue line holds **core markup**: text with `&`, `<` and `>` escaped, plus HTML-like inline tags. Readers write this string, and writers convert the tags back to the syntax of their format.

- **Why a string**: the PHP project stores the same string, so the Go readers compare byte for byte with the PHP read goldens.
- **Cost**: writers parse the tags again with exact string rules. The helpers below hold these rules, so a format package does not write its own.

## Terms

- **Tag**: `<` not followed by white space (space, tab, LF, CR, FF, VT), then any characters except `<` and `>`, then `>`. So `< b>` and `a < b > c` are text, `<>` is a tag, and a quote does not hide the `>`: `<v O'Neil>` is one tag.
- **Tag name**: the text after `<` or `</` up to white space, `/` or `>`. The name of `<c.yellow>` is `c.yellow`.
- **Word timestamp**: `<hh:mm:ss.mmm>` with 2 or more hour digits, such as `<00:01:02.500>` or `<100:59:59.999>`. `<01:02.500>` and `<00:00:01.50>` are text in tag form.
- **Entity**: `&name;`, `&#digits;` or `&#xhex;`.
- **Text run**: the text between 2 tags of one line.

## Tag set

| Tag | Meaning | Written by |
|:--- |:--- |:--- |
| `<b>`, `<i>`, `<u>`, `<s>` | bold, italic, underline, strikethrough | most readers |
| `<font color="#rrggbb">` | text color. Readers write lower-case `#rrggbb` | ASS, SAMI, TTML, SubRip (as written) and others |
| `<v Name>`, `<v.class Name>` | speaker | WebVTT (as written), ASS `Name`, TTML agents |
| `<hh:mm:ss.mmm>` | word timestamp: the time a word is spoken | WebVTT, ASS karaoke, TTML timed spans, LRC |
| `<c.class>`, `<lang xx>`, `<ruby>`, `<rt>`, class forms such as `<b.loud>` | WebVTT spans, kept as written | WebVTT only |
| any other tag | kept as written | SubRip (tags with attributes), WebVTT |

- **Style tags**: `b i u s font`. **Core tags**: the style tags and `v`.
- **Unknown tags**: SubRip and WebVTT readers escape a start tag without attributes, such as `<today>`, as text when no line of the cue holds its end tag. Its name must not be a core tag, `c`, `lang`, `ruby`, `rt` or `br`. FFmpeg writes such text from ASS.
- **Speaker names**: a quote in a name stays a raw character, for example `<v O'Neil>`.
- **Ruby**: WebVTT keeps it and library JSON keeps the text. Every other writer writes `<ruby>base<rt>kan</rt></ruby>` as `base (kan)` and drops `<rp>` elements.

## Escaping

Text outside tags holds `&`, `<` and `>` as `&amp;`, `&lt;` and `&gt;`.

- **WebVTT exception**: the WebVTT reader escapes only a bare `&`. A raw `>`, and a `<` before white space, stay raw in WebVTT lines, because WebVTT text allows them and a round trip keeps the bytes.
- **Changed text runs**: `MapTextRuns` escapes a changed run in the form of its raw run. It keeps `&` and `>` raw where the raw run has them raw, and it keeps an entity such as `&nbsp;` as an entity.
- **Entities**: `DecodeEntities` decodes as PHP `html_entity_decode` with `ENT_QUOTES | ENT_HTML5`. A named entity needs its `;`, and a numeric entity decodes only to a code point that HTML5 allows. So `&amp` and `&#13;` stay as written. Go `html.UnescapeString` differs: it maps `&#128;` to the euro sign and decodes `&notit;` as the not sign plus `it;`.

## Colors

- **Font colors**: `<font color='#FF0000'>`, `<font color=#ff0000>` and `<FONT COLOR="#FF0000">` give the same color. Writers read the `color` attribute with double quotes, single quotes or no quotes, in any case and next to other attributes.
- **Color values**: writers read every value with one rule. They accept `#rgb`, `#rrggbb`, `#rrggbbaa`, `rgb(r, g, b)`, `rgba(r, g, b, a)` and the 18 TTML color names, in any case and with entities decoded. So `#FF0`, `yellow` and `Yellow` all give `#ffff00`, and `#ff0000zz` gives `#ff0000`.
- **Alpha**: the alpha of `rgba()` is a fraction such as `0.5`, a percentage, or an integer from 0 to 255. So `rgba(255,0,0,0.5)` gives `#ff000080`, and `rgba(255,0,0,1)` gives `#ff000001`. CSS reads `rgba(255,0,0,1)` as opaque. The integer rule is the TTML rule, and TTML files use it.
- **Alpha in output**: TTML output keeps a valid TTML color as written and keeps the alpha channel. The other writers drop the alpha.
- **WebVTT color classes**: a `<c>` tag with one of the 8 [color classes](https://www.w3.org/TR/webvtt1/#default-text-color) is a text color. Writers that write `<font color>` read `<c.yellow>` as `<font color="#ffff00">`. The colors are `white` `#ffffff`, `lime` `#00ff00`, `cyan` `#00ffff`, `red` `#ff0000`, `yellow` `#ffff00`, `magenta` `#ff00ff`, `blue` `#0000ff` and `black` `#000000`. When a tag has 2 color classes, the later one in this list wins, as in a browser. `bg_` classes such as `bg_black` set the background color.
- **WebVTT output**: the WebVTT writer turns a `<font>` color with a class into that class and drops other colors.
- **Color names**: the 16 names of HTML 4.01 plus `magenta` and `cyan`, as [TTML](https://www.w3.org/TR/ttml1/#style-value-namedColor) lists them. The SAMI reader reads only the 16 HTML names.

## Helpers

Package `markup` holds the public helpers. They help in an OCR engine, a text change of your own, or a reader for a format that the library does not have. The helpers that only the format packages need live in `internal/markupcore`, so they stay out of the API.

<!-- example: Example -->
```go
fmt.Println(markup.StripAllTags("<b>Hi</b> &amp; bye"))
fmt.Println(markup.PlainText("<b>Hi</b> &amp; bye"))
fmt.Println(markup.KeepTags("<b>Hi</b> <c.red>you</c>", "b"))
fmt.Println(markup.EscapeText("Fish & Chips"))
fmt.Println(markup.VoiceTag("O'Neil", ""))
fmt.Println(markup.HasVisibleText([]string{"<i></i>", " "}))
// Output:
// Hi &amp; bye
// Hi & bye
// <b>Hi</b> you
// Fish &amp; Chips
// <v O'Neil>
// false
```

- **Word timestamps**: `InsertWordTimestamps` escapes the text and skips a word without a start or a word that it does not find. `MapWordTimestamps` and `Cue.MapWordTimestamps` round each new time to whole milliseconds and make a negative time 0.
- **Visible length**: `VisibleLength` counts the characters of the text without tags, with entities decoded and trimmed. U+00A0 is not trimmed.
