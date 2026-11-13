# Chapters

A chapter list names the parts of a video or a podcast episode. The library reads one into a `*model.Subtitle` whose cues are chapters. Package `format/chapters` holds the readers and writers. Package `validation` holds the YouTube check.

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
- **Lenient mode**: the chapter readers ignore `ReadOptions.Lenient` for their own lines. The text pre-pass still honours it, so a NUL byte fails a strict read and reads with `Lenient`. The CLI prints no `--lenient` hint after a failed chapter read.
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
