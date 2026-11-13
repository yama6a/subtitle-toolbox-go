# Core markup

A SubRip line `<i>Fish & Chips</i>` reads as `<i>Fish &amp; Chips</i>`. An ASS line `{\i1}Hi{\i0}` reads as `<i>Hi</i>`. Every cue line holds **core markup**: text with `&`, `<` and `>` escaped, plus HTML-like inline tags. Readers write this string, and writers convert the tags back to the syntax of their format.

- **Why a string**: the PHP project stores the same string, so the Go readers compare byte for byte with the PHP read goldens.
- **Cost**: writers parse the tags again with exact string rules. The helpers below hold these rules, so a format package does not write its own.

## Terms

- **Tag**: `<` not followed by white space (space, tab, LF, CR, FF, VT), then any characters except `<` and `>`, then `>`. So `< b>` and `a < b > c` are text, `<>` is a tag, and a quote does not hide the `>`: `<v O'Neil>` is one tag.
- **Tag name**: the text after `<` or `</` up to white space, `/` or `>`. The name of `<c.yellow>` is `c.yellow`.
- **Word timestamp**: `<hh:mm:ss.mmm>` with 2 or more hour digits, such as `<00:01:02.500>` or `<100:59:59.999>`. `<00:00:01.50>` is a tag, but not a word timestamp. The WebVTT reader writes `<01:02.500>` as `<00:01:02.500>`, so it is a word timestamp there. The lines of a cue hold no word timestamps, because `Cue.Words` holds them. See the section on word timestamps and voices below.
- **Entity**: `&name;`, `&#digits;` or `&#xhex;`.
- **Text run**: the text between 2 tags of one line.

## Tag set

| Tag | Meaning | Written by |
|:--- |:--- |:--- |
| `<b>`, `<i>`, `<u>`, `<s>` | bold, italic, underline, strikethrough | most readers |
| `<font color="#rrggbb">` | text color. Readers write lower-case `#rrggbb` | ASS, SAMI, TTML, SubRip (as written) and others |
| `<v Name>`, `<v.class Name>` | speaker | WebVTT (as written), ASS `Name`, TTML agents |
| `<hh:mm:ss.mmm>` | word timestamp: the time a word is spoken | `Cue.TaggedLines` for WebVTT, ASS karaoke, TTML timed spans, LRC and library JSON version 1 |
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

<!-- example: markup/Example -->
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

- **Word timestamps**: `Cue.ExtractWordTags` moves them out of the lines into `Cue.Words`, and `Cue.TaggedLines` puts them back. The package `markup` has no helper for them.
- **Visible length**: `VisibleLength` counts the characters of the text without tags, with entities decoded and trimmed. U+00A0 is not trimmed. A character is a grapheme cluster of UAX #29, so `Cafe` plus U+0301 counts 4 and a thumbs-up with a skin tone counts 1. The soft hyphen U+00AD, the zero-width space U+200B, the BOM U+FEFF and the bidi controls count 0. Invalid UTF-8 counts its bytes. [grapheme.go](../internal/grapheme/grapheme.go) names the Unicode version.

## Word timestamps and voices

A reader of a format with word timestamp tags calls `Cue.ExtractWordTags` on each cue. A writer writes `Cue.TaggedLines()` in place of `Cue.Lines`. This keeps the bytes of every unedited file.

### Placement rule

`TaggedLines` puts the tag of a word at its place by these steps. Example: the word `dusk` at 6.5 s in the line `at <i>dusk</i>` gives `at <00:00:06.500><i>dusk</i>`.

1. Find the word in the plain text of the cue, which is the lines joined with one space without tags and with entities decoded. Each word is searched after the end of the word before it. The first byte of the match is the place of the word.
2. Move back from that place while the bytes before it are white space or a whole opening tag. An opening tag is a tag that does not start with `</`.
3. Move forward over white space.
4. Write `<` and the time of the word and `>` there.

A word that the text does not hold gets no tag, and neither do the words after it. The voice tag of `Speaker`, such as `<v.loud Anna>`, goes at the start of the first line with visible text. With the vtt cue data `voiceEnd`, `</v>` goes at the end of the last line with visible text.

### Reading rules

`ExtractWordTags` reads the lines of a cue like this:

1. It cleans the lines as `SetText` does.
2. A word timestamp is a tag that matches the word timestamp pattern with a time below 100000 hours. Other tags stay in the lines.
3. A tag whose time is before the last kept tag is dropped. Its text belongs to the word before it.
4. Each kept tag starts a word. Its text runs to the next kept tag or to the end of the cue. The text has no tags, with entities decoded and white space collapsed and trimmed. A word without text is dropped.
5. The end of a word is the time of the next tag in the cue, also of a dropped tag. The last word ends at the end of the cue. An end is never before its start.
6. The word timestamp tags leave the lines. When the removal joins text into a new word timestamp tag, such as `<<00:00:01.000>00:00:02.000>`, that tag leaves too.
7. The cue has a whole-cue voice when the first line with visible text starts with a `<v>` tag with a name, every `<v>` tag has the same classes and name, no `</v>` is followed by visible text, and the classes are empty or the cue has vtt cue data.
8. With a whole-cue voice, every `<v>` tag and every `</v>` tag leaves the lines. `Speaker` gets the name.
9. The lines are cleaned again.

A cue with 2 voices keeps its `<v>` tags in the lines and has no `Speaker`. A tag removal can join text into a new tag, such as `<a<00:00:01.000>>`. Then the words that the new text lacks go.

### Tagged source

`ExtractWordTags` keeps the lines it read. While the lines, the speaker and the text and start of each word are as `ExtractWordTags` left them, `TaggedLines` returns the lines it read, byte for byte. A time retiming or a text edit changes that, so the placement rule writes the tags. A voice class or a `</v>` that was not stored in vtt cue data stays with an edited cue while `Speaker` stays.
