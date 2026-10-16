# Documentation

Start with the [README](../README.md) for the install steps and a first call. Each page below holds usage and decisions. The godoc of each package holds the reference of every type and field.

| Page | Covers |
|:--- |:--- |
| [design.md](design.md) | the decisions behind the package layout, the options, the errors and the dependencies |
| [formats.md](formats.md) | the `Format` type, shared extensions, the time limit, and one section per format |
| [json.md](json.md) | library JSON version 1, which keeps every field of the model |
| [subtitle.md](subtitle.md) | `Subtitle` and `Cue`, times, comments, metadata, alignment and lookups |
| [format-data.md](format-data.md) | the data that a format keeps for its own writer, and its type checks |
| [markup.md](markup.md) | the inline tags, escaping, colors and the markup helpers |
| [reading-writing.md](reading-writing.md) | `Load`, `Parse` and `Write`, the options, line endings and the BOM |
| [detection.md](detection.md) | how `Detect` picks a format, and the formats it never detects |
| [encodings.md](encodings.md) | BOMs, UTF-16, code page detection and invalid UTF-8 |
| [lenient-parsing.md](lenient-parsing.md) | strict and lenient mode, and the warnings of a read |
| [errors.md](errors.md) | the sentinel errors, the error types, line numbers and the CLI exit codes |
| [editing.md](editing.md) | retiming, timing fixes, cue structure and line wrapping |
| [text.md](text.md) | case, replace, strip tags, right-to-left fixes and dialogue dashes |
| [cli.md](cli.md) | the `stbox` command: install, arguments, input and output files, `info`, `formats` and exit codes |
| [compatibility.md](compatibility.md) | what a release may change, and what stays the same for the whole 1.x line |
| [php-differences.md](php-differences.md) | each behaviour that differs from PHP 5.x on purpose |
