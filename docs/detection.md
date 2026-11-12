# Format detection

`stbox.Detect` picks the format or the video container of a file from its content. `Load` and `Parse` call it when `ReadOptions.Format` is empty. Example: `stbox.Load("notes.srt", stbox.ReadOptions{})` reads a file that starts with `WEBVTT` as WebVTT. The content decides before the extension, because some extensions belong to 2 or more formats. `.sub` is MicroDVD and SubViewer, and a download often has no name at all.

## Steps of Detect

Before step 1, `Detect` looks for a video container in the raw bytes. A container wins over every format and returns a `Detection` with `Container` set and `Format` empty. A subtitle file returns a `Detection` with `Format` set. The container head needs `ContainerHeadSize` bytes, 389, so pass at least that many to find a container.

1. **Raw signatures**: the formats with a binary header that steps 2 to 4 would change check the bytes as they are. A match is final. PGS is the only one. Its header holds a 32-bit PTS at bytes 2 to 5 and a 32-bit DTS at bytes 6 to 9, which can contain CR, LF or bytes that step 2 removes, and its segment type is byte 10. `pgs.Detect` checks `PG` at byte 0 and one of `14h`, `15h`, `16h`, `17h` and `80h` at byte 10.
2. Remove all leading UTF-8 BOMs. Then remove the leading bytes space, tab, LF, CR, NUL and VT.
3. Content that starts with `{` and decodes as a JSON object goes to the JSON checks only. Their result is final, also when no check matches. Content that is not valid JSON goes on to step 4.
4. Turn each run of CRs before an LF into one LF, and each other CR into LF.
5. Try the signatures in order. The first match wins. No match gives no result.

- **Format packages**: each `Detect` of a format package gets the content after steps 2 and 4. So a SubRip file with a BOM and CR LF line endings detects.
- **Bad UTF-8 in JSON**: `encoding/json` reads each bad byte in a JSON string as U+FFFD, so the decode goes on.
- **PGS**: a BOM or white space before the header hides the PGS signature, because step 1 reads the raw bytes. A `.sup` file has none. `Load` also reads a file with the extension `.sup` as PGS when no signature matches, because only PGS lists `.sup`.
- **Never detected**: `Detect` returns only formats whose `IsAutoDetected` is true.
- **No signature**: Cavena 890 has none, so `Detect` finds no result for a `.890` file. `Load` reads it by its extension before any decoding, step 3 below. `Parse` without `ReadOptions.Format` returns `ErrUnknownFormat`.

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
| 3 | DCP subtitles | the XML prolog of rule 2, then `<DCSubtitle` or `<SubtitleReel` with or without a prefix such as `<dcst:SubtitleReel`, then white space, `>` or `/` |
| 4 | SAMI | the XML prolog of rule 2, then `<SAMI` and white space or `>`. Both are in any case, also `<!DOCTYPE` |
| 5 | ASS | the first line is `[Script Info]` in any case, plus optional spaces or tabs |
| 6 | MPSub | the content starts with 1 or more ASCII capitals and `=`, and some line starts with `FORMAT=`, the first line too |
| 7 | MicroDVD | `{digits}{digits or nothing}` at the start, such as `{24}{72}` or `{24}{}` |
| 8 | QuickTime text | `{QTtext}` at the start, in any case |
| 9 | SubRip | a cue number and a timing line. Or a timing line with 2 or more hour digits and a comma, alone or after up to 5 lines of text and a cue number |
| 10 | SBV | the first line is `H:MM:SS.mmm,H:MM:SS.mmm` plus optional spaces or tabs |
| 11 | SubViewer | a line `******** START SCRIPT ********` anywhere, or `[INFORMATION]` as the first line, or lines that start with `[`, then a SubViewer 2 timing line with exactly 2 fraction digits, such as `00:00:01.50,00:00:04.00`. Each line may end in spaces or tabs |
| 12 | LRC | an LRC time tag or an ID tag such as `[ti:Rain]` at the start, and a line that starts with an LRC time tag after optional spaces or tabs |
| 13 | EBU STL | 3 ASCII digits, then `STL25.01` or `STL30.01`, at the start. A byte check |
| 14 | PAC | the bytes `01h` and 20 bytes `00h`, 2 bytes of the subtitle number and a record mark from `60h` to `67h`. A byte check |
| 15 | Spruce STL | `$` or `//` lines, then a cue line. Or a cue line whose text does not start with a double quote. A cue line is `hh:mm:ss:ff`, a comma, `hh:mm:ss:ff` and a comma, with exactly 2 hour digits, minutes and seconds up to 59 and optional spaces or tabs around the commas |
| 16 | SCC | the first line is `Scenarist_SCC V1.0` plus optional spaces or tabs |
| 17 | MCC | the first line is `File Format=MacCaption_MCC V<n>.<n>` plus optional spaces or tabs |
| 18 | MPL2 | `[digits][digits]` at the start |
| 19 | TMPlayer | `H:MM:SS` with minutes and seconds up to 59, an optional `,` and digits, then `=` or `:`. A `:` does not count when 1 or 2 digits, spaces or tabs and a comma follow it, or digits `:dd:dd` |

- **DCP subtitles**: `dcp.Detect` holds rule 3. It comes after TTML, because the roots differ, and before SAMI. DCP subtitle files use the extension `.xml`, which TTML owns in `FormatFromPath`. So `Load` finds them by this rule, and a write needs the format `dcp`.
- **ASS header**: `ass.Detect` holds rule 5. A file that starts with a comment line before `[Script Info]` is not detected, but the ASS reader reads it.
- **MPSub**: `mpsub.Detect` holds rule 6. A file with headers but no `FORMAT=` line, such as `TITLE=Bakery` and `AUTHOR=Jane Doe`, is not detected, but the MPSub reader reads it. MPSub files often use the extension `.sub`, which `FormatFromPath` maps to MicroDVD. The content decides first, so `Load` reads them as MPSub.
- **SubViewer**: `subviewer.Detect` holds rule 11. It comes after SBV and before LRC, which also matches `[00:00:01]`. A SubViewer 2 file without `[INFORMATION]` and without a timing line after its `[` lines is not detected, but the reader reads it. A SubViewer 1 file without a script line is not detected either. SubViewer files often use the extension `.sub`, which `FormatFromPath` maps to MicroDVD. The content decides first, so `Load` reads them as SubViewer.
- **Spruce STL**: `sprucestl.Detect` holds rule 15. It comes after LRC and before MPL2. A cue line with a quote after its times, such as `00:00:01:10,00:00:03:10,"Hello"`, is a headerless CSV row. It detects as Spruce STL only after a `$` or `//` line. The reader takes 2 or more hour digits, but the signature needs exactly 2, so a file whose first cue starts at hour 100 is not detected. Spruce STL files use the extension `.stl`, which EBU STL uses too.
- **QuickTime text**: `qttext.Detect` holds rule 8. A file without the `{QTtext}` line is not detected as QuickTime text. It detects as LRC, because `[00:00:01.00]` is an LRC time tag. Pass the format `qttext` with the lenient option to read it.
- **MicroDVD**: `microdvd.Detect` holds rule 7. It accepts an empty end frame, but the reader needs digits, so `{24}{}Hello` detects as MicroDVD and then fails.
- **LRC time tag**: `[`, optional hours and `:`, minutes of 1 to 3 digits, `:`, seconds `00` to `59`, an optional `.` and 1 to 3 digits, `]`. Spaces or tabs may stand inside the brackets. The hours count only when 1 or 2 minute digits up to 59 and `:` follow them, so `[1:75:00.00]` and `[1234:45.67]` are no time tags. `lrc.Detect` holds rule 12, and its doc comment states it.
- **TMPlayer and frame timecodes**: the two exceptions of rule 19 keep out the `hh:mm:ss:ff` frame timecodes of Spruce STL and CSV, such as `00:00:01:10,00:00:03:10,"Hello"`. The text `00:00:01:5 trains leave` still matches. `tmplayer.Detect` holds the rule, and its doc comment states it.
- **Binary signatures**: `Load` and `Parse` run `Detect` on the raw bytes before they decode. A binary result reads the raw bytes.
- **PAC**: `pac.Detect` holds rule 14. It comes after EBU STL and before Spruce STL, in the PHP order. The check reads the first 24 bytes of the content after steps 2 and 4. So a first subtitle number whose bytes are `0Dh 0Ah` is not detected, because step 4 turns them into one LF. PHP has the same limit. A file with such a number still loads by the extension `.pac` or `.fpc`.
- **SCC**: `scc.Detect` holds rule 16. The SCC header is a line of its own, so it needs no rule against other formats. It comes after Spruce STL and before MPL2, in the PHP order.
- **MCC**: `mcc.Detect` holds rule 17. It accepts any version, such as `V3.0`, and the reader rejects all but 1.0 and 2.0. It comes after SCC and before MPL2, in the PHP order.
- **Order**: WebVTT comes before SubRip, because a WebVTT file without its header looks like SubRip. A new format inserts its signature at its place in the PHP order. `TestDetectionListsFollowTheOrder` in [registry_test.go](../registry_test.go) holds that order.
- **SubRip hours and comma**: a timing line without a cue number on the line before it needs 2 hour digits and a comma. So a headerless WebVTT file that starts with `00:00:01.000 -->` and SBV with `0:00:01,500` do not match. A headerless WebVTT file that starts with a cue number, such as `1` LF `00:00:01.000 -->`, detects as SubRip.
- **SBV fraction**: SBV has 3 fraction digits and SubViewer has 2, so `0:00:01.5,0:00:04.000` is no SBV signature. `sbv.Detect` holds rule 10.

## Load and Parse without a format

| Step | Rule |
|---:|:--- |
| 1 | **Video file**: `Load` and `Parse` read the first `ContainerHeadSize` bytes only. A Matroska, MP4 or MPEG-TS head goes to the container reader and its track rules. A version without the reader for that container returns an error that matches `ErrUnsupportedFormat`: `movie.mp4 is an MP4 file. This version cannot read MP4 files.` `Parse` says `The content is an MP4 file.` With a reader and `Format` set, the error is `movie.mkv is an MKV or WebM file. Read it by track.` The row check comes first. This step runs with a format too |
| 2 | **Binary by content**: `Detect` on the raw bytes, for a format only. A binary format reads the raw bytes. `Encoding` is ignored and `SourceEncoding` is empty |
| 3 | **Binary by extension**: only `Load`. When every format that lists the extension is binary, the extension decides before any decoding. `.pac`, `.fpc` and `.890` are such extensions. `.stl` is not, because Spruce STL lists it too. Blank content gives an empty subtitle, as in step 5 |
| 4 | **Decode** the content to UTF-8, as [encodings.md](encodings.md) says |
| 5 | **Empty content**: nothing is left after the BOMs and the bytes of step 2 of `Detect` go, at both ends. `Load` returns an empty subtitle in the format of the extension, under the fallback rule of step 8 |
| 6 | **Detect** the format of the decoded content |
| 7 | **Profile by extension**: when `Detect` gives TTML and the extension is `.itt`, `Load` reads the file as iTT |
| 8 | **Extension fallback**: without a detection result, `Load` uses `FormatFromPath`, when the library can read that format and the extension is not `json`, `txt` or `ffmeta`. CSV and TSV have no signature, so `.csv` and `.tsv` files load only by this step. A binary format from this step reads the raw bytes, not the decoded text |
| 9 | **No format**: an error that matches `ErrUnknownFormat`, with `Format detection found no subtitle format.` `Parse` has no path, so it skips steps 3, 5, 7 and 8 |

| Head | Bytes |
|:--- |:--- |
| Matroska and WebM | `1A 45 DF A3` at byte 0 |
| MP4 | `ftyp` at bytes 4 to 7 |
| MPEG-TS | `47` at bytes 0, 188 and 376 |
| M2TS, MPEG-TS with 192-byte packets | `47` at bytes 4, 196 and 388 |

- **Fixed fallback list**: the extensions `json`, `txt` and `ffmeta` also belong to formats of later releases that detection never finds, such as cloud speech-to-text JSON and chapters. So these extensions say nothing about the content. The list stays fixed for the whole 1.x line, so a `.json` file gives the same result in every release.
