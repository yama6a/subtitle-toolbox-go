# Command line tool

`stbox` reads, converts and checks subtitle files from the shell. Example: `stbox convert season1 --to vtt --output-dir out` reads every subtitle file of `season1/` and writes `out/<name>.vtt` for each. It never overwrites a file, prints one report line per file and a summary, and exits 3 when one file failed.

## Calls

| Call | Result |
|:--- |:--- |
| `stbox`, `stbox --help`, `stbox -h`, `stbox help` | the global help on stdout, exit 0 |
| `stbox --version`, `stbox -V` | the version, for example `1.0.0`, exit 0 |
| `stbox help CMD`, `stbox CMD --help`, `stbox CMD -h` | the help of CMD, exit 0. `--help` wins over every other word before `--` |
| `stbox CMD ...` | runs CMD |
| an unknown command | `Error: Unknown command "NAME".` and `Run "stbox help" for the usage.`, exit 2 |

## Argument syntax

The words after the command name follow the PHP 5.x CLI. The standard `flag` package stores the values, and a pre-pass applies these rules first.

| Word | Meaning |
|:--- |:--- |
| `--` | every later word is a file argument |
| `-` | standard input, or standard output for `-o` |
| a word without a leading `-` | a file argument |
| `--name` | a flag, or a value option that takes the next word as its value, also when that word starts with `-`: `--shift -2.5` |
| `--name=value` | a value option, split at the first `=` |
| `-o`, `-h` | the short names of `--output` and `--help`. Only the exact word counts: `-ofile` and `-o=file` are unknown options |

- **Order**: options and file arguments mix in any order. `stbox convert in.srt --to vtt` works.
- **Repeats**: a flag given twice is fine. A value option given twice fails with `The option --NAME is given twice.`, unless its help says "Repeatable".
- **Negative numbers**: `-2.5` as a file argument fails with `Unknown option -2.5.`. Pass `-- -2.5` for a file of that name.
- **Choices**: values such as `--line-ending CRLF` ignore ASCII case.
- **Numbers**: a number option takes the PHP numeric forms `2.5`, `-2.5`, `+1`, `.5`, `5.` and `1e3`. Hexadecimal, `inf` and `NaN` fail.
- **Times**: a time option takes seconds, such as `2.5`, or a time code `h:mm:ss.mmm` or `m:ss.mmm` with `.` or `,`, such as `00:00:02,500`. A leading `-` makes it negative. Times round to whole milliseconds and stay below 100000 hours.

## Input files

| File argument | Inputs |
|:--- |:--- |
| `-` | standard input. Messages call it `stdin` |
| a directory | its files, not recursive, in byte order of the names: each file whose extension names a readable format, and each `.txt` file in which content detection finds a format |
| an existing path | the path |
| a glob such as `"season1/*.srt"` | the matching files, for shells that do not expand globs. No match keeps the argument, which then fails as a missing file |

- **Duplicates**: an input whose real path came before is left out. `"dir/*.srt" dir/b.srt` reads `dir/b.srt` once.
- **Format**: without `--from`, the content decides, then the extension, as [detection.md](detection.md) says. `--from` and `--to` take a format name or an extension: `srt`, `.SRT` and `vtt` work.
- **Encoding**: `--encoding NAME` sets the source encoding of input that is not UTF-8. An unknown name fails before the first read, and the message lists every name. [encodings.md](encodings.md#supported-names) holds the list.
- **Lenient mode**: `--lenient` skips or repairs broken cues and prints each warning on stderr as `LABEL: line N: MESSAGE (skipped)`. A strict read that fails on a broken cue prints `Pass --lenient to skip or repair broken cues.`.
- **Batches**: the run stops at the first input that fails. `--keep-going` goes on with the next one.

### Text files in a directory

A `.txt` file in a directory is an input only when content detection finds a format in it. A WebVTT file named `film.txt` converts, and a note such as `notes.txt` stays out without a message.

- **Why**: MPL2, TMPlayer and QuickTime text use `.txt` too, and so do notes and logs. The extension alone cannot tell them apart.
- **Not chosen**: keep every `.txt` file, which fails on each note. Skip every `.txt` file as PHP does, which leaves the subtitle files of these formats unread.
- **Cost**: the expansion reads each `.txt` file of a directory once more.

## Output files

| Call | Subtitle data | Report lines such as `in.srt -> out/in.vtt` |
|:--- |:--- |:--- |
| one input, no `-o`, no `--output-dir` | stdout | stderr |
| `-o -` | stdout | stderr |
| `-o FILE` | FILE | stdout, or stderr for standard input |
| `--output-dir DIR` | `DIR/NAME` | stdout |

- **Several inputs** need `--output-dir DIR`. Each output keeps the base name of its input. An extension that the output format does not own becomes the first extension of that format: `movie.en.srt` with `--to vtt` gives `out/movie.en.vtt`.
- **Never overwrite**: every output is checked before the first read. An output that exists, also as a directory or a symbolic link, fails with exit 2, and so does an output that is a file the command reads, or the output of 2 inputs. The create is exclusive, so a file that appears during the run fails too.
- **Rollback**: when an output cannot be created or written, the run removes every file and directory that it created, also the outputs of earlier inputs, and exits 3.
- **Extension rule**: the extension of `-o` never picks the format. `--to srt -o film.vtt` fails. Without `--to`, an `-o` extension of another writable format fails too, so SubRip never lands in a `.vtt` file by mistake. An extension of no format, such as `.bak`, always works.
- **Interrupt**: Ctrl-C or SIGTERM stops the run before the next input. The finished outputs stay, because each is a complete file. A second Ctrl-C ends the process at once. An output that is being written then can stay incomplete, and the run removes nothing.

## info

`stbox info <input>... [--json] [options]` prints what the reader found in each subtitle file. It takes `--json` and the input options. Example: `stbox info latin1.srt` prints `Format:   srt`, `Encoding: Windows-1252` and `Cues:     1`.

| Row | Shown when | Value |
|:--- |:--- |:--- |
| `Format:` | always | the format name, for example `srt` |
| `Encoding:` | the read has a source encoding | for example `UTF-8` or `Windows-1252` |
| `Cues:` | always | the cue count |
| `Warnings:` | the read returned warnings | their count |
| `Metadata KEY:` | one row per metadata entry | the value, rows in byte order of the keys |

- **Text**: each input prints a block that starts with the input as given, or `stdin`. A block after the first successful one starts with an empty line. A failed file does not count.
- **Exit code**: 3 when a file failed, else 0.

`--json` prints a list with one object for each input that did not fail. The keys come in this order:

| Key | Value |
|:--- |:--- |
| `file` | the input as given, `-` for standard input |
| `format` | the format name |
| `encoding` | the source encoding, or `null` when the read has none |
| `metadata` | an object with the metadata keys in byte order, `{}` when empty |
| `warnings` | the warnings of the read as in [JSON output](#json-output), `[]` when none |

```json
[
    {
        "file": "trip.srt",
        "format": "srt",
        "encoding": "UTF-8",
        "metadata": {},
        "warnings": []
    }
]
```

A minor release may add keys to these objects. [compatibility.md](compatibility.md) states the rule.

## formats

`stbox formats` lists the formats that can be read or written, in registry order. The names are the values of `--from` and `--to`. It takes no argument and no option except `-h`.

```
Name  Extensions  Read  Write
json  .json       yes   yes
srt   .srt        yes   yes
vtt   .vtt        yes   yes
```

- **Shared extensions**: the first format in the list owns an extension. `--from` reads a file as another format.
- **Hidden formats**: a format that can be neither read nor written has no row.

## diff

`stbox diff OLD NEW` compares two files and prints the differences of [compare.md](compare.md). It exits 1 when the files differ.

- **Expansion**: only OLD goes through the input expansion. A directory or glob that gives 2 or more files fails with exit 2. NEW is used as given, and it cannot be `-`.
- **Input options**: `--from`, `--encoding` and `--lenient` apply to both files. `--new-from` replaces `--from` for NEW.
- **Failure**: a failed file prints `PATH: MESSAGE` with its own path and exits 3. A message about the format of NEW names `--new-from`.
- **No `--keep-going`**: the command has one pair of files.
- **JSON**: one object with the keys `oldFile`, `newFile`, `equal`, `differences`, `oldWarnings` and `newWarnings`. Cue times are seconds, such as `6.2`.

## JSON output

`--json` prints one JSON array on stdout at the end of the run, with one object for each input that did not fail. When every input failed, it prints `[]`.

- **Warnings**: each object lists the warnings of its read as `{"lineNumber": 6, "blockIndex": 1, "message": "...", "action": "skipped"}`. `lineNumber` and `blockIndex` are `null` when the warning has no line or no block. The warnings also print on stderr as text.
- **Summary**: with `--json`, the batch summary goes to stderr, so stdout holds JSON only.
- **Text**: invalid UTF-8 prints as U+FFFD. `<`, `>` and `&` stay as they are.

## Exit codes

| Code | Meaning | Example |
|---:|:--- |:--- |
| 0 | success | every input was written |
| 1 | result | `diff` found a difference. Not an error |
| 2 | invalid arguments | an unknown option, no input, an output that exists, `--encoding CP1125` |
| 3 | a file could not be read or written | a missing input, a broken cue in strict mode, an output directory that cannot be created |
| 130 | stopped by Ctrl-C or SIGTERM | the finished outputs stay |

The phase decides the code, not the error. [errors.md](errors.md#cli-exit-codes) holds the rules.
