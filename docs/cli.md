# Command line tool

`stbox` reads, converts and checks subtitle files from the shell. Example: `stbox convert season1 --to vtt --output-dir out` reads every subtitle file of `season1/` and writes `out/<name>.vtt` for each. It never overwrites a file, prints one report line per file and a summary, and exits 3 when one file failed.

## Install

| Form | Needs | Example |
|:--- |:--- |:--- |
| archive on the GitHub release | nothing | `curl -fsSLO https://github.com/yama6a/subtitle-toolbox-go/releases/download/v1.0.0/stbox_1.0.0_linux_amd64.tar.gz` |
| `go install` | Go of the `go` line in `go.mod` | `go install github.com/yama6a/subtitle-toolbox-go/v4/cmd/stbox@latest` |
| image `ghcr.io/yama6a/subtitle-toolbox-go` | Docker or another container runtime | `docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" ghcr.io/yama6a/subtitle-toolbox-go:1.0.0 convert in.srt --to vtt -o out.vtt` |
| image `ghcr.io/yama6a/subtitle-toolbox-go` with Tesseract | Docker or another container runtime | `docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" ghcr.io/yama6a/subtitle-toolbox-go:1.0.0-tesseract convert film.sup --ocr --ocr-engine tesseract --ocr-language rus --to srt -o film.srt` |

- **Version**: the archive names, the image tag and `stbox --version` show `1.0.0`. The git tag is `v1.0.0`. The image also has the tags `1.0`, `1` and `latest`.
- **Tesseract image**: the tags are `1.0.0-tesseract`, `1.0-tesseract`, `1-tesseract` and `tesseract`. It runs the same `stbox` and adds the `tesseract` program with the fast models of every language. It is about 400 MB larger. See [ocr.md](ocr.md#tesseract).
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

MicroDVD counts time in video frames, so a MicroDVD read and a MicroDVD write need the frame rate of the video. So do CSV and TSV times in `hh:mm:ss:ff`, the time codes of Spruce STL, and the iTT and MCC writers. Example: `stbox convert film.sub --fps 25 --to srt -o film.srt` reads a MicroDVD file without a `{1}{1}<fps>` first line at 25 fps.

| Option | Commands | Sets | Without it |
|:--- |:--- |:--- |:--- |
| `--input-fps RATE` | every command that reads subtitle files | `ReadOptions.MicroDVD.FrameRate`, `ReadOptions.CSV.FrameRate` and `ReadOptions.SpruceSTL.FrameRate` | `--fps`, else the `{1}{1}<fps>` line of a MicroDVD file, and 25 or 30 fps guessed for Spruce STL. A CSV or TSV time in `hh:mm:ss:ff` fails |
| `--output-fps RATE` | `convert`, `sync`, `dual` | the frame rate field of the output format only: `WriteOptions.MicroDVD`, `WriteOptions.SpruceSTL`, `WriteOptions.ITT` or `WriteOptions.MCC` | `--fps`, else the frame rate that the reader stored |
| `--fps RATE` | every command that reads subtitle files | both rates, and the rate of the video for `--video-fps` of `convert` | |

- **Check**: each rate is a number above 0. `--fps` is checked first, also when the specific option is given, so a bad `--fps` always fails. `--input-fps` comes before `--from` in the check order, and `--output-fps` right after `--to`.
- **Advice**: `frames.sub: The frame rate is unknown. Pass --fps or --input-fps, start the file with {1}{1}<fps>, or pass --lenient for 23.976 fps.`, `frames.csv: line 2: The time "00:00:01:12" counts frames and needs a frame rate. Pass --fps or --input-fps.` and `trip.srt: MicroDVD output needs the frame rate of the video. Pass --fps or --output-fps.`. Each exits 3. iTT output without a rate gives the same advice: `trip.srt: iTT output needs the frame rate of the video. Pass --fps or --output-fps.`.
- **Rates that a format lacks**: MCC accepts 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60 fps, and iTT 23.976, 24, 25, 29.97 and 30 fps. With `--to`, `convert trip.srt --to mcc --output-fps 12` fails with exit 2 before the first read. Without `--to`, the check fails the input that writes MCC, exit 3. `--output-fps 12` with SubRip output changes nothing, because the CLI sets only the field of the output format.
- **Drop-frame**: the CLI never sets drop-frame. MCC output at 29.97 or 59.94 fps and SCC output write drop-frame time codes, unless the input stored non-drop.
- **TSV to CSV**: `convert trip.tsv --to csv` writes commas, and `--to tsv` writes tabs, as [formats.md](formats.md#csv-and-tsv) says.
- **Frame rate change**: `--from-fps` and `--to-fps` of the `retime` group retime the cues. They do not take `--fps`.

### Broadcast options

| Option | Commands | Sets | Without it |
|:--- |:--- |:--- |:--- |
| `--scc-roll-up MODE` | every command that reads subtitle files | `ReadOptions.SCC.RollUp` and `ReadOptions.MCC.RollUp`, and the same field for the `c608` captions of an MP4 track. `MODE` is `screen` or `lines`, in any case | `screen`: one cue per screen of a roll-up caption. `lines` gives one cue per row |
| `--scc-fit` | `convert`, `sync`, `dual` | `WriteOptions.SCC.Fit`, for SCC output only | an SCC write of a cue that SCC cannot hold fails |

- **Order**: `--scc-roll-up` comes after `--fps` and is checked after `--input-fps`, before `--track` and `--from`. `--scc-fit` comes after `--no-bom`.
- **Write reports**: after a write, the CLI prints each change on stderr. SCC and MCC changes read `ferry.srt: Cue #3 at 5 s is left out. Its caption data needs the frames up to 5.839 s, but the cue ends at 5.4 s. (dropped)`. The action is `transliterated`, `wrapped`, `delayed` or `dropped`. The SCC changes need `--scc-fit`, and the MCC writer always fits.
- **EBU STL report**: a character that the character code table lacks becomes `?` in the file. The CLI prints one line for each, where C stands for the character, such as an em dash in cue 1: `show.srt: Cue 1: the character table has no "C", so the file holds "?".`. A character of a GSI field gives `GSI field OPT: ...`, and a character of a comment gives `Comment: ...`. PHP prints nothing.
- **SCC advice**: a cue that fits 4 lines of 32 characters after a wrap fails with `Pass --structure-wrap --structure-max-cpl 32 --structure-max-lines 4.`. A cue that needs a split fails without advice.
- **Binary output**: EBU STL, PAC and Cavena 890 output goes to stdout or to a file as raw bytes. `info` shows no `Encoding:` row for them, and `--json` gives `"encoding": null`.

### Video files and tracks

An MKV, WebM, MP4, MOV, MPEG-TS or M2TS file holds subtitle tracks. A track number picks one. [containers.md](containers.md) lists the track numbers of each container.

```
stbox info movie.mkv
stbox convert movie.mkv --track 3 --to srt -o movie.srt
```

| Option | Commands | Sets | Without it |
|:--- |:--- |:--- |:--- |
| `--track NUMBER` | every command that reads subtitle files | `ReadOptions.Track`. A whole number of 1 or more | the only subtitle track of the file. A file with 0 or 2 or more tracks fails |
| `--reference-track NUMBER` | `sync` | `ReadOptions.Track` of the reference file | as for `--track` |
| `--new-track NUMBER` | `diff` | `ReadOptions.Track` of the new file | `--track`, but only when the new file is a video file. `diff a.mkv b.srt --track 3` reads `b.srt` as a subtitle |
| `--primary-track NUMBER` | `dual` | the renamed `--track` of the primary file | as for `--track` |
| `--secondary-track NUMBER` | `dual` | `ReadOptions.Track` of the secondary file. It does not inherit `--primary-track` | as for `--track` |

- **Order**: `--track` comes after `--scc-roll-up`, is checked before `--from`, and comes before `--keep-going`.
- **Detection**: the first 389 bytes decide whether an input is a video file. The extension does not.
- **Format**: a track read has no input format, because the track decides it. `--from` has no effect with `--track`. `--from` without `--track` fails with `movie.mp4: The input is an MP4 file. Pass --track N.` The text says `an MKV or WebM file` or `an MPEG-TS file` for the other containers.
- **Memory**: a file input goes to `stbox.Load`, which reads only the bytes that the track needs. Standard input is read into memory, because the library cannot seek in a pipe. PHP writes it to a temporary file first.
- **Track errors**: a file with 0 or 2 or more subtitle tracks and no `--track` fails with exit 3 and lists the tracks. The sentence names the option of the file, here `--track`. The lenient hint does not print.

```
movie.mkv: The MKV or WebM file has 2 subtitle tracks. Pass --track N with one of them:
  3: S_TEXT/UTF8, de, "Deutsch (Forced)", forced
  4: S_TEXT/ASS, eng, "English", default
```

- **Track text**: each track line joins the codec, the language, the name as a JSON string, `default` and `forced`. It leaves out the parts that are empty or false.
- **Wrong number**: a number that no track has fails with `The file has no subtitle track with the number 9.`. `--track` on a file that is no video file fails with `Only MKV, WebM, MP4 and MPEG-TS files have tracks.` Both exit 3.
- **Output names**: the output format is the track format when `--to` is absent. A SubRip track of `movie.mkv` with `--output-dir out` writes `out/movie.srt`, and an MP4 `c608` track writes SCC.

### VobSub input

VobSub has 2 files, such as `movie.idx` and `movie.sub`. See [VobSub in formats.md](formats.md#vobsub).

- **Input**: pass the `.idx` path. Pass the `.sub` path with `--from vobsub`.
- **Standard input**: `--from vobsub` with `-` fails with exit 2 before any read: `VobSub needs the path of the .idx file. Standard input does not work.`
- **Directory**: a `.sub` file that the read of an `.idx` file of the folder opens is not an input of its own. The read looks for the same stem, with `.sub` or `.SUB`. The `.idx` file is the input.
- **Never overwrite**: for a VobSub input, the `.idx` file and the `.sub` file both count as files that the command reads. No output may overwrite either.

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
- **Never overwrite**: every output is checked before the first read, also the files of `--mute-edl` and `--mute-filter`. An output that exists, also as a directory or a symbolic link, fails with exit 2, and so does an output that is a file the command reads, or the output of 2 inputs. The create is exclusive, so a file that appears during the run fails too.
- **Rollback**: when an output cannot be created or written, the run removes every file and directory that it created, also the outputs of earlier inputs, and exits 3.
- **Extension rule**: the extension of `-o` never picks the format. `--to srt -o film.vtt` fails. Without `--to`, an `-o` extension of another writable format fails too, so SubRip never lands in a `.vtt` file by mistake. An extension of no format, such as `.bak`, always works.
- **Interrupt**: Ctrl-C or SIGTERM stops the run before the next input. The finished outputs stay, because each is a complete file. With `--ocr`, the run stops within one image, and the input that OCR was reading writes no output. A second Ctrl-C ends the process at once. An output that is being written then can stay incomplete, and the run removes nothing.

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
| 2 | `ocr` | `--ocr`, `--ocr-*` | OCR reads only the forced cues that stay, and the `errors` group then fixes the OCR text |
| 3 | `errors` | `--errors-fix`, `--errors-*` | the annotation removal and the replacements see fixed words such as `It's` for `lt's` |
| 4 | `sdh` | `--sdh`, `--sdh-*` | the replacements and the text edits see the text without annotations |
| 5 | `replace` | `--replace FROM=TO`, `--replace-regex`, `--replace-ignore-case` | |
| 6 | `text` | `--case`, `--strip-tags`, `--rtl` | text edits change the line lengths, so they run before `structure` |
| 7 | `structure` | `--structure-*` | joins and splits change the cues, so the timing edits run after it |
| 8 | `retime` | `--sync`, `--shift`, `--scale`, `--from-fps` with `--to-fps`, in this order | |
| 9 | `snap` | `--snap-*`, `--video-fps`, `--no-snap-chain` | the shot changes need the final times of the retime group, and the timing fixes in seconds come after the frames |
| 10 | `timing` | `--timing-*` | |
| 11 | `masking` | `--mask-words FILE`, `--mask`, `--mute-*` | the mute ranges of `--mute-edl` and `--mute-filter` need the final times |

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

### Image cues

PGS, VobSub and DVB subtitles hold bitmaps, so their cues have no text. A text writer fails on such a cue. `stbox convert text_1080p.sup --to srt` fails with exit 3:

```
text_1080p.sup: Cue #0 holds an image but no text, and srt needs text. Pass --ocr to read the text, --to pgs to keep the images, or --skip-image-cues to leave them out.
```

- **`--skip-image-cues`**: leaves out the image cues without text and sets `WriteOptions.SkipImageCues`. It comes after `--no-bom`, on `convert`, `sync` and `dual`. `convert text_1080p.sup --to srt --skip-image-cues` writes only the cues with text, here none.
- **Order**: the writer checks the image cues before its other checks, so an SCC write of PGS input reports them first.
- **`hls`**: the command has no `--skip-image-cues`. An image cue without text fails the cut.

### OCR

`--ocr` reads the text of image cues, for example from PGS or VobSub, with `ocr.Run`. `stbox convert text_1080p.sup --ocr --to srt` on a machine without Tesseract picks the glyph engine. The first cue reads `The train to Bergen leaves at 7:45.`.

| Option | Value | Default | Effect |
|:--- |:--- |:--- |:--- |
| `--ocr` | flag, turns the group on | off | |
| `--ocr-engine ENGINE` | `tesseract` or `glyph` | Tesseract when it is installed, else glyph | `auto.Options.Engine` |
| `--ocr-language CODE` | a Tesseract model name, `deu` or `deu+eng` | `eng` | `tesseract.Options.Language`. The glyph engine ignores it |
| `--ocr-database FILE` | a glyph database in `.nocr` format | the subtitle fonts database | `glyph.Options.Database`. It selects the glyph engine |

- **Checks**: the options `--ocr-engine`, `--ocr-language` and `--ocr-database` need `--ocr`: `Pass --ocr with --ocr-engine.` An engine other than `tesseract` or `glyph` fails: `The option --ocr-engine must be tesseract or glyph, got "easyocr".` `--ocr-database` with `--ocr-engine tesseract` fails: `Pass --ocr-engine glyph with --ocr-database.` All three exit 2.
- **Before the first read**: the command chooses the engine. An explicit `--ocr-engine tesseract` without the program, or a language without its data, exits 2 with the install hint of [ocr.md](ocr.md#tesseract). No output file exists then.
- **Database file**: the command loads it once per run, as a side file. A file that go-glyph-ocr rejects, for example a text file, prints `Error: FILE: MESSAGE` and exits 3.
- **Per input**: an input without image cues without lines runs no engine. The glyph engine is new for each input, because it learns the glyph heights of one stream. All glyph engines share one database.
- **Language**: with the glyph engine, `--ocr-language` prints `Warning: the glyph engine ignores --ocr-language.` once per run.
- **Progress**: the command prints `LABEL: OCR DONE/TOTAL` to standard error after every 100 images and after the last one, for example `film.sup: OCR 200/250`.
- **Failure**: an engine error fails the input with exit 3. The message names the cue, counted from 1: `film.sup: cue 3: MESSAGE`.

### Common errors

`--errors-fix` runs `fixing.Apply` with the rules of [text.md](text.md#common-errors). `stbox convert film.srt --errors-fix --language en -o film.fixed.srt` turns `lt's late.` into `It's late.` and `Really ?` into `Really?`.

| Option | Value | Library field |
|:--- |:--- |:--- |
| `--errors-fix` | flag, turns the group on and runs the rules that are on by default | |
| `--errors-replace-list FILE` | a [Subtitle Edit OCR replace list](text.md#ocr-replace-lists), an XML file. It is a side file, read once per run | `ReplaceList` |
| `--errors-enable NAME[,NAME]` | a comma list of the rules that are off by default: `adCues`, `doubleApostrophes`, `loneLowercaseI`, `unneededPeriods`, `dialogueOnOneLine`, `musicNotes`, `sentenceStartCase` | `Enable` |
| `--errors-list-fixes` | flag, prints each fix to stderr | |
| `--language CODE` | a common option | `Language` |

- **Replace list**: a list that does not read prints `Error: FILE: MESSAGE` and exits 3, before any output file exists. The reader skips a pattern that does not compile, so it never fails here.
- **Checks**: `--errors-replace-list`, `--errors-enable` and `--errors-list-fixes` need `--errors-fix`: `Pass --errors-fix with --errors-list-fixes.` The names of `--errors-enable` are trimmed and match in any case. An empty name is skipped. Another name fails: `Unknown rule "X" in --errors-enable. The valid names are adCues, doubleApostrophes, loneLowercaseI, unneededPeriods, dialogueOnOneLine, musicNotes, sentenceStartCase.`
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

### Masking

`--mask-words FILE` runs `profanity.Apply` with the rules of [text.md](text.md#profanity-filter). `stbox convert movie.srt -o clean.srt --mask-words words.txt --mute-edl clean.edl` writes `What the ****?` and the time of each match for Kodi and MPlayer.

| Option | Value | Default | Library |
|:--- |:--- |:--- |:--- |
| `--mask-words FILE` | a word file, turns the group on | | `Options.Words` |
| `--mask STYLE` | `stars`, `first-letter`, `remove` or `none`, in any case | `stars` | `Options.Mask` |
| `--mute-edl FILE` | a new file | none | `profanity.EDL` |
| `--mute-filter FILE` | a new file | none | `profanity.FFmpegVolumeFilter` |
| `--mute-padding SECONDS` | a time of 0 or more | 0 | `Options.Padding` |

- **Checks**: every option except `--mask-words` needs it: `Pass --mask-words with --mute-edl.` Then `--mask`, then `-` as a mute file: `The option --mute-edl needs a file path.`, then `--mute-padding`.
- **Word file**: one word per line. A leading UTF-8 BOM goes. CR LF, LF and CR end a line. Each line is trimmed, and empty lines go. The tool reads the file once, after the checks of the arguments. A missing file stops the run with exit 3: `Error: words.txt: The file does not exist.` An empty list or a bad word fails the option checks, so the run exits 2 with the help hint: `A profanity word must be a non-empty UTF-8 string with a * only at the end, got 'a*b'.`
- **One input**: the mute files hold the ranges of one input. With 2 or more inputs, `--mute-edl` and `--mute-filter` fail: `--mute-edl and --mute-filter take one input file, got 2.`
- **Mute files**: they join the output checks. Each must not exist, must not be a file that the command reads, such as the word file, and must differ from the subtitle output and from each other: `The --mute-filter file radio.mute is also the output of --mute-edl.`
- **Write**: the mute files come after the subtitle output, each with its report line such as `radio.vtt -> radio.edl`. A file that is not empty ends with LF. Without a match both files are empty, 0 bytes.

### Snap

`--snap-shot-changes FILE` runs `timing.Apply` of [editing.md](editing.md#shot-changes-and-gaps). `stbox convert garden.srt -o timed.srt --video-fps 24 --snap-shot-changes scenes.txt` moves each cue time that lies within 12 frames of a shot change onto it, closes gaps shorter than 12 frames and puts every time on a frame.

| Option | Value | Default | `Options` field |
|:--- |:--- |:--- |:--- |
| `--snap-shot-changes FILE` | a side file | none | `ShotChanges` |
| `--video-fps RATE` | a number above 0 | `--fps` | `FrameRate` |
| `--snap-window-frames FRAMES` | a whole number of 0 or more | half the frame rate in frames | `SnapWindowFrames` |
| `--snap-min-gap-frames FRAMES` | a whole number of 0 or more | 2 | `MinGapFrames` |
| `--snap-min-duration-frames FRAMES` | a whole number of 0 or more | 20 | `MinDurationFrames` |
| `--no-snap-chain` | flag | the chain is on | `KeepGaps` |

- **Trigger**: the group runs when one of `--snap-shot-changes`, `--snap-window-frames`, `--snap-min-gap-frames`, `--snap-min-duration-frames` or `--no-snap-chain` is set. Without `--snap-shot-changes`, it only closes gaps and puts the times on frames.
- **Rates**: `--video-fps` sets only the snap rate, `--input-fps` sets only the read rate and `--fps` sets both. A MicroDVD file written at 25 fps, read with `--input-fps 25 --video-fps 24`, snaps at 24 fps on the times read at 25 fps.
- **Checks**, in this order, each exit 2:
  1. The rate: `--fps`, then `--video-fps`.
  2. `--video-fps` without a trigger: `Pass --snap-shot-changes FILE with --video-fps.` `--fps` alone does not fail, because other groups use it.
  3. A trigger without a rate: `Pass --video-fps RATE or --fps RATE with --snap-window-frames.` It names the first set trigger of the table.
  4. `--no-snap-chain` without shot changes: `Pass --snap-shot-changes FILE. With --no-snap-chain and no shot changes, snapping changes nothing.`
  5. The frame counts, in table order: `The option --snap-window-frames needs a whole number of 0 or more, got "-1".`
  6. `Options.Validate`.
- **Shot change file**: the tool reads it once, after the checks of the arguments. Content with `pts_time:` is an FFmpeg `showinfo` log, other content is one time per line in seconds or `hh:mm:ss.mmm`. A missing file stops the run with exit 3: `Error: missing.txt: The file does not exist.` A bad line does too: `Error: scenes.txt: line 3: The shot change time "00:01:2x" is not valid. Use seconds or hh:mm:ss.mmm.`
- **Output checks**: the shot change file is a file that the command reads, so no output may replace it.

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
| `Image cues:` | the subtitle has 1 or more image cues | `N, M with text`. N counts the cues with an image, M those of them with lines |
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
| `imageCues` | `{"count": N, "withText": M}` as for the `Image cues:` row, also with 0 image cues |
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

### Video files

Without `--track`, `info` lists the tracks of a video file when the extension of the input names no subtitle format. `movie.mkv`, `movie.webm`, `movie.mp4`, `rec.ts` and `-` qualify. A video file named `x.srt` reads as a subtitle. With `--track`, `info` reads the track and prints the normal block.

```
movie.mp4
  Container: mp4
  Track 2: tx3g, eng, "English", default
  Track 3: tx3g, fr-CA, forced
  Track 4: c608, eng
  Track 5: enct, deu
```

`Container:` is `matroska`, `mp4` or `mpegts`. The first line is the input as given, or `stdin`. A damaged container makes `info` read the file as usual, which then reports the damage. The JSON object of a video file has the keys `file`, `container` and `tracks`. A track has the keys `number`, `codecId`, `language`, `name`, `default` and `forced`. `language` and `name` are `null` when empty.

```json
{
    "file": "pgs.mkv",
    "container": "matroska",
    "tracks": [
        {"number": 3, "codecId": "S_HDMV/PGS", "language": "ger", "name": null, "default": true, "forced": false}
    ]
}
```

The real output puts each key of a track on its own line, with 4 spaces of indent per level.

A minor release may add keys to these objects. [compatibility.md](compatibility.md) states the rule.

## validate

`stbox validate <input>... [--preset NAME] [rule options] [--json] [options]` checks each input with `validation.Check` of [validation.md](validation.md). It takes `--json` and the input options.

| Option | Effect |
|:--- |:--- |
| `--preset NAME` | `netflix-en` or `bbc` in any ASCII case. The preset is the base rules: `validation.NetflixEnglish(rate)` or `validation.BBC()` |
| `--video-fps RATE` | The frame rate of the `netflix-en` gap. Default: 23.976. Without it, `--fps` sets the rate. The option needs `--preset netflix-en` |

A rule option sets its field on the preset result. A flag can only turn a check on. Example: `--preset netflix-en --max-cps 30` keeps the other Netflix rules and allows 30 characters per second. `--fps` sets `--input-fps` and `--video-fps`, and a specific option wins over `--fps`.

| Option | Rule |
|:--- |:--- |
| `--max-cps CHARS` | `maxCharactersPerSecond` |
| `--max-cpl CHARS` | `maxCharactersPerLine` |
| `--max-lines LINES` | `maxLinesPerCue` |
| `--min-duration SECONDS` | `minDuration` |
| `--max-duration SECONDS` | `maxDuration` |
| `--min-gap SECONDS` | `minGap` |
| `--check-overlaps` | `noOverlap` |
| `--check-empty-cues` | `noEmptyCues` |
| `--max-wpm WORDS` | `maxWordsPerMinute` |
| `--min-seconds-per-word SECONDS` | `minSecondsPerWord` |
| `--max-speakers SPEAKERS` | `maxSpeakersPerCue` |
| `--dialogue-dash STYLE` | `dialogueDashStyle`. STYLE is the exact text of one of the 8 styles, such as `"- "` or `"-"` |
| `--allowed-characters LIST` | `allowedCharacters`. LIST is a list or a class, passed as it is |
| `--check-double-spaces` | `noDoubleSpaces` |
| `--check-leading-or-trailing-spaces` | `noLeadingOrTrailingSpaces` |
| `--check-unbalanced-tags` | `noUnbalancedTags` |
| `--check-all-caps-lines` | `noAllCapsLines` |

- **Usage errors**: a bad option value, a rule that `Rules.Validate` rejects, an unknown preset, `--video-fps` without `--preset netflix-en`, and a call without a preset or rule option (`Pass --preset or at least one rule option.`) exit with 2. The rules `requireCues` and `noNegativeDuration` have no option.
- **Text**: one line per violation, `LABEL: cue N: RULE VALUE` and `, limit LIMIT` for a rule with a limit. N counts from 1 and the part `cue N: ` is left out for a rule about the whole subtitle. Numbers have at most 3 decimals, and `INF` stands for an infinite value. An input without a violation prints `LABEL: no problems`.
- **JSON**: one object per input with the keys `file`, `format`, `valid`, `violations` and `warnings`. A violation has the keys `cueIndex` (0-based, `null` for no cue), `rule`, `value`, `infinite` and `limit`. `value` is `null` and `infinite` is `true` for an infinite value, because JSON has no infinity.
- **Summary**: with 2 or more inputs, `N files: V valid, P with problems, F failed.` follows the lines. It goes to stderr with `--json`.
- **Exit code**: 3 when an input failed, else 1 when an input has a violation, else 0.

## sync

`stbox sync <input>... --reference FILE [options]` retimes each input to the cue times of a reference subtitle. `--silence-log FILE --media-duration SECONDS` takes the speech of an FFmpeg `silencedetect` log as the reference instead. It finds the offset and the frame-rate scale. [sync.md](sync.md#command-line) lists its options and output.

`--reference-track NUMBER` picks the track when the reference is a video file. A video file with 2 or more subtitle tracks and no `--reference-track` fails with `Error: movie.mkv: The MKV or WebM file has 6 subtitle tracks. Pass --reference-track N with one of them:` and the track lines, exit 3. A PGS or VobSub reference track works without OCR, because only the cue times count.

## diff

`stbox diff OLD NEW` compares two files and prints the differences of [compare.md](compare.md). It exits 1 when the files differ.

- **Expansion**: only OLD goes through the input expansion. A directory or glob that gives 2 or more files fails with exit 2. NEW is used as given, and it cannot be `-`.
- **Input options**: `--from`, `--track`, `--encoding`, `--lenient`, `--input-fps` and `--fps` apply to both files. `--new-from` replaces `--from` for NEW, and `--new-track` replaces `--track`. Without `--new-track`, NEW takes `--track` only when it is a video file.
- **Failure**: a failed file prints `PATH: MESSAGE` with its own path and exits 3. A message about the format of NEW names `--new-from`, and a track error names `--new-track`.
- **No `--keep-going`**: the command has one pair of files.
- **JSON**: one object with the keys `oldFile`, `newFile`, `equal`, `differences`, `oldWarnings` and `newWarnings`. Cue times are seconds, such as `6.2`.

## translate

`stbox translate <input>... --engine deepl|google|openai --target-language CODE` sends the cue text to a translation service with `translation.Translate` of [translation.md](translation.md). It keeps the times and the tags, and sets the `language` metadata to the target. It is a write command, so the output rules of [Output files](#output-files) apply.

```
DEEPL_API_KEY=... stbox translate station.srt --engine deepl --source-language en --target-language de --to vtt
OPENAI_MODEL=llama3 OPENAI_BASE_URL=http://localhost:11434/v1 stbox translate station.srt --engine openai --target-language de
```

- **Options**: `--engine` takes `deepl`, `google` or `openai` in any ASCII case. `--target-language` is required. `--source-language` is optional, and the engine detects the language without it. The CLI does not check the language codes.
- **Key**: `--api-key` wins over the environment variable of the engine, also when it is empty. An empty variable counts as unset. Only the variable of the chosen engine counts.
- **Checks**: every check runs before any read and fails with exit 2. The order is engine, key, target, then for `openai` the model and the base URL, then the engine options.

| Variable | Engine | Effect |
|:--- |:--- |:--- |
| `DEEPL_API_KEY` | `deepl` | API key when `--api-key` is absent. Required |
| `GOOGLE_TRANSLATE_API_KEY` | `google` | The same. Required |
| `OPENAI_API_KEY` | `openai` | The same. Optional, because a local service needs no key |
| `OPENAI_MODEL` | `openai` | The model name. Required |
| `OPENAI_BASE_URL` | `openai` | The base URL. Default `https://api.openai.com/v1` |

- **Key rules**: a key of only white space gives `The API key is empty. Pass --api-key or set DEEPL_API_KEY.`. A key with a control character, or a space at the start or end, gives `The API key has a control character, or a space at the start or end. Pass the key without them.`. The key never appears in a message, a warning or the help.
- **Warnings**: each warning of the run goes to stderr as `LABEL: cue N: MESSAGE`, for example `station.srt: cue 3: The engine dropped or changed a placeholder tag. The cue has no tags.`.
- **Failure**: a service error fails the input with exit 3 and the message `LABEL: MESSAGE`, for example `stdin: DeepL rejected the API key (HTTP 403). Check the key and its plan.`. `--keep-going` goes on with the next input. Ctrl-C stops the request in flight.

## dual

`stbox dual --primary FILE --secondary FILE` merges two subtitles in two languages with `dual.FromPair` of [editing.md](editing.md#dual-subtitles). The output has the format of the primary file, unless `--to` sets another one.

```
stbox dual --primary movie.en.srt --secondary movie.de.srt --secondary-style i -o movie.en-de.srt
stbox dual --primary movie.en.srt --secondary movie.de.srt --mode top-bottom --to ass -o movie.en-de.ass
```

- **Files**: both go by option, and `dual` takes no positional argument. `--primary` can be `-` and goes through the input expansion, so a glob or directory that gives 2 or more files fails with exit 2. `--secondary` is used as given, and it cannot be `-`.
- **Input options**: `--primary-from` replaces `--from` and sets the format of the primary file. `--secondary-from` sets the format of the secondary file, which otherwise comes from the content. The secondary file does not inherit `--primary-from`. `--primary-track` replaces `--track` for the primary file, and `--secondary-track` sets the track of the secondary file, which does not inherit `--primary-track`. `--encoding`, `--lenient`, `--input-fps` and `--fps` apply to both files. A message about the format or the track names the option of its file.
- **Failure**: a failed secondary file prints `PATH: MESSAGE` with its own path and exits 3. The secondary file counts as a file that the command reads, so `-o` cannot name it. `--output-dir` takes the base name of the primary file.
- **Options**: `--mode` is `stack` or `top-bottom`. `--secondary-alignment` is a numpad value from 1 to 9, `--snap-tolerance` a time of 0 or more, and `--secondary-style` a core tag such as `i` or `font color="#ffff00"`.
- **No `--keep-going`**: the command has one pair of files.

## hls

`stbox hls INPUT --output-dir DIR` cuts a subtitle into WebVTT segments with `hls.Segment` of [hls.md](hls.md) and writes a VOD media playlist for them. The output is always WebVTT, so the command has no `--to`, `-o`, `--line-ending`, `--bom` or `--no-bom` option.

```
stbox hls film.srt --output-dir subs --segment 6
stbox hls film.srt --output-dir subs --segment 00:10 --mpegts 126000 --local 10 --pattern part%03d.vtt --playlist index.m3u8 --media-duration 00:02:30
```

| Option | Value | Default | Library |
|:--- |:--- |:--- |:--- |
| `--output-dir DIR` | a path. Required | | the directory of the segments and the playlist |
| `--segment SECONDS` | a time above 0 | 6 | `SegmentDuration` |
| `--playlist NAME` | a file name, may hold `../` | `subs.m3u8` | |
| `--pattern PATTERN` | a printf pattern with one `%d` | `sub%d.vtt` | `FileNamePattern` |
| `--mpegts TICKS` | a whole number of 0 or more | 900000 | `Map.MPEGTS` |
| `--local SECONDS` | a time of 0 or more | 0 | `Map.Local` |
| `--media-duration SECONDS` | a time above 0 | the end of the last cue | `MediaDuration` |

- **Input options**: `--from`, `--encoding`, `--lenient`, `--input-fps`, `--fps` and `--scc-roll-up` work as for `diff`. There is no `--keep-going`, because the command has one input.
- **Checks before the read**: each failure exits 2 and writes nothing.
  - 2 or more inputs: `The hls command takes one input file, got N.`
  - The playlist has the name of a segment, for example `--playlist sub0.vtt`.
  - The input is the playlist or a segment, so the run would overwrite it.
  - The playlist exists, or a file of the segment folder matches the pattern for any number. The run does not know the segment count before the read.
- **Segment folder**: `--output-dir` plus the part of `--pattern` up to the last `/` before the first `%` that is not `%%`. So `--pattern ../seg%d.vtt` puts the segments in `DIR/..`.
- **Pattern match**: a file name matches when the pattern reads a number of 0 or more from it, and printing that number with the pattern gives the same name. So `sub01.vtt` does not match `sub%d.vtt`.
- **Writing**: each segment, then the playlist. A failed write removes the files and the directories of the run, and the command exits 3.
- **Output**: `film.srt -> subs/subs.m3u8, 4 segments` on stdout. The directory has no trailing slash.
- **Failure**: a read error or a subtitle without a cue and without `--media-duration` exits 3.

## formats

`stbox formats` lists the formats that can be read or written, in registry order. The names are the values of `--from` and `--to`. It takes no argument and no option except `-h`.

```
Name                Extensions        Read  Write
ass                 .ass .ssa         yes   yes
cavena890           .890              yes   yes
csv                 .csv              yes   yes
itt                 .itt              yes   yes
json                .json             yes   yes
aws-transcribe      .json             yes   yes
lrc                 .lrc              yes   yes
mcc                 .mcc              yes   yes
microdvd            .sub              yes   yes
mpsub               .mpsub            yes   yes
pac                 .pac .fpc         yes   yes
pgs                 .sup              yes   yes
sami                .smi .sami        yes   yes
sbv                 .sbv              yes   yes
scc                 .scc              yes   yes
srt                 .srt              yes   yes
stl                 .stl              yes   yes
spruce-stl          .stl              yes   yes
subviewer           .sub              yes   yes
ttml                .ttml .dfxp .xml  yes   yes
dcp                 .xml              yes   yes
tsv                 .tsv              yes   yes
txt                 .txt              no    yes
mpl2                .txt              yes   yes
tmplayer            .txt              yes   yes
qttext              .txt              yes   yes
vobsub              .idx              yes   no
vtt                 .vtt              yes   yes
whisper             .json             yes   yes
podcast-transcript  .json             yes   yes
youtube-chapters    .txt              yes   yes
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
| 1 | result | `diff` found a difference, or `validate` found a broken rule. Not an error |
| 2 | invalid arguments | an unknown option, no input, an output that exists, `--encoding CP1125` |
| 3 | a file could not be read or written | a missing input, a broken cue in strict mode, an output directory that cannot be created |
| 130 | stopped by Ctrl-C or SIGTERM | the finished outputs stay |

The phase decides the code, not the error. [errors.md](errors.md#cli-exit-codes) holds the rules.
