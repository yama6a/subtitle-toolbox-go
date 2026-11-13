# Chapters

A chapter list names the parts of a video or a podcast episode. The library reads one into a `*model.Subtitle` whose cues are chapters. Package `format/chapters` holds the readers and writers of 4 formats: YouTube, Podcasting 2.0, FFmpeg metadata and OGM. Package `validation` holds the YouTube check.

The same chapter in each format:

```text
2:48 Hearing aids                              YouTube chapters
{"startTime": 168, "title": "Hearing aids"}    Podcasting 2.0 chapters
[CHAPTER] TIMEBASE=1/1000 START=168000 ...     FFmpeg metadata chapters
CHAPTER02=00:02:48.000                         OGM chapters
```

## Model

- **Cue**: one cue is one chapter. `Start` is the chapter start. `Lines` holds the title as one escaped line, see [markup.md](markup.md). A chapter without a title has no lines.
- **End**: a chapter list read from text has no ends in the file, so the reader sets them with the end rule.
- **Order**: cues stay in start order, as in every subtitle.

## End rule

The reader sorts the chapters by start. The sort is stable. Each chapter then ends at the first of these:

1. The end that the file gives. A chapter format that has ends moves each end with its chapter.
2. The start of the next chapter after the sort.
3. For the last chapter, the later of its own start and `ReadOptions.Chapters.MediaDuration`.

Example: chapters at 0 s, 168 s and 7200 s end at 168 s, 7200 s and 7200 s. With `MediaDuration` of 8000 s the last chapter ends at 8000 s. Without it, the length of the last chapter is unknown and counts as 0.

## Options

| Field | Type | Zero value | Meaning |
|:--- |:--- |:--- |:--- |
| `ReadOptions.Chapters.MediaDuration` | `time.Duration` | 0, unknown | where a last chapter without its own end ends |

- **Validate**: a negative `MediaDuration` fails with `The media duration must be 0 or more seconds, got -1.` and the option name `ReadOptions.Chapters.MediaDuration`.
- **Lenient mode**: the chapter readers ignore `ReadOptions.Lenient` for their own lines. Every error of the 4 formats fails the same way in both modes. The text pre-pass still honours it, so a NUL byte fails a strict read and reads with `Lenient`. The CLI prints no `--lenient` hint after a failed chapter read.
- **Write**: `chapters.WriteOptions` has no fields.

## YouTube chapters

The format name is `youtube-chapters` (`stbox.YouTubeChapters`). It uses `.txt`, which plain text owns, so detection never returns it. Pass `ReadOptions.Format` or `--from` and `--to`.

### Read

The reader takes the lines that start or end with a time and ignores all other lines. A whole video description works as input. It never fails on a line.

| Line | Result |
|:--- |:--- |
| `Recorded live.` | ignored |
| `0:00 Intro` | 0 s, `Intro` |
| `(2:48) Hearing aids` | 168 s, `Hearing aids` |
| `The big players: 1:31:50` | 5510 s, `The big players` |
| `12:30pm meeting` | ignored, because a letter follows the time |
| `0:75 Bad seconds` | ignored |
| `75:00 Long video` | 4500 s, `Long video` |
| `2:00:00` | 7200 s, no title |

- **Time**: an optional `(` or `[`, optional hours and `:`, minutes, `:`, 2 digits of seconds and an optional `)` or `]`.
- **Separator**: a white space character, `|`, `:`, `-`, an en dash or an em dash. A leading time must be followed by the end of the line or a separator.
- **Trailing time**: a line without a leading time can end with a time. The time starts at the line start or after a separator. The title is the text before the leftmost such start.
- **Backtracking**: `12:34:5x` is minutes 12, seconds 34 and the title `5x`, because the form with hours is not followed by a separator.
- **Range**: a line is ignored when the seconds are above 59, when the minutes are above 59 and the line has hours, or when the time is 100000 hours or more.
- **Title**: the separators at both ends go. The rest is escaped.
- **Cost**: the reader runs once over each line, so the time grows with the input size.

### Write

Each cue becomes one line: the start in whole seconds as `m:ss`, or `h:mm:ss` from one hour, a space and the plain title. A negative start writes as 0. Ends are lost. The line ending is LF and the BOM is off by default.

```text
0:00 Intro
59:59 Last minute
1:00:00
10:00:00 Ten hours
```

## Podcasting 2.0 chapters

The format name is `podcast-chapters` (`stbox.PodcastChapters`). It uses `.json`, which the library JSON owns, so detection never returns it. Pass `ReadOptions.Format` or `--from` and `--to`. The format of the [spec](https://github.com/Podcastindex-org/podcast-namespace/blob/c0ff5caa3729610362ee93f8034454fa41f3c493/docs/examples/chapters/jsonChapters.md) is a JSON object with a `chapters` list.

### Read

The reader decodes the file as [json.md](json.md) does, so a list needs a JSON array. Every error below fails in both modes.

| Input | Error |
|:--- |:--- |
| not JSON | `The content is not valid JSON: Syntax error.` |
| a root that is not an object | `The JSON root must be an object.` |
| no `chapters` array | `The JSON has no "chapters" list.` |
| `startTime` is missing, a string such as `"1:00"`, negative or `1e400`, or the chapter is no object | `The field chapters[0].startTime must be a number.` |
| `endTime` is present, not null and not such a number | `The field chapters[0].endTime must be a number.` |
| a time of 100000 hours or more | `The field chapters[0].startTime is not below 100000 hours.` |

- **Times**: seconds, rounded to whole milliseconds. The chapters are sorted by start and get their ends from the end rule, with `endTime` as the end that the file gives.
- **Title**: `title` when it is a string, escaped.
- **Metadata**: a top-level `title` and `author` that are strings set the title and author metadata.
- **Format data**: the file format data `podcast-chapters` holds the other top-level members, such as `version` and `podcastName`. The cue format data `podcast-chapters` holds the other members of a chapter, such as `img`, `url`, `toc` and `location`.

### Write

The document holds `version`, `author`, `title`, the other stored members and `chapters`, in this order. `version` is the stored one, else `1.2.0`. The output is pretty JSON with 4 spaces and a final LF.

- **Chapter**: `startTime`, then `endTime` only when the end is not the start of the next chapter (for the last chapter, its own start). Then `title`, the plain lines joined with a space, when it is not empty. Then the stored chapter members.
- **Numbers**: a whole second is an integer such as `168`. Else the number is the shortest float, such as `95.5`. A negative time is 0. A stored number keeps the form of the file, so `52.0` stays `52.0`.

```json
{
    "version": "1.2.0",
    "chapters": [
        {
            "startTime": 0,
            "title": "Intro"
        },
        {
            "startTime": 168,
            "title": "Hearing aids"
        },
        {
            "startTime": 260,
            "endTime": 300
        }
    ]
}
```

## FFmpeg metadata chapters

The format name is `ffmeta-chapters` (`stbox.FFMetadataChapters`). It is the `;FFMETADATA1` text that `ffmpeg -f ffmetadata` writes and `-map_metadata` reads. The extension `.ffmeta` maps to it in `FormatFromPath`, so an output file `meta.ffmeta` gets the format. `Load` of a `.ffmeta` file still needs `ReadOptions.Format`, because the extension fallback skips `ffmeta`. Detection never returns it.

### Read

- **Header**: the content must start with `;FFMETADATA`, else `The content does not start with the ;FFMETADATA header.` on line 1.
- **Lines**: CR LF and CR become LF. A line that ends with an odd number of backslashes continues on the next line, joined with LF, except the last line of the file. An empty line and a line that starts with `;` or `#` are dropped.
- **Sections**: tags before a section are global tags. `[STREAM]` starts a stream with its own tags. `[CHAPTER]` starts a chapter. The line must match exactly.
- **Chapter times**: only the lines right after `[CHAPTER]`, in this order, as FFmpeg reads them: `TIMEBASE=N/D`, `START=ticks` and `END=ticks`. Each is optional. Text after the digits is ignored. A later `START=` line is a normal tag.
- **Time base**: without `TIMEBASE` it is `1/1000000000`, which is nanoseconds. A 0 in `N` or `D`, or a number beyond 64 bits, fails with `The chapter time base 0/1000 is not valid.` and its line.
- **Start and end**: the ticks times N / D seconds, rounded half away from zero to whole milliseconds. A chapter without `START` starts at the `END` of the chapter before, when that one had an `END`, else at 0. A time of 100000 hours or more, also a number beyond 64 bits, fails with `The time "START=360000000000" is not below 100000 hours.` and its line. A negative time of -100000 hours or less, also a number beyond 64 bits, becomes -359999999.999 s, because `time.Duration` cannot hold more.
- **Tags**: `KEY=VALUE`, split at the first `=` that no backslash escapes. A backslash before any character stands for that character, in key and value. A line without such an `=` is ignored. A repeated key replaces the value and keeps the place of the first one.
- **Title**: the `title` tag of the chapter, escaped. A title with LF gives several lines.
- **Metadata**: the global tags `title`, `author`, `artist`, `album` and `language`.
- **Format data**: the file format data `ffmeta-chapters` is `{"tags": all global tags, "streams": [the tags of each stream]}`. The cue format data is `{"timeBase": "N/D", "tags": the chapter tags without title}`. The stored time base has no leading zeros, so `TIMEBASE=01/1000` is `1/1000`.

### Write

- **Header**: `;FFMETADATA1` and LF.
- **Global tags**: the stored tags in stored order, then those of the 5 metadata keys that the stored tags lack. A metadata key takes the current metadata value, and a removed key drops its tag. A tag with a null value is not written.
- **Streams**: `[STREAM]` and the stored tags of each stored stream.
- **Chapters**: `[CHAPTER]`, `TIMEBASE=`, `START=`, `END=`, then `title` (the plain lines joined with LF, when not empty) and the stored chapter tags without `title`. The time base is the stored one, else `1/1000`. The ticks are the time, 0 or more, times D / N, rounded half away from zero.
- **Escapes**: key and value get a backslash before `#`, `;`, `=`, `\` and LF.

```text
;FFMETADATA1
title=Notes on Bread Baking
[CHAPTER]
TIMEBASE=1/1000
START=0
END=30000
title=Opening credits
```

## OGM chapters

The format name is `ogm-chapters` (`stbox.OGMChapters`). It is the simple chapter format of `mkvmerge`. It uses `.txt`, which plain text owns, so detection never returns it. Pass `ReadOptions.Format` or `--from` and `--to`.

### Read

Each chapter has 2 lines.

```text
CHAPTER02=00:02:48.000
CHAPTER02NAME=Hearing aids
```

- **Lines**: split at LF, CR LF and CR, each trimmed. Empty lines are skipped.
- **Time line**: `CHAPTER`, digits, `=`, then hours, minutes and seconds as digits with `:` between them, then `.` or `,` and 1 to 9 digits. White space is allowed around each separator. Text after the digits is ignored. The fraction digits are a decimal fraction, so nanoseconds are allowed and round to milliseconds.
- **Name line**: the next line that is not empty must be `CHAPTER`, digits, `NAME`, `=` and the name. The name is escaped and trimmed.
- **Last line**: a time line at the end of the file without a name line is dropped without an error. The chapter numbers are not checked and not kept.
- **Errors**: `The line "L" is not a CHAPTERxx= line.`, `The line "L" is not a CHAPTERxxNAME= line.`, `The line "L" has a minute or second above 59.` and `The time "L" is not below 100000 hours.`, each with its line.
- **Format data and metadata**: none.

### Write

Each cue becomes 2 lines, numbered from 1: `CHAPTERnn=` and the start as `HH:MM:SS.mmm`, then `CHAPTERnnNAME=` and the plain lines joined with a space. `nn` has at least 2 digits, so `CHAPTER100` follows `CHAPTER99`. A negative start is 0. Ends are lost.

```text
CHAPTER01=00:00:00.000
CHAPTER01NAME=Intro
CHAPTER02=00:02:48.000
CHAPTER02NAME=Hearing aids
```

## YouTube chapter check

`validation.CheckYouTubeChapters(s)` lists the rules of [YouTube Help](https://support.google.com/youtube/answer/9884579) that a chapter list breaks. It returns violations as [validation.md](validation.md) describes. It is in `validation`, because a format package cannot import a service.

| Rule | Violation when | Cue | Value | Limit |
|:--- |:--- |:--- |:--- |:--- |
| `firstChapterAtZero` | the first start, floored to whole seconds, is above 0 | 0 | the first start in seconds | 0 |
| `minChapters` | fewer than `YouTubeMinChapters` (3) chapters | `NoCue` | the count | 3 |
| `minDuration` | a chapter is shorter than `YouTubeMinDuration` (10 s) | the chapter | its length in seconds | 10 |

- **Order**: the 2 whole-list rules come first, then `minDuration` by chapter.
- **Last chapter**: a last chapter of 0 s or less is skipped, because its length is unknown.
- **No settings**: `Rules` has no field for `minChapters` and `firstChapterAtZero`. Only this function reports them.

Example: chapters 5 to 9.5 s and 9.5 to 30 s give `firstChapterAtZero` (5 s), `minChapters` (2) and `minDuration` (cue 0, 4.5 s).
