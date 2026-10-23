# Format detection

`stbox.Detect` picks the format of a file from its content. `Load` and `Parse` call it when `ReadOptions.Format` is empty. Example: `stbox.Load("notes.srt", stbox.ReadOptions{})` reads a file that starts with `WEBVTT` as WebVTT. The content decides before the extension, because some extensions belong to 2 or more formats. `.sub` is MicroDVD and SubViewer, and a download often has no name at all.

## Steps of Detect

1. Remove all leading UTF-8 BOMs. Then remove the leading bytes space, tab, LF, CR, NUL and VT.
2. Content that starts with `{` and decodes as a JSON object goes to the JSON checks only. Their result is final, also when no check matches. Content that is not valid JSON goes on to step 3.
3. Turn each run of CRs before an LF into one LF, and each other CR into LF.
4. Try the signatures in order. The first match wins. No match gives no result.

- **Format packages**: each `Detect` of a format package gets the content after steps 1 and 3. So a SubRip file with a BOM and CR LF line endings detects.
- **Bad UTF-8 in JSON**: `encoding/json` reads each bad byte in a JSON string as U+FFFD, so the decode goes on.
- **Never detected**: `Detect` returns only formats whose `IsAutoDetected` is true.

## JSON checks

| # | Format | Rule |
|---:|:--- |:--- |
| 1 | library JSON | `version` is a JSON number and `cues` is a JSON array |

`{"version": "1", "cues": []}` and `{"version": 1, "cues": {}}` match no check. `stboxjson.DetectJSON` holds rule 1.

## Signatures

| # | Format | Rule |
|---:|:--- |:--- |
| 1 | WebVTT | `WEBVTT`, then a space, a tab, LF or the end |
| 2 | TTML | the XML prolog, then `<tt` or `<prefix:tt`, then white space, `>` or `/`. The prolog is any run of white space, `<?...?>`, `<!--...-->` and `<!DOCTYPE...>` without `>` inside |
| 3 | SAMI | the XML prolog of rule 2, then `<SAMI` and white space or `>`. Both are in any case, also `<!DOCTYPE` |
| 4 | ASS | the first line is `[Script Info]` in any case, plus optional spaces or tabs |
| 5 | MPSub | the content starts with 1 or more ASCII capitals and `=`, and some line starts with `FORMAT=`, the first line too |
| 6 | MicroDVD | `{digits}{digits or nothing}` at the start, such as `{24}{72}` or `{24}{}` |
| 7 | SubRip | a cue number and a timing line. Or a timing line with 2 or more hour digits and a comma, alone or after up to 5 lines of text and a cue number |
| 8 | SBV | the first line is `H:MM:SS.mmm,H:MM:SS.mmm` plus optional spaces or tabs |
| 9 | LRC | an LRC time tag or an ID tag such as `[ti:Rain]` at the start, and a line that starts with an LRC time tag after optional spaces or tabs |
| 10 | MPL2 | `[digits][digits]` at the start |
| 11 | TMPlayer | `H:MM:SS` with minutes and seconds up to 59, an optional `,` and digits, then `=` or `:`. A `:` does not count when 1 or 2 digits, spaces or tabs and a comma follow it, or digits `:dd:dd` |

- **ASS header**: `ass.Detect` holds rule 4. A file that starts with a comment line before `[Script Info]` is not detected, but the ASS reader reads it.
- **MPSub**: `mpsub.Detect` holds rule 5. A file with headers but no `FORMAT=` line, such as `TITLE=Bakery` and `AUTHOR=Jane Doe`, is not detected, but the MPSub reader reads it. MPSub files often use the extension `.sub`, which `FormatFromPath` maps to MicroDVD. The content decides first, so `Load` reads them as MPSub.
- **MicroDVD**: `microdvd.Detect` holds rule 6. It accepts an empty end frame, but the reader needs digits, so `{24}{}Hello` detects as MicroDVD and then fails.
- **LRC time tag**: `[`, optional hours and `:`, minutes of 1 to 3 digits, `:`, seconds `00` to `59`, an optional `.` and 1 to 3 digits, `]`. Spaces or tabs may stand inside the brackets. The hours count only when 1 or 2 minute digits up to 59 and `:` follow them, so `[1:75:00.00]` and `[1234:45.67]` are no time tags. `lrc.Detect` holds rule 9, and its doc comment states it.
- **TMPlayer and frame timecodes**: the two exceptions of rule 11 keep out the `hh:mm:ss:ff` frame timecodes of Spruce STL and CSV, such as `00:00:01:10,00:00:03:10,"Hello"`. The text `00:00:01:5 trains leave` still matches. `tmplayer.Detect` holds the rule, and its doc comment states it.
- **Order**: WebVTT comes before SubRip, because a WebVTT file without its header looks like SubRip. A new format inserts its signature at its place in the PHP order. `TestDetectionListsFollowTheOrder` in [registry_test.go](../registry_test.go) holds that order.
- **SubRip hours and comma**: a timing line without a cue number on the line before it needs 2 hour digits and a comma. So a headerless WebVTT file that starts with `00:00:01.000 -->` and SBV with `0:00:01,500` do not match. A headerless WebVTT file that starts with a cue number, such as `1` LF `00:00:01.000 -->`, detects as SubRip.
- **SBV fraction**: SBV has 3 fraction digits and SubViewer has 2, so `0:00:01.5,0:00:04.000` is no SBV signature. `sbv.Detect` holds rule 8.

## Load and Parse without a format

| Step | Rule |
|---:|:--- |
| 1 | **Video file**: content that starts with a Matroska, MP4 or MPEG-TS head returns an error that matches `ErrUnsupportedFormat`: `movie.mkv is a Matroska file. This version reads no video files.` An MP4 file gives `is an MP4 file`, an MPEG-TS file `is an MPEG-TS file`. `Parse` says `The content is a Matroska file.` This step runs with a format too |
| 2 | **Decode** the content to UTF-8, as [encodings.md](encodings.md) says |
| 3 | **Empty content**: nothing is left after the BOMs and the bytes of step 1 of `Detect` go, at both ends. `Load` returns an empty subtitle in the format of the extension, under the fallback rule of step 5 |
| 4 | **Detect** the format of the decoded content |
| 5 | **Extension fallback**: without a detection result, `Load` uses `FormatFromPath`, when the library can read that format and the extension is not `json`, `txt` or `ffmeta`. CSV and TSV have no signature, so `.csv` and `.tsv` files load only by this step |
| 6 | **No format**: an error that matches `ErrUnknownFormat`, with `Format detection found no subtitle format.` `Parse` has no path, so it skips steps 3 and 5 |

| Head | Bytes |
|:--- |:--- |
| Matroska and WebM | `1A 45 DF A3` at byte 0 |
| MP4 | `ftyp` at bytes 4 to 7 |
| MPEG-TS | `47` at bytes 0, 188 and 376 |
| M2TS, MPEG-TS with 192-byte packets | `47` at bytes 4, 196 and 388 |

- **Fixed fallback list**: the extensions `json`, `txt` and `ffmeta` also belong to formats of later releases that detection never finds, such as cloud speech-to-text JSON and chapters. So these extensions say nothing about the content. The list stays fixed for the whole 1.x line, so a `.json` file gives the same result in every release.
