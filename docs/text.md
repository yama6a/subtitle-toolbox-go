# Text transforms

Caption files from TV often come in upper case. A CEA-608 WebVTT cue `>> GOOD MORNING. THE 8:15 TRAIN` becomes `>> Good morning. The 8:15 train` with `s.ChangeCase(model.CaseSentence, "en")`.

<!-- example: ExampleSubtitle_ChangeCase -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 1001 * time.Millisecond, End: 3336 * time.Millisecond, Lines: []string{">> GOOD MORNING. THE 8:15 TRAIN", "TO MAIN STREET IS LATE."}},
	model.Cue{Start: 3503 * time.Millisecond, End: 5505 * time.Millisecond, Lines: []string{"<i>I THINK WE MISSED IT.</i>"}},
)

if err := s.ChangeCase(model.CaseSentence, "en"); err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Text())
}
// Output:
// >> Good morning. The 8:15 train
// to main street is late.
// <i>I think we missed it.</i>
```

| Method | Works on | Does |
|:--- |:--- |:--- |
| `MapText(fn)` | text runs | calls `fn(text, cue)` for each text run |
| `MapLines(fn)` | whole lines | calls `fn(line, cue)` for each line, tags and entities included. Use it to change tags |
| `ReplaceText(search, replace, opts)` | text runs | replaces plain text or a regular expression |
| `StripFormatting(opts)` | whole lines | removes tags, except the names in a keep list |
| `ChangeCase(mode, language)` | text runs | writes upper case, lower case or sentence case |
| `FixRightToLeft()` | whole lines | wraps each right-to-left line in an embedding |
| `RemoveBidiControls()` | text runs | removes U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069 |
| `EditLines(fn, hasText)` | whole cues | sets new lines on many cues, then applies the cue removal rule |

- **Text run**: the text between 2 tags of one line, with `&lt;`, `&gt;` and `&amp;` decoded. A changed run is escaped again in the style of its raw form, so a replacement cannot add tags. A run that comes back unchanged keeps its bytes. Other entities such as `&nbsp;` stay as written. [markup.md](markup.md#escaping) has the escaping rules.
- **Tags never match**: a search cannot match inside a tag, and a match cannot cross a tag. `ReplaceText("ff", "XX", ...)` keeps `<font color="#ff0000">`.
- **Character counts**: `markup.VisibleLength` counts grapheme clusters, see [markup.md](markup.md#helpers).

## Cue removal rule

A cue that had visible text before a transform and has none after is removed. Its comments move to the next cue. A cue that was empty before stays.

- **Example**: replacing `\[\w+\]` with `""` removes the cue `[MUSIC]`, and a comment before it moves before the next cue.
- **EditLines**: every method on this page applies the rule through `EditLines`. Code outside package `model` uses it too, because the cue list of a `Subtitle` is not exported. `fn` returns the new lines and true, or false to keep the cue as it is.
- **Visible text**: `hasText` decides. Nil means `markup.HasVisibleText`, where an entity counts as text.
- **Result**: `EditLines` returns the old indexes of the removed cues, so a service can report how many cues went.

## Case

| Mode | Text run in valid UTF-8 | Text run in invalid UTF-8 |
|:--- |:--- |:--- |
| `CaseUpper` | full Unicode upper case: U+00DF becomes `SS` | ASCII letters only |
| `CaseLower` | full Unicode lower case, with the Greek final sigma rule | ASCII letters only |
| `CaseSentence` | lower case, with an upper case letter at the start of each sentence | ASCII rules |

- **Full mapping**: `golang.org/x/text/cases` maps the case. `strings.ToUpper` keeps U+00DF, so it does not fit.
- **Language**: a BCP 47 code such as `tr-TR`, or `""` for none. The **primary subtag** is the part before the first `-` or `_`, in lower case.
- **Turkic rules**: the primary subtags `tr` and `az` map `i` to U+0130 in upper case, and `I` to U+0131 in lower case. Without them, U+0130 becomes `i` plus U+0307 in lower case.
- **Mode errors**: any other `CaseMode`, `""` included, returns an error that matches `ErrInvalidArgument`.

### Sentence case

One pass runs over the cues in order. The state carries across cues and text runs.

- **Cue start**: a cue starts a sentence when one of these holds:
  - it is the first cue
  - the text of the previous cue ends with `.`, `!`, `?` or U+2026, then optional closing marks such as `"` or `)`
  - the text of the previous cue ends with `]`, `)`, U+266A or `:`, a sound description or a speaker label
  - its own text starts with `[`, `(` or U+266A
  - it starts 2 s or more after the end of the previous cue
- **Line start**: a line starts a sentence after sentence punctuation, and when it starts with `>>` or a dialogue dash. `>>` marks a speaker change in CEA-608 captions.
- **Inside a run**: a space after `.`, `!`, `?` or U+2026 starts a sentence. A comma right after the punctuation cancels it, so `"WHY?," HE ASKED.` becomes `"Why?," he asked.`. A dot inside a word starts nothing, so `www.example.com` stays lower case.
- **Abbreviations**: after `i.e.`, `e.g.`, `etc.` and `vs.`, a lower case word in the input continues the sentence and an upper case word starts one. So `Fruit, e.g. apples.` stays, and `APPLES, ETC. THEN WE LEFT.` becomes `Apples, etc. Then we left.`.
- **English pronoun I**: with the language `""` or the primary subtag `en`, `i`, `i'm`, `i'll`, `i've` and `i'd` become upper case as whole words. The check reads the whole line across tags, so `W<b>I</b>NDOW` becomes `W<b>i</b>ndow`. `i.e.` and `i's` stay lower case.
- **Proper names**: they become lower case, as in `I saw john.`. Fix them after the pass with `ReplaceText`.

## Replace

| Options | Match | `replace` |
|:--- |:--- |:--- |
| none | each byte match, as `strings.ReplaceAll` | literal |
| `IgnoreCase` | the search in any case, with Unicode case folding | literal: `$1` and `\1` stay as written |
| `Regex` | a regexp2 pattern, case-sensitive | substitutions `$1`, `${1}`, `${name}` and `$$` |
| `Regex`, `IgnoreCase` | a regexp2 pattern in any case | substitutions |

<!-- example: ExampleSubtitle_ReplaceText -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"[BELL RINGS]"}},
	model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"<i>Wait.....</i> [DOOR] Tom &amp; Jerry"}},
)

if err := s.ReplaceText(`\s*\[[^\]]*\]`, "", model.ReplaceOptions{Regex: true}); err != nil {
	fmt.Println(err)
	return
}
if err := s.ReplaceText(`\.{4,}`, "...", model.ReplaceOptions{Regex: true}); err != nil {
	fmt.Println(err)
	return
}
if err := s.ReplaceText("tom & jerry", "Tom & Jerry's", model.ReplaceOptions{IgnoreCase: true}); err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Start, c.Text())
}
// Output:
// 4s <i>Wait...</i> Tom &amp; Jerry's
```

- **Why regexp2**: a user pattern needs lookaround and back references, which the Go `regexp` package lacks. [github.com/dlclark/regexp2](https://github.com/dlclark/regexp2) ports the .NET syntax.
- **Pattern**: the bare pattern, without delimiters and flags. Turn on case-insensitive matching with `IgnoreCase`, or with `(?i)` inside the pattern.
- **Not in regexp2**: the PCRE features `\K`, `\h`, `\R`, `\X`, possessive quantifiers such as `a++`, and `(*VERB)` controls. A pattern with one of them returns the invalid pattern error. Write `[ \t]+` for `\h+`.
- **Time limit**: each match may take 1 s. A slower match returns an error that matches `ErrInvalidArgument` and names the cue, and the subtitle stays unchanged. A pattern such as `(a+)+$` takes that long on 40 letters `a` and a `!`.
- **Errors**: an empty search, an invalid pattern and an invalid replacement return an error that matches `ErrInvalidArgument`.
- **Invalid UTF-8**: a regex keeps such a run unchanged, because regexp2 reads runes and would write U+FFFD for a bad byte. A plain search that ignores case matches the case of ASCII letters only in such a run.

## Strip tags

`StripFormatting` removes the tags of each line. `<b>Run</b>, <font color="#ff0000">now</font>!` becomes `Run, now!`.

- **Keep list**: `StripOptions.KeepTags` names the tags to keep, compared in lower case. With `KeepTags: []string{"i"}`, `<b><i>Run</i></b> &amp; <v Fred>hide` becomes `<i>Run</i> &amp; hide`.
- **Word timestamps**: `<00:00:01.500>` tags stay unless `RemoveWordTimestamps` is set.
- **ASS override blocks**: `{\an8}` is no tag and stays.

## Right-to-left text

A player with a left-to-right base direction shows the final `?` of an Arabic line at its left end. `FixRightToLeft` wraps each line with a right-to-left letter in U+202B and U+202C, inside its leading and trailing tags. Punctuation that a file stores at the start of the text moves to its end.

- **Idempotent**: a second call changes nothing, so a pipeline can call it on every file.
- **Lines without right-to-left letters**: left-to-right lines, lines with an ASS drawing and lines of invalid UTF-8 stay as they are.
- **Undo**: `RemoveBidiControls` removes the embedding and every other bidi control from the text runs. U+00A0 stays. It does not move punctuation back, so a file that `FixRightToLeft` repaired keeps the repaired order.

## Dialogue dashes

A **dialogue dash** is a hyphen-minus, U+2010, U+2013 or U+2014 at the start of a line that opens a dialogue turn. Sentence case starts a sentence there.

- **Rule**: the dash is not followed by another dash. After it come 0 or more spaces, tabs or U+00A0, then a character that is neither white space nor a number.
- **Examples**: `- Hi` and `-Hi` start a turn. `-20 degrees`, `--` and a lone `-` do not.
- **Styles**: `DialogueDashStyle` names 8 forms of the dash and its space. `Text()` gives the form as text.

| Style | Text |
|:--- |:--- |
| `DialogueDashHyphenSpace`, the zero value | `-` and a space |
| `DialogueDashHyphen` | `-` |
| `DialogueDashUnicodeHyphen` | U+2010 |
| `DialogueDashUnicodeHyphenSpace` | U+2010 and a space |
| `DialogueDashEnDash` | U+2013 |
| `DialogueDashEnDashSpace` | U+2013 and a space |
| `DialogueDashEmDash` | U+2014 |
| `DialogueDashEmDashSpace` | U+2014 and a space |
