# The subtitle model

A SubRip cue `00:00:01,500 --> 00:00:04,000` with the text `<i>Hello</i>` becomes a `Cue` with `Start` 1.5 s, `End` 4 s and one line `<i>Hello</i>`. Every format reads into a `*Subtitle` and writes from it. The godoc of package `model` holds the reference of each type and method.

## Value cues

A `Subtitle` keeps its cues in start order at all times. `Cue` is plain data with exported fields, and the cue list itself is not exported.

- **Copies in, copies out**: `AddCue`, `AddCues`, `SetCue` and `ReplaceCues` store a copy of each cue. `Cue(i)`, `Cues()` and `All()` return copies. A change to a returned `Cue` takes effect only when you pass it back with `SetCue`.
- **Why**: one type owns the start-order rule, and the compiler stops direct writes to the cue list. A cue belongs to no subtitle, so 2 subtitles never share a cue.
- **Cost**: each read copies the lines of the cue. A service that rebuilds many cues calls `ReplaceCues` once and sorts once.
- **All**: the loop sees the cues as they were when it started. Changes inside the loop do not affect it.

## Times and text

- **Times**: `time.Duration`, rounded to whole milliseconds with `Round(time.Millisecond)` when a cue enters a subtitle. 1.0005 s becomes 1.001 s, and -1.0005 s becomes -1.001 s. Negative times are legal in the model. Writers clamp them to 0.
- **End**: nothing checks that `End` is at or after `Start`.
- **Line cleaning**: on entry, CR LF and a lone CR become LF, each run of tabs and spaces becomes 1 space, each run of LF becomes 1 LF, and each line is trimmed. U+00A0 stays. Empty lines are dropped. Cleaning is lossy: double spaces inside a line do not come back.
- **Inner LF**: `Lines: []string{"a\nb"}` keeps one line `"a\nb"`. Only `SetText` splits at LF.
- **No lines**: a cue with 0 lines is legal. A SubRip timing line without text gives one.
- **Markup**: lines hold the core markup. Word times are `<hh:mm:ss.mmm>` tags and speakers are `<v Name>` tags in the text. `MapWordTimestamps` rewrites the word times.

## SetCue and index drift

`SetCue` gives the order of a stable sort of the changed list. The changed cue moves to its sorted place, and cues with the same start keep their index order.

<!-- example: ExampleSubtitle_SetCue -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 1 * time.Second, Lines: []string{"A"}},
	model.Cue{Start: 2 * time.Second, Lines: []string{"B"}},
	model.Cue{Start: 3 * time.Second, Lines: []string{"C"}},
)

c, _ := s.Cue(0) // a copy of A
c.Start = 4 * time.Second
_ = s.SetCue(0, c) // A moves to index 2, B and C move down

for _, c := range s.All() {
	fmt.Print(c.Text(), " ")
}
// Output: B C A
```

- **Equal starts**: with A at 1 s, B at 2 s and C at 3 s, `SetCue(2, C at 2 s)` and `SetCue(0, A at 2 s)` both give A, B, C. The cue with the lower index stays first.
- **Index drift**: a cue index is valid until the next change. After `SetCue`, `AddCue` or `RemoveCue`, the same index can point to another cue. Look the cue up again, for example with `CuesWhere`.
- **Added cues**: `AddCues` sorts once with a stable sort, so an added cue goes after the stored cues with the same start.

## Comments

A comment is text that a format writes before a cue, such as a WebVTT `NOTE` block. `Comment.BeforeCue` is the index of that cue. The cue count or more means after the last cue.

<!-- example: ExampleSubtitle_AddComment -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 1 * time.Second, Lines: []string{"first"}},
	model.Cue{Start: 2 * time.Second, Lines: []string{"second"}},
)
_ = s.AddComment("NOTE before the second cue", 1)

s.AddCue(model.Cue{Start: 0, Lines: []string{"new first"}}) // the second cue moves to index 2

fmt.Println(s.Comments())
// Output: [{NOTE before the second cue 2}]
```

- **Reorder**: a comment stays before the same cue through every sort and move.
- **Removed cue**: a comment before a removed cue comes before the next kept cue, or after the last cue.
- **Index above the count**: `AddComment` stores it as is. Writers put the comment after the last cue. A later change that adds, moves or removes a cue sets it to the cue count, except `ReplaceCues` without an anchor.
- **Negative index**: `AddComment` returns an error that matches `ErrInvalidArgument`.
- **ReplaceCues with an anchor**: the anchor maps each old cue index to an index in the new list, and each comment follows its cue there. A comment after the last old cue stays after the last cue.
- **ReplaceCues without an anchor**: `BeforeCue` counts as an index into the new list. Example: a comment at index 3 of a 3-cue subtitle comes before new cue 3 after `ReplaceCues` with 5 cues. A reader that adds the comments before the cues needs this rule.

## Metadata, alignment and forced cues

- **Metadata**: free string keys, with the constants `MetadataTitle`, `MetadataAuthor`, `MetadataArtist`, `MetadataAlbum` and `MetadataLanguage`. An empty value is a value. `DeleteMetadata` removes a key.
- **Alignment**: 9 places on a numeric keypad grid. The zero value is `BottomCenter`. `Numpad()` gives the keypad key, for example 8 for `TopCenter` as in the ASS tag `{\an8}`. Library JSON stores this number. A value outside the 9 constants counts as `BottomCenter`.
- **Forced**: a forced cue is a forced narrative. The player shows it also when the viewer turned subtitles off.
- **ID**: the empty string means the cue has no identifier.

## Lookups

| Method | Returns |
|:--- |:--- |
| `CuesAt(t)` | the cues with `Start <= t < End`. A cue with `Start == End` is never on screen |
| `CueIndexAt(t)` | the lowest index of `CuesAt(t)`, or false |
| `CuesBetween(from, to)` | the cues with `Start < to` and `End > from`, uncut. `from > to` returns an error that matches `ErrInvalidArgument` |
| `CuesWhere(match)` | the cues for which `match` returns true |

- **Indexes**: each lookup returns ascending cue indexes. Read each cue with `Cue(i)`.
- **Speed**: the first lookup after a change builds an index of the cue times. Later lookups use it until the next change.
- **Concurrency**: a `Subtitle` is not safe for concurrent use. A lookup writes the index, so concurrent lookups need a lock too.

## Copies and equality

- **Clone**: a deep copy. A change to the copy, including its format data, never changes the original.
- **Equality**: package `model` has no `Equal` method. Compare `Cues()`, `Comments()`, `AllMetadata()` and the `FormatData` of each key in `FormatDataKeys()`.
- **Format data**: per-format data on the subtitle and on each cue, see [format-data.md](format-data.md).
