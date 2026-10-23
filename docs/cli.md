# Command line tool

`stbox` reads, converts and checks subtitle files from the shell. Example: `stbox convert season1 --to vtt --output-dir out` reads every subtitle file of `season1/` and writes `out/<name>.vtt` for each. It never overwrites a file, prints one report line per file and a summary, and exits 3 when one file failed.

## Install

| Form | Needs | Example |
|:--- |:--- |:--- |
| archive on the GitHub release | nothing | `curl -fsSLO https://github.com/yama6a/subtitle-toolbox-go/releases/download/v1.0.0/stbox_1.0.0_linux_amd64.tar.gz` |
| `go install` | Go of the `go` line in `go.mod` | `go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@latest` |
| image `ghcr.io/yama6a/subtitle-toolbox-go` | Docker or another container runtime | `docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" ghcr.io/yama6a/subtitle-toolbox-go:1.0.0 convert in.srt --to vtt -o out.vtt` |

- **Version**: the archive names, the image tag and `stbox --version` show `1.0.0`. The git tag is `v1.0.0`. The image also has the tags `1.0`, `1` and `latest`.
- **Archives**: `stbox_1.0.0_OS_ARCH.tar.gz` for linux and darwin, `stbox_1.0.0_windows_ARCH.zip` for windows. Each holds `stbox`, `LICENSE`, `README.md` and `THIRD_PARTY_NOTICES.md`.
- **Checksums**: `sha256sum --ignore-missing -c stbox_1.0.0_checksums.txt`.
- **macOS**: the binaries are not notarized. macOS blocks a downloaded binary until the user runs `xattr -d com.apple.quarantine stbox`.
- **Image**: the tool runs in `/work`, so mount the files there. `--user` makes the tool write files that the user owns. Without it, the tool runs as user 65532 and cannot write to most mounted folders.
- **Platforms**: archives for linux, darwin and windows on amd64 and arm64. The image is for linux/amd64 and linux/arm64.

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
- **Format**: without `--from`, the content decides, then the extension, as [detection.md](detection.md) says. `--from` and `--to` take a format name or an extension: `srt`, `.SRT`, `vtt`, `ssa`, the extension of `ass`, and `sub`, the extension of `microdvd`, work.
- **Encoding**: `--encoding NAME` sets the source encoding of input that is not UTF-8. An unknown name fails before the first read, and the message lists every name. [encodings.md](encodings.md#supported-names) holds the list.
- **Lenient mode**: `--lenient` skips or repairs broken cues and prints each warning on stderr as `LABEL: line N: MESSAGE (skipped)`. A strict read that fails on a broken cue prints `Pass --lenient to skip or repair broken cues.`.
- **Batches**: the run stops at the first input that fails. `--keep-going` goes on with the next one.

### Frame rates

MicroDVD counts time in video frames, so a MicroDVD read and a MicroDVD write need the frame rate of the video. So do CSV and TSV times in `hh:mm:ss:ff`. Example: `stbox convert film.sub --fps 25 --to srt -o film.srt` reads a MicroDVD file without a `{1}{1}<fps>` first line at 25 fps.

| Option | Commands | Sets | Without it |
|:--- |:--- |:--- |:--- |
| `--input-fps RATE` | every command that reads subtitle files | `ReadOptions.MicroDVD.FrameRate` and `ReadOptions.CSV.FrameRate` | `--fps`, else the `{1}{1}<fps>` line of a MicroDVD file. A CSV or TSV time in `hh:mm:ss:ff` fails |
| `--output-fps RATE` | `convert`, `sync` | `WriteOptions.MicroDVD.FrameRate`, for MicroDVD output only | `--fps`, else the frame rate of a MicroDVD input |
| `--fps RATE` | every command that reads subtitle files | both rates | |

- **Check**: each rate is a number above 0. `--fps` is checked first, also when the specific option is given, so a bad `--fps` always fails. `--input-fps` comes before `--from` in the check order, and `--output-fps` right after `--to`.
- **Advice**: `frames.sub: The frame rate is unknown. Pass --fps or --input-fps, start the file with {1}{1}<fps>, or pass --lenient for 23.976 fps.`, `frames.csv: line 2: The time "00:00:01:12" counts frames and needs a frame rate. Pass --fps or --input-fps.` and `trip.srt: MicroDVD output needs the frame rate of the video. Pass --fps or --output-fps.`. Each exits 3.
- **TSV to CSV**: `convert trip.tsv --to csv` writes commas, and `--to tsv` writes tabs, as [formats.md](formats.md#csv-and-tsv) says.
- **Frame rate change**: `--from-fps` and `--to-fps` of the `retime` group retime the cues. They do not take `--fps`.

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

## convert

`stbox convert <input>... [--to FORMAT] [-o FILE | --output-dir DIR] [options]` reads subtitle files, runs edits on them, and writes them in the format of `--to`, else in the input format. Example: `stbox convert film.srt --to vtt --shift -0.5 --timing-fix-overlaps -o film.vtt` shifts every cue 0.5 s earlier, ends each cue before the next one starts, and writes WebVTT.

- **Help**: `stbox convert --help` lists the common options and the groups. `--help GROUP` prints the options of one group, `--help all` prints all of them. A topic with `.`, `/` or `\`, or the name of an existing file, is no topic: `convert in.srt -h out.srt` prints the default help.
- **Checks**: every option check runs before the first read, so a bad value exits 2 and reads nothing. The input options come first, then the output options, `--language`, the groups in pipeline order, the `ass` group and the `--to ass` rule. The first failing check decides the message.
- **`--language`** sets the language of `--case` and `--errors-fix`, for example `tr` for the Turkish dotted i. Without `--case` or `--errors-fix` it fails. Without it, `--errors-fix` takes the language metadata of the input.
- **WebVTT colors**: WebVTT has classes for 8 colors. For each other color, convert prints `LABEL: Cue #3 at 4 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.` on stderr and still exits 0.

### Pipeline

The edits run in this order, whatever the order of the flags. A group runs only when one of its options is set.

| # | Group | Options | Why at this place |
|---:|:--- |:--- |:--- |
| 1 | `forced` | `--forced-only` | later steps see only the cues that stay |
| 2 | `errors` | `--errors-fix`, `--errors-*` | the annotation removal and the replacements see fixed words such as `It's` for `lt's` |
| 3 | `sdh` | `--sdh`, `--sdh-*` | the replacements and the text edits see the text without annotations |
| 4 | `replace` | `--replace FROM=TO`, `--replace-regex`, `--replace-ignore-case` | |
| 5 | `text` | `--case`, `--strip-tags`, `--rtl` | text edits change the line lengths, so they run before `structure` |
| 6 | `structure` | `--structure-*` | joins and splits change the cues, so the timing edits run after it |
| 7 | `retime` | `--sync`, `--shift`, `--scale`, `--from-fps` with `--to-fps`, in this order | |
| 8 | `timing` | `--timing-*` | |

The `ass` group comes last in the help. It runs no edit: it sets how the ASS writer writes the output.

| Option | Value | Default | Effect |
|:--- |:--- |:--- |:--- |
| `--ass-karaoke-tag TAG` | `k`, `kf` or `ko` | `k` | the karaoke tag of the word timestamps of a converted cue, `WriteOptions.ASS.KaraokeTag` |
| `--ass-style STYLE` | `Field=Value,...` | none | changes the `Default` style, as FFmpeg `force_style`, `WriteOptions.ASS.Style` |

- **Check order**: the `--ass-karaoke-tag` choice, then the style check, whose error reads `The option --ass-style is not valid. MESSAGE`, then the `--to ass` rule.
- **`--to ass` rule**: both options need `--to ass` or `--to ssa`. The rule reads `--to` only, so an ASS input without `--to` fails too: `Pass --to ass with --ass-style.`

Each group calls the library methods of [editing.md](editing.md) and [text.md](text.md). Inside `structure` the run order is not the help order:

| Step | Option | Library call |
|---:|:--- |:--- |
| 1 | `--structure-unwrap` | `UnwrapLines()` |
| 2 | `--structure-merge-same-time` | `MergeSameTimeCues(0)` |
| 3 | `--structure-merge-short` | `MergeShortCues` with `--structure-max-cpl` and `--structure-max-lines` |
| 4 | `--structure-wrap` | `WrapLines(cpl, lines)`, 42 and 2 by default |
| 5 | `--structure-merge-duplicates` | `RemoveDuplicateCues(0)` |

`--structure-merge-same-time` comes last in the help but runs second. So its joined cues take part in the merge of short cues and in the wrap.

### Common errors

`--errors-fix` runs `fixing.Apply` with the rules of [text.md](text.md#common-errors). `stbox convert film.srt --errors-fix --language en -o film.fixed.srt` turns `lt's late.` into `It's late.` and `Really ?` into `Really?`.

| Option | Value | Library field |
|:--- |:--- |:--- |
| `--errors-fix` | flag, turns the group on and runs the rules that are on by default | |
| `--errors-enable NAME[,NAME]` | a comma list of the rules that are off by default: `adCues`, `doubleApostrophes`, `loneLowercaseI`, `unneededPeriods`, `dialogueOnOneLine`, `musicNotes`, `sentenceStartCase` | `Enable` |
| `--errors-list-fixes` | flag, prints each fix to stderr | |
| `--language CODE` | a common option | `Language` |

- **Checks**: `--errors-enable` and `--errors-list-fixes` need `--errors-fix`: `Pass --errors-fix with --errors-list-fixes.` The names of `--errors-enable` are trimmed and match in any case. An empty name is skipped. Another name fails: `Unknown rule "X" in --errors-enable. The valid names are adCues, doubleApostrophes, loneLowercaseI, unneededPeriods, dialogueOnOneLine, musicNotes, sentenceStartCase.`
- **Fix line**: `LABEL: cue N: RULE: "BEFORE" -> "AFTER"`, with N from 1. BEFORE and AFTER are JSON strings, so a line feed reads `\n` and invalid UTF-8 reads U+FFFD: `film.srt: cue 3: ocrLowercaseL: "lt's late." -> "It's late."`.

### Hearing-impaired annotations

`--sdh` runs `hearingimpaired.Apply` with the rules of [text.md](text.md#hearing-impaired-annotations). `stbox convert film.srt --sdh -o film.clean.srt` removes `[DOOR SLAMS]`, `(laughs)`, `JOHN:` and lines of music symbols, and each cue with no text left.

| Option | Value | Library field |
|:--- |:--- |:--- |
| `--sdh` | flag, turns the group on | |
| `--sdh-keep-square-brackets` | flag | `KeepSquareBrackets` |
| `--sdh-keep-parentheses` | flag | `KeepParentheses` |
| `--sdh-keep-speaker-labels` | flag | `KeepSpeakerLabels` |
| `--sdh-keep-music-lines` | flag | `KeepMusicLines` |
| `--sdh-any-case-labels` | flag | `AnyCaseLabels` |
| `--sdh-lyrics` | flag | `Lyrics` |
| `--sdh-brackets PAIR` | repeatable, exactly 2 characters such as `{}` or `**` | `CustomBrackets`, in flag order |

- **Checks**: every option except `--sdh` needs `--sdh`: `Pass --sdh with --sdh-lyrics.` The options are checked in the order of the table. Then each PAIR in flag order: `The option --sdh-brackets needs two characters, an opening and a closing one, got "{".`
- **Character**: one Unicode code point. A PAIR of invalid UTF-8 fails.

### Sync points

`--sync OLD=NEW` moves the time OLD to NEW. OLD is a time, `first`, `last` or `#N`, the start of cue N in start order. `--sync first=00:00:12.5 --sync last=01:41:05` stretches the whole file. One point shifts every cue.

- The points with a time as OLD must increase in both times. The check runs before the first read.
- `first`, `last` and `#N` resolve per file. A file without cues, or with fewer cues than N, fails, exit 3.

### Regular expressions

`--replace-regex` reads each FROM as a PHP PCRE pattern with delimiters, such as `/\.{4,}/` or `#colou?r#i`. Convert turns it into a pattern of [regexp2](https://github.com/dlclark/regexp2), the engine of [text.md](text.md#replace).

| Part | Rule | Example |
|:--- |:--- |:--- |
| leading white space | skipped | ` /a/` reads as `/a/` |
| delimiter | the first character, not a letter, a digit, `\` or white space. `(`, `[`, `{` and `<` close with their partner and nest. Other delimiters close with the same character | `/a/`, `#a/b#`, `{a{2}}` |
| escapes | a `\` before the delimiter or other punctuation stays, because PHP passes it to PCRE and regexp2 reads it the same way. `\_` becomes `_`, because regexp2 rejects `\_` | `/a\/b/` gives `a\/b`, `/a\_b/` gives `a_b` |
| flags | `i` matches in any case. `m`, `s` and `x` become `(?msx)` before the pattern. `u` changes nothing, because Go strings are UTF-8. A space, LF and CR are skipped, as in PHP. Other flags fail | `/col(ou)r/i` |
| TO | `\N`, `$N` and `${N}`, each with 1 or 2 digits, insert group N, or nothing when the pattern has no group N. `\\` and `\$` write `\` and `$`. Every other `$` is text, as in PHP | `\1` gives group 1, `$_` stays `$_` |

- **Not supported**: `\K`, `\h`, `\o{...}`, `(?|...)`, `(*VERB)` controls and the Python-style named groups `(?P<name>...)`, `(?P=name)` and `(?P>name)`. Write `(?<name>...)` and `\k<name>`. A pattern with one of them fails as an invalid regular expression.
- **POSIX classes**: `[[:alpha:]]` and `[[:digit:]]` give no error, but they match the characters of the brackets, not a letter or a digit. Write `\p{L}` and `\d`.
- **Without `--replace-regex`**, FROM and TO are plain text.

## info

`stbox info <input>... [--json] [options]` prints what the reader found in each subtitle file and the [statistics](statistics.md) of its cues. It takes `--json` and the input options. Example: `stbox info morning.srt` for a one-cue file prints the block below.

```
morning.srt
  Format:                srt
  Encoding:              UTF-8
  Cues:                  1
  Words:                 2
  Characters:            13
  Display time:          2 s
  Span:                  2 s
  Characters per second: min 6.5, average 6.5, max 6.5
  Words per minute:      min 60, average 60, max 60
  Characters per line:   min 13, average 13, max 13
  Gaps:                  -
  Most used words:       good (1), morning (1)
```

| Row | Shown when | Value |
|:--- |:--- |:--- |
| `Format:` | always | the format name, for example `srt` |
| `Encoding:` | the read has a source encoding | for example `UTF-8` or `Windows-1252` |
| `Cues:` | always | the cue count |
| `Warnings:` | the read returned warnings | their count |
| `Words:`, `Characters:` | always | the counts |
| `Display time:` | always | the total display time, `X s` |
| `Span:` | always | `X s`, or `-` without cues |
| `Characters per second:`, `Words per minute:`, `Characters per line:` | always | `min A, average B, max C`, or `-` |
| `Gaps:` | always | `min A, average B, max C s`, or `-` |
| `Most used words:` | always | `word (n), word (n), ...`, at most 10, wrapped at 80 columns |
| `Metadata KEY:` | one row per metadata entry | the value, rows in byte order of the keys |

- **Numbers**: an integer prints as it is. Any other number prints with at most 3 decimals, without trailing zeros, so `27.53333` prints `27.533` and `4.90` prints `4.9`. Times are in seconds.
- **Text**: each input prints a block that starts with the input as given, or `stdin`. A block after the first successful one starts with an empty line. A failed file does not count.
- **Exit code**: 3 when a file failed, else 0.

`--json` prints a list with one object for each input that did not fail. The keys come in this order:

| Key | Value |
|:--- |:--- |
| `file` | the input as given, `-` for standard input |
| `format` | the format name |
| `encoding` | the source encoding, or `null` when the read has none |
| `metadata` | an object with the metadata keys in byte order, `{}` when empty |
| `statistics` | the numbers of [statistics.md](statistics.md#json) |
| `warnings` | the warnings of the read as in [JSON output](#json-output), `[]` when none |

```json
[
    {
        "file": "morning.srt",
        "format": "srt",
        "encoding": "UTF-8",
        "metadata": {},
        "statistics": {
            "cueCount": 1,
            "wordCount": 2,
            "characterCount": 13,
            "totalDisplayTime": 2,
            "span": 2,
            "charactersPerSecond": {
                "min": 6.5,
                "average": 6.5,
                "max": 6.5
            },
            "wordsPerMinute": {
                "min": 60,
                "average": 60,
                "max": 60
            },
            "charactersPerLine": {
                "min": 13,
                "average": 13,
                "max": 13
            },
            "gaps": null,
            "mostUsedWords": [
                {
                    "word": "good",
                    "count": 1
                },
                {
                    "word": "morning",
                    "count": 1
                }
            ]
        },
        "warnings": []
    }
]
```

A minor release may add keys to these objects. [compatibility.md](compatibility.md) states the rule.

## sync

`stbox sync <input>... --reference FILE [options]` retimes each input to the cue times of a reference subtitle. It finds the offset and the frame-rate scale. [sync.md](sync.md#command-line) lists its options and output.

## diff

`stbox diff OLD NEW` compares two files and prints the differences of [compare.md](compare.md). It exits 1 when the files differ.

- **Expansion**: only OLD goes through the input expansion. A directory or glob that gives 2 or more files fails with exit 2. NEW is used as given, and it cannot be `-`.
- **Input options**: `--from`, `--encoding`, `--lenient`, `--input-fps` and `--fps` apply to both files. `--new-from` replaces `--from` for NEW.
- **Failure**: a failed file prints `PATH: MESSAGE` with its own path and exits 3. A message about the format of NEW names `--new-from`.
- **No `--keep-going`**: the command has one pair of files.
- **JSON**: one object with the keys `oldFile`, `newFile`, `equal`, `differences`, `oldWarnings` and `newWarnings`. Cue times are seconds, such as `6.2`.

## formats

`stbox formats` lists the formats that can be read or written, in registry order. The names are the values of `--from` and `--to`. It takes no argument and no option except `-h`.

```
Name       Extensions        Read  Write
ass        .ass .ssa         yes   yes
csv        .csv              yes   yes
json       .json             yes   yes
lrc        .lrc              yes   yes
microdvd   .sub              yes   yes
mpsub      .mpsub            yes   yes
sami       .smi .sami        yes   yes
sbv        .sbv              yes   yes
srt        .srt              yes   yes
subviewer  .sub              yes   yes
ttml       .ttml .dfxp .xml  yes   yes
tsv        .tsv              yes   yes
txt        .txt              no    yes
mpl2       .txt              yes   yes
tmplayer   .txt              yes   yes
qttext     .txt              yes   yes
vtt        .vtt              yes   yes
```

- **Shared extensions**: the first format in the list owns an extension, so `.sub` is MicroDVD and `.txt` is plain text. `--from` reads a file as another format. A `.sub` file with MPSub or SubViewer content is found by its content. Plain text has no reader, so `--from txt` fails with `The format txt can be written but not read.` Pass `--from mpl2`, `--from tmplayer` or `--from qttext`. `--to txt` writes plain text with the default options.
- **Hidden formats**: a format that can be neither read nor written has no row.

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
