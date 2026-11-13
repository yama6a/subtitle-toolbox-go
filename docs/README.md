# Documentation

Start with the [README](../README.md) for the install steps and a first call. Each page below holds usage and decisions. The godoc of each package holds the reference of every type and field.

| Page | Covers |
|:--- |:--- |
| [design.md](design.md) | the decisions behind the package layout, the options, the errors and the dependencies |
| [chapters.md](chapters.md) | chapter lists as subtitles, the end rule, `ReadOptions.Chapters`, the YouTube chapter format and the YouTube chapter check |
| [formats.md](formats.md) | the `Format` type, shared extensions, the time limit, and one section per format |
| [json.md](json.md) | library JSON versions 1 and 2, which keep every field of the model |
| [subtitle.md](subtitle.md) | `Subtitle` and `Cue`, times, comments, metadata, alignment and lookups |
| [format-data.md](format-data.md) | the data that a format keeps for its own writer, and its type checks |
| [markup.md](markup.md) | the inline tags, escaping, colors and the markup helpers |
| [reading-writing.md](reading-writing.md) | `Load`, `Parse` and `Write`, the options, line endings and the BOM |
| [streaming.md](streaming.md) | package `stream`: SubRip and WebVTT read and written one cue at a time, and how it differs from the full reader |
| [containers.md](containers.md) | the subtitle tracks of video files: `Tracks`, `ReadOptions.Track`, and the codecs and rules of each container |
| [detection.md](detection.md) | how `Detect` picks a format, and the formats it never detects |
| [encodings.md](encodings.md) | BOMs, UTF-16, code page detection and invalid UTF-8 |
| [lenient-parsing.md](lenient-parsing.md) | strict and lenient mode, and the warnings of a read |
| [frame-rates.md](frame-rates.md) | the named frame rates, frames and durations, the frame-count and clock time models, drop-frame labels and `ParseTime` |
| [errors.md](errors.md) | the sentinel errors, the error types, line numbers and the CLI exit codes |
| [editing.md](editing.md) | retiming, timing fixes, shot changes and gaps, cue structure, line wrapping and dual subtitles |
| [compare.md](compare.md) | `diff` and `stbox diff`: the added, removed and changed cues between two subtitles, and how cues pair |
| [sync.md](sync.md) | `refsync` and `stbox sync`: the offset and frame-rate scale against a reference subtitle, and splits at ad breaks |
| [statistics.md](statistics.md) | `stats` and the statistics of `stbox info`: counts, display time, reading speed, gaps and the most used words |
| [validation.md](validation.md) | `validation` and `stbox validate`: the 19 rules, their units, the order of the violations and the command options |
| [text.md](text.md) | case, replace, strip tags, right-to-left fixes, dialogue dashes, `hearingimpaired`: the removal of hearing-impaired annotations, `speakers`: labels, renames and prefix, dash or color styles, `fixing`: the fixes of common text and OCR errors, and `profanity`: masked words and mute ranges |
| [ocr.md](ocr.md) | package `ocr`: the engine interface, `Run`, the report, the language constants and an engine for an OCR web service |
| [hls.md](hls.md) | package `hls` and `stbox hls`: the WebVTT segments, the playlist, the timestamp map and the joiner |
| [cli.md](cli.md) | the `stbox` command: install, arguments, input and output files, `info`, `validate`, `sync`, `diff`, `dual`, `hls`, `formats` and exit codes |
| [compatibility.md](compatibility.md) | what a release may change, and what stays the same for the whole 3.x line |
| [upgrade-2.0.md](upgrade-2.0.md) | the breaks of 2.0 with the old call and the new call, the new types, and the conversions that change output |
| [upgrade-3.0.md](upgrade-3.0.md) | the breaks of 3.0 with the old call and the new call |
| [upgrade-4.0.md](upgrade-4.0.md) | the breaks of 4.0 with the old call and the new call |
| [runbooks/release.md](runbooks/release.md) | the release steps, the checks before a major release, and the checks after each release |
| [php-differences.md](php-differences.md) | each behaviour that differs from PHP 5.x on purpose |
