# Contributing

## Workflow

- Run `make ci` before you push.
- Each pull request carries one release label: `major`, `minor`, `patch` or `skip-release`. Before v1.0.0 every pull request carries `skip-release`. Only the 1.0.0 pull request (#43) carries `major`.
- The pull request title becomes the squash commit subject. Write it as a release note line: imperative, for users, one line. Example: "Add the SBV format, read and write".
- The pull request body ends with a line `Release note: ...` in the same style.

## Packages

A package imports only packages of a lower layer. Every layer may also import the standard library and the allowed modules of [CLAUDE.md](CLAUDE.md), which also lists the package map. `internal/apicheck/layers_test.go` holds the same table. It fails for a forbidden import and for a package without a layer, so the issue that adds a package adds its row in both places.

| Layer | Packages | May import |
|---:|:--- |:--- |
| 0 | leaf helpers in `internal/`: `markupcore`, `timecode`, `grapheme`, `ssatag`, `textenc`, `xmlraw`, the test helpers `testfiles` and `golden`, later `codepage`, `cea608`, `cea708`, `ebml`, `orderedjson` | other layer 0 packages |
| 1 | `markup` | layer 0 |
| 2 | `model` | layers 0 and 1 |
| 3 | `internal/` helpers that work on model types: `blocks`, `subrip`, `vtt`, `position`, `parsing`, `writing`, later `caption`, `ttmlwrite`, `wordgroup` | layers 0 to 2 |
| 4 | `format/<name>` | layers 0 to 3. A format package imports another format package only when its format is a profile of the other, for example iTT of TTML. Other shared code goes to `internal/` |
| 4b | `container/<name>`, from 3.x | layers 0 to 4. A container returns cues, so it calls the format readers |
| 5 | root `stbox` and the service packages | layers 0 to 4b. A service never imports the root. The root never imports a service |
| 6 | `internal/cli`, `cmd/stbox` | everything |

### Format packages

Every format package declares `ReadOptions` and `WriteOptions`, empty where the format has no options. Its functions always take them, so a first option is a new field and never a signature change. A format without a reader has no `Read`. A format without a signature has no `Detect`.

```go
// Read reads UTF-8 text. The root decodes other encodings first.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error)
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error
// Detect gets the content without leading BOMs or white space, with LF line endings.
func Detect(content []byte) bool
```

- **Other formats**: a reader ignores the option fields of other formats, so one `ReadOptions` value serves a batch of mixed files.
- **Format data**: an option wins over stored format data, and stored format data wins over the format default.

### Format registry

The registry is written by hand, and no package registers itself in `init()`. The facts table and the functions table hold the same rows in the same order. The rows follow the PHP row order, because the first row that lists an extension owns it in `FormatFromPath`. A later line inserts its rows at the PHP position.

| Table | File | Holds |
|:--- |:--- |:--- |
| facts | `model/format.go` | extensions, can read, can write, auto-detected, binary. The `Format` methods read it |
| functions | `registry.go` | the read and write function of each format |
| detection lists | `registry.go` | the JSON checks and the signatures, in detection order. First match wins |

Every 1.x row and `Format` constant exists from 1.0.0, so `FormatFromPath` gives the same result for the whole 1.x line. The issue that adds a format reader or writer:

1. Sets can read, can write and auto-detected in its facts row.
2. Sets the `read` and `write` functions of its row in `registry.go`.
3. Inserts its matcher into `signatures` or `jsonChecks`. `TestDetectionListsFollowTheOrder` in `registry_test.go` holds the order.
4. Adds a field of its options type to the root `ReadOptions` or `WriteOptions` when the type is not empty. TSV uses the `CSV` field.
5. Adds a compile-time shape check in the root for each of its `Read`, `Write` and `Detect`, for example `var _ readFunc[srt.ReadOptions] = srt.Read`. The first format issue declares the generic types:

   ```go
   type readFunc[O any] func(io.Reader, model.CommonReadOptions, O) (*model.Subtitle, []model.ParseWarning, error)
   type writeFunc[O any] func(io.Writer, *model.Subtitle, model.CommonWriteOptions, O) error
   type detectFunc func([]byte) bool
   ```

## Credit for ported work

- **Copied file**: add it to the section of its project in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md), or add the section. Add an Apache-2.0 `NOTICE` file too when the project has one.
- **Ported or adapted code**: add a credit line to the comment of the type, function or table, even when the Go code looks different. Pin the link to a commit. For an exported identifier, the doc sentence comes first, then an empty `//` line, then the credit.

  ```go
  // ReadOptions holds the settings of the Spruce STL reader.
  //
  // Adapted from mantas-done/subtitles, src/Code/Converters/StlConverter.php
  // (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/src/Code/Converters/StlConverter.php), MIT.
  type ReadOptions struct {
  ```

  For an unexported identifier, the credit can stand alone. When a whole package ports one upstream file, the credit goes in the package doc comment.
- **Wording**: write "Adapted from" for the same logic written again. Write "Copied from" for verbatim code or data. Use exactly these two forms, so `grep -rn "Adapted from\|Copied from"` finds every credit.
- **Apache-2.0**: add a line `// Changed: ...` that states what changed.
- **Upstream test data**: a test case with data from another project names the project in its case name, for example `name: "mantas-done styles"`. A ported test case cites the upstream test method in a comment, in the same credit form.
- **Fixtures**: every file under a `testdata` folder has a row in a `SOURCES.md` of its folder or of a parent folder, up to the `testdata` folder. A row is `| File | Source | License |`. The first cell holds a file name, a glob or a comma list of both, relative to the folder of the `SOURCES.md`, each in backticks. `TestEveryFixtureHasASourcesRow` and `TestEverySourcesRowNamesAFile` in `repo_test.go` check this.
- **Fuzz corpus**: files under `testdata/fuzz/` are exempt. `go test -fuzz` writes them, and they are this repository's own data under MIT.
- **Re-hosted data**: a project that re-hosts the data of another project gets a credit for the original source too. Example: the IRT EBU STL files that sandflow/ttconv took from irt-open-source/scf.
- **Files from the PHP repository**: a `SOURCES.md` copied from yama6a/subtitle-toolbox-php keeps its rows unchanged. It gets one line under its heading: `Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/<folder>, MIT. In the rows below, "this repository" means subtitle-toolbox-php.` Rows for files that are not copied are removed.
- **Code from the PHP repository**: Go code ported from yama6a/subtitle-toolbox-php carries no credit line. Both repositories have the same owner and the same MIT notice. The third-party credits inside a ported PHP file move to the Go code.
- **Linked modules**: the binary `stbox` contains every module it imports. A pull request that adds such a module to `go.mod` adds a section `## <module path>` with the `LICENSE` text of the module at that version. `TestEveryLinkedModuleHasANotice` checks this.

## Licence rules

- Permissive sources may be adapted, always with credit.
- Copyleft, proprietary and unlicensed sources are behaviour references only. Read the behaviour or the spec, write your own code and tests, and copy nothing.
- A project not in the table: check its licence at the pinned commit before you use anything. An unknown or missing licence means ideas only.

| Project | Licence | Use allowed |
|:--- |:--- |:--- |
| mantas-done/subtitles | MIT | adapt with credit |
| captioning/captioning | MIT | adapt with credit |
| podlove/webvtt-parser | MIT | adapt with credit |
| hartman/vtt-vivid | Apache-2.0 OR MIT. Its test data from mozilla/vtt.js is Apache-2.0 | adapt with credit. The vtt.js data needs its Apache-2.0 NOTICE |
| delphiki/SubRip-File-Parser | BSD | adapt with credit |
| Saeven/subtitles, ismailceylan/php-subtitle-toolkit, medienbaecker/kirby-vtt, Thijs-Riezebeek/subtitles | MIT | adapt with credit |
| benlipp/srt-parser | DBAD, permissive with attribution | adapt with credit |
| johnnoel/php-ass | CC-BY-SA-3.0 | behaviour only |
| iceman1010/srt-translation-validator | proprietary, no redistribution | ideas only, nothing copied |
| subtitle.js, subsrt, media-captions of vidstack, osk/node-webvtt | MIT | adapt with credit |
| shaka-project/shaka-player | Apache-2.0 | adapt with credit, keep the licence text, state changes |
| pysubs2, srt by cdown | MIT | adapt with credit |
| pycaption by PBS | Apache-2.0 | adapt with credit, keep the NOTICE |
| sandflow/ttconv | BSD-2-Clause. The STL files in `src/test/resources/stl/irt/` are Apache-2.0 with a NOTICE, checked at `1fed6ab` | adapt with credit. The IRT files need their NOTICE |
| go-astisub, martinlindhe/subtitles, libgosubs | MIT | adapt with credit |
| androidx/media (Media3) | Apache-2.0 | adapt with credit, keep the NOTICE, state changes |
| JDaren/subtitleConverter | MIT per its README | adapt with credit |
| noophq/subtitle | LGPL-3.0 | behaviour only |
| Subtitle Edit (libse, seconv) | MIT at every commit whose `LICENSE` file is MIT. The switch to MIT is commit `771236c5` of 2026-02-28. Older commits are GPL-3.0 | adapt with credit. Check the `LICENSE` file at the pinned commit first |
| AlexPoint/SubtitlesParser | MIT | adapt with credit |
| kitsumed/SubtitlesParserV2 | LGPL-3.0 | behaviour only |
| Aegisub | BSD-3 or ISC per file header. Some files differ | adapt with credit only if the file header is BSD or ISC. Otherwise behaviour only |
| KDE Subtitle Composer | GPL-2.0-or-later | behaviour only |
| MKVToolNix | GPL-2.0 | behaviour only |
| FFmpeg | LGPL-2.1 or later, some parts GPL | behaviour only |
| libass | ISC | adapt with credit |
| CCExtractor | GPL-2.0 | behaviour only |
| subparse, Rust | MPL-2.0, copyleft per file | behaviour only |
| rsubs-lib | MIT | adapt with credit |
| xqq/libaribcaption | MIT. The source files carry an ISC-style notice. Checked at `37dde0f` | adapt with credit |
| aribb24 | LGPL-3.0 | behaviour only |
| aspasia | 0BSD | adapt, credit appreciated |
| chardetng, used by aspasia | Apache-2.0 OR MIT, Mozilla Foundation, checked at `4a53a99` | adapt with credit under MIT |
| alass | GPL-3.0 | behaviour only |
| webvtt-ruby, srt for Ruby, subtitle_it | MIT | adapt with credit |
| SubTypo, submerger, subtitle_app_utils, Kotlin | GPL-3.0 | behaviour only |
| SwiftSubtitles, SwiftWebVTT | MIT | adapt with credit |
| swift-subtitle-kit | MIT, Dionysis Karatzas 2026, checked at `728d62d` | adapt with credit. Its `sample.*` fixtures are copies of the subsrt files, so credit subsrt. `embedded-bom-srt-fixture.srt` is its own |
| LibreTranslate, server | AGPL-3.0, checked at `54a66e8` | behaviour only. A client for its HTTP API is our own code. Never copy server code |
| KBlixt/subcleaner | no licence found | ideas only |
| BYVoid/OpenCC | Apache-2.0, no NOTICE file at `8cf737a` | adapt with credit, keep the licence text, state changes |
| Liberation Fonts | SIL OFL 1.1 | redistribute with the licence text |
| Podcastindex-org/podcast-namespace examples | CC0-1.0 | free to use. Credit is a courtesy |
| W3C specs and examples | W3C Software and Document License | examples may be used with the W3C notice |
| ISO/IEC, 3GPP, SMPTE, EBU, CEA specs, Adobe docs | specifications | implement from the spec. No code exists to copy |
| yama6a/subtitle-toolbox-php, yama6a/php-glyph-ocr | MIT, same owner | port freely. Third-party credits in a ported file move with the code |
| The Go standard library | BSD-3-Clause | adapt with credit |
| `golang.org/x/text`, `github.com/rivo/uniseg`, `github.com/Eyevinn/mp4ff`, `github.com/asticode/go-astits`, `github.com/dlclark/regexp2` | the `LICENSE` file of each module at the version in `go.mod` | import only. The binary links them, so each has a section in `THIRD_PARTY_NOTICES.md` |

## API rules

Every exported identifier is a promise to importers until the next major release. Reviewers check each PR against these rules. [docs/compatibility.md](docs/compatibility.md) states what a release may change.

`internal/apicheck/names_test.go` checks part of the rules on exported names:

- No name starts with `Get` or `Find` (N5).
- No name holds `Colour`, `Fps`, `Secs`, `Seconds` or `Millis` (N4 and the time rule).
- Each exported field of an options struct has a doc comment (N20). An options struct is a struct whose name ends in `Options`, or a struct with a `Validate` method.

Its exception table names the rule and the reason for each exception.

### General rules

| Topic | Rule |
|:--- |:--- |
| Time | Every time and duration in the API is a `time.Duration`. Parsers and setters round it to whole milliseconds with `d.Round(time.Millisecond)`, which rounds half away from zero. Example: 1.0005 s becomes 1.001 s. No name carries a unit: `LastCueDuration`, not `LastCueSeconds` |
| Options | One `ReadOptions` and one `WriteOptions` in the root package, with one typed field per format |
| Logging | None. The library never writes to stdout, stderr or a logger. It returns warnings and reports. The CLI prints plain messages to stderr. This is a documented exception to the org rule "Go repos log with zap" |
| Majors | A break lands only in the issue that opens a major line. The major release moves the module path to `/vN` |
| Errors | Wrap with `%w`. Sentinels `ErrX` live in `errors.go`. Error types `XError` have an `Unwrap` method when the type has an `Err` field. Interfaces get compile-time checks |
| Comments | A doc comment on each exported identifier, one sentence that starts with the name |
| Go version | `go 1.27.2` in `go.mod`, no `toolchain` line. Renovate raises the `go` line |

### Naming rules

| # | Rule | Example |
|---:|:--- |:--- |
| N1 | A `Format` value is the CLI format name. Format data keys use the same value. No aliases | `"srt"`, `"vtt"`, `"ass"`, `"ttml"`, `"sami"`, `"sbv"`, `"microdvd"`, `"mpl2"`, `"tmplayer"`, `"mpsub"`, `"subviewer"`, `"qttext"`, `"lrc"`, `"csv"`, `"tsv"`, `"json"`, `"txt"`. `sub`, `smi` and `ssa` are extensions, never format names |
| N2 | A format constant carries the format's own name. A format package has the lower-case name from the package map | `SubRip`, `WebVTT`, `ASS`, `TTML`, `SAMI`, `SBV`, `MicroDVD`, `MPL2`, `TMPlayer`, `MPSub`, `SubViewer`, `QuickTimeText`, `LRC`, `CSV`, `TSV`, `JSON`, `PlainText`. Packages `srt`, `webvtt`, `ass` |
| N3 | Initialisms are all capitals, as Go style and the `revive` linter want. Brand names keep their own capitals | `ID`, `URL`, `BOM`, `CRLF`, `PNG`, `OCR`. `SubRip`, `WebVTT`, `MicroDVD` |
| N4 | Full words and US spelling | `FrameRate`, not `FPS`. `MaxLinesPerCue`, not `MaxLines`. `Color`, not `Colour` |
| N5 | A lookup that can miss returns `(T, bool)`. An operation that can fail returns `error`. No `Get` and no `Find` prefix | `FormatFromPath("movie.mkv")` returns `"", false`. `s.CueIndexAt(t)` returns `(int, bool)` |
| N6 | A method that changes a `*Subtitle` is a verb and never returns a `*Subtitle`. A function or method that returns a `*Subtitle` never changes its input | `s.Shift(2 * time.Second)` changes `s`. `s.Clone()` and `s.Slice(from, to)` return a new subtitle |
| N7 | A time range is always `From` and `To`. A tolerance is always `TimeTolerance` | `s.CuesBetween(from, to)`, `s.MergeSameTimeCues(timeTolerance)` |
| N8 | `Append` adds the cues of another subtitle. `Merge` and `Join` combine cues inside one subtitle | `s.Append(other, offset)`, `s.JoinCues(3, 4)`, `s.MergeShortCues(opts)` |
| N9 | Every closed set of values is a named type with constants. Its zero value is the default and has a name. A set whose values appear in JSON, the CLI or format data is a `string` type with exactly those values. Other sets use `uint8` and `iota`, default first. Each has a `String()` method | `Alignment` zero is `BottomCenter`, and `Numpad()` gives 1 to 9 for JSON. `WarningAction` has `"skipped"` and `"repaired"` |
| N10 | A setting that may get a third value is an enum from the start, never a `bool` | ellipsis style: none, three dots, the U+2026 character |
| N11 | Reports and results are structs with named fields. No `map[string]any`, no lists of pairs | `[]WordCount{Word, Count}`, not `map[string]int` |
| N12 | A service is one package with `Apply(s *model.Subtitle, opts Options) (Report, error)`, or a few functions. It changes the subtitle in place | `hearingimpaired.Apply(s, hearingimpaired.Options{})` |
| N13 | Plain data uses exported fields: options, reports, results, `Comment`. A type whose values must keep an invariant hides the field and offers methods | `Subtitle` keeps its cues in start order, so its cue list is not an exported field |
| N14 | Code that users must not call goes in `internal/` or stays unexported. No exported "for internal use" identifiers | `internal/blocks`, `internal/textenc` |
| N15 | Export an interface only when users implement it. Return concrete types, as the `ireturn` linter wants | OCR engine (3.x), translation engine and HTTP client (4.x) |
| N16 | A function that starts a process or a network request takes `ctx context.Context` first. A file read does not | `ocr/tesseract`, `translation/deepl` |
| N17 | Bad input and bad options return an error. A panic is a library bug. No `Must` functions | a fuzz target checks every parser |
| N18 | No mutable package state and no `init`. A lookup table at package level carries `//nolint:gochecknoglobals // <reason>` | the format registry |
| N19 | Each options struct has `Validate() error`. Every entry point calls it before it reads or writes | `ReadOptions.Validate()` returns an error that matches `ErrInvalidArgument` |
| N20 | The zero value of an option is its default. A bool is named so that false is the default. A setting with 3 states is an enum. A pointer appears only where 0 is legal and differs from the default. The doc comment of an option field states what its zero value does and the unit | `OmitHeader`, `BOM`. The 2 pointers of 1.x are `LastCueDuration` and the plain text `ParagraphGap`. `// LastCueDuration is the length of a last cue without an end. Nil means 5 s.` |
| N21 | A minor release may mark an identifier `// Deprecated: use X.`. Only the next major release removes it | |
