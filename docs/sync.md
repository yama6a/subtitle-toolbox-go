# Sync

A German SubRip file for a 25 fps release runs late and drifts against a 23.976 fps video. An English file for that video is in sync. `refsync.Apply(german, refsync.Options{Reference: english})` finds the scale 1.04271 (25/23.976), the offset -2.3 s and a score of 0.89, then retimes the German file. [editing.md](editing.md#retiming) covers a shift, a scale and a sync by known points.

## Sync to a reference subtitle

<!-- example: refsync/ExampleApply -->
```go
at := func(start, end float64, text string) model.Cue {
	return model.Cue{
		Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)),
		Lines: []string{text},
	}
}
var english, german model.Subtitle
english.AddCues(
	at(10, 12.5, "Where is the station?"),
	at(20, 23, "Turn left at the bakery."),
	at(31, 33, "Thank you."),
	at(45, 47.5, "The last train leaves at ten."),
)
german.AddCues(
	at(10.79, 13.188, "Wo ist der Bahnhof?"),
	at(20.381, 23.258, "Links an der Ecke."),
	at(30.93, 32.848, "Danke."),
	at(44.357, 46.755, "Der letzte Zug geht um zehn."),
)

r, err := refsync.Apply(&german, refsync.Options{Reference: &english})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Printf("scale %.5f, offset %s, score %.2f\n", r.Scale, r.Offset, r.Score)
for _, c := range german.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
// Output:
// scale 1.04271, offset -1.25s, score 1.00
// 10.001s 12.501s Wo ist der Bahnhof?
// 20.001s 23.001s Links an der Ecke.
// 31.001s 33.001s Danke.
// 45.001s 47.502s Der letzte Zug geht um zehn.
```

- **Times only**: the text of the cues never counts, so the reference can be in another language. The idea comes from [alass](https://github.com/kaegi/alass).
- **In place**: `Apply` retimes the subtitle that you pass, with `Scale` and then `Shift`. Pass `s.Clone()` to read the report only.
- **Span**: a time interval that at least 1 cue covers. Time that 2 cues of one file share counts once.
- **Scale**: 1, or one of 24/23.976, 25/24, 25/23.976 and their inverses. No other factor is found. One scale applies to the whole file. `NoScale` keeps it at 1.
- **Offset**: from `MinOffset` to `MaxOffset`, -60 s to 60 s by default, to 0.01 s.
- **Score**: the time that both files cover after the sync, divided by the time that either file covers, from 0 to 1. Below 0.5 the files likely do not match. Missing and extra cues lower the score.
- **No cues**: a subtitle without a cue of length above 0, on either side, gives scale 1, offset 0, score 0, and nothing changes.
- **Limits**: each offset is from -86400 s to 86400 s, and the 2 offsets are at most 7200 s apart. The split search keeps a value for each 0.1 s of that range. `MaxSplits` is from 0 to 10.
- **Errors**: a bad option and a cue that would end at 100,000 hours or later return an error that matches `ErrInvalidArgument`. The subtitle then keeps all its times.

## Sync to speech

Without a subtitle in sync, the speech in the audio can serve as the reference. The library reads no audio and runs no voice detection. It turns the speech between silences into cues without text and passes them to `Apply` as `Options.Reference`. Apply uses only cue times, so such a reference works like any other.

Three inputs give a speech reference:

| Input | Go | Command line |
|:--- |:--- |:--- |
| the stderr log of the FFmpeg `silencedetect` filter, plus the media duration | `SpeechFromSilenceLog(log, mediaDuration)` | `--silence-log` and `--media-duration` |
| speech intervals from any voice activity detector | `SpeechFromIntervals(intervals)` | none |
| a transcript with times, such as Whisper JSON | `Load` of the transcript, then `Options.Reference` | `--reference whisper.json` |

The user runs FFmpeg. This command writes the silences of the audio:

```
ffmpeg -i movie.mkv -af silencedetect=noise=-30dB:d=0.4 -f null - 2> silence.log
```

<!-- example: refsync/ExampleSpeechFromSilenceLog -->
```go
log := []byte("[silencedetect @ 0x1] silence_start: 0\n" +
	"[silencedetect @ 0x1] silence_end: 2.5 | silence_duration: 2.5\n" +
	"[silencedetect @ 0x1] silence_start: 4.25\n" +
	"[silencedetect @ 0x1] silence_end: 6 | silence_duration: 1.75\n")

speech, err := refsync.SpeechFromSilenceLog(log, 10*time.Second)
if err != nil {
	fmt.Println(err)
	return
}
for _, c := range speech.All() {
	fmt.Println(c.Start, c.End)
}
// Output:
// 2.5s 4.25s
// 6s 10s
```

- **Log reader**: a line counts when it holds `silence_start:` or `silence_end:` with a number. Other lines, such as the banner and the progress lines, do not matter. The `silence_start` and `silence_end` lines must alternate. A log without such lines gives one cue from 0 to the media duration.
- **Times**: each time is clamped to 0 up to the media duration and rounded to whole milliseconds. A time that is below the time before it counts as that one.
- **Errors**: a media duration of 0 or less, or of 100000 hours or more, and a bad interval return an error that matches `ErrInvalidArgument`. A log with a line per channel (`silencedetect=mono=1`) or with lines out of order returns a `*ParseError` with an empty `Format` and the 1-based `Line`.
- **Intervals**: `SpeechFromIntervals` needs a start of 0 or more and an end at or after its start and below 100000 hours. It sorts the cues by start. An interval of length 0 gives a cue of 0 ms, which `Apply` skips.
- **Score**: speech starts later and ends earlier than its cue. So the score is lower than against a subtitle. The German example scores 0.78 against the speech and 0.89 against the English subtitle. The low score line below 0.5 still applies.
- **Bound**: the log reader scans each byte once. Its time and memory grow with the size of the log. `BenchmarkSpeechFromSilenceLog` runs a log of 1 MB.

## Splits

An English TV recording has a 2:30 ad break after the cue at 45 s. The German file of the streaming release has no break and runs 2 s late. A **split** is a point where the offset jumps. A **segment** is the part between 2 splits, and it gets its own offset. All segments share the scale.

<!-- example: ExampleApply_splits -->
```go
at := func(start, end float64, text string) model.Cue {
	return model.Cue{
		Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)),
		Lines: []string{text},
	}
}
var englishTV, german model.Subtitle
englishTV.AddCues(
	at(10, 12.5, "Where is the station?"),
	at(20, 23, "Turn left at the bakery."),
	at(31, 33, "Thank you."),
	at(45, 47.5, "The last train leaves at ten."),
	at(210, 212.5, "Is this seat free?"),
	at(221, 224, "Yes, please sit down."),
	at(232, 234, "Where are you going?"),
	at(246, 248.5, "To my sister in the north."),
)
german.AddCues(
	at(12, 14.5, "Wo ist der Bahnhof?"),
	at(22, 25, "Links an der Ecke."),
	at(33, 35, "Danke."),
	at(47, 49.5, "Der letzte Zug geht um zehn."),
	at(62, 64.5, "Ist dieser Platz frei?"),
	at(73, 76, "Ja, bitte setzen Sie sich."),
	at(84, 86, "Wohin fahren Sie?"),
	at(98, 100.5, "Zu meiner Schwester im Norden."),
)

r, err := refsync.Apply(&german, refsync.Options{
	Reference: &englishTV,
	MinOffset: new(-180 * time.Second),
	MaxOffset: new(180 * time.Second), // the part after the break needs 148 s
	MaxSplits: 2,
})
if err != nil {
	fmt.Println(err)
	return
}
for _, seg := range r.Segments {
	to := "nil"
	if seg.To != nil {
		to = seg.To.String()
	}
	fmt.Printf("from %s, to %s, scale %.5f, offset %s\n", seg.From, to, seg.Scale, seg.Offset)
}
fmt.Printf("offset %s, score %.2f\n", r.Offset, r.Score)
// Output:
// from 0s, to 1m2s, scale 1.00000, offset -2s
// from 1m2s, to nil, scale 1.00000, offset 2m28s
// offset -2s, score 1.00
```

- **Report offset**: `r.Offset` is the offset of the first segment.
- **From**: the start of a cue before the sync. A cue goes to the last segment whose `From` is at or before its start. Splits fall between cues.
- **Penalty**: a split stays only when it raises the score by more than `SplitPenalty`, 0.1 by default. A segment with 5 % of the cue time raises the score by about 0.1. In unrelated files with 60 cues, a chance split raises it by up to 0.12, so such short files need a penalty above 0.12.
- **Overlaps**: a segment can move onto the next one. Then each cue of the earlier segment that starts before the first cue of the later segment and overlaps it ends 1 ms before that cue. An earlier cue that starts after that cue keeps its end.
- **Order**: segments that swap order swap their cues, so the cues stay in start order.
- **Time limit**: the 100,000 hour check runs on the mapped times before any change, as it does for `Scale` and `Shift`.

### Why the search uses blocks

The split search cuts the target into 200 blocks and moves each split to the best span near its block boundary afterwards. A search per span is fast enough in Go, but it can pick other splits than PHP. The PHP fixtures `own_target_de_split_synced.srt` and `own_target_de_split_penalty_synced.srt` need the same results, so Go ports the PHP formulas and loop orders, and every tie uses an epsilon of `1e-9`.

## Command line

```
stbox sync de.srt --reference tv.srt --min-offset -180 --max-offset 180 --max-splits 2 -o synced.srt
```

| Option | Value | Default |
|:--- |:--- |:--- |
| `--reference FILE` | a subtitle in any readable format. Exactly one of `--reference` and `--silence-log` | |
| `--reference-track NUMBER` | a whole number of 1 or more, the track of a video file | the only subtitle track |
| `--silence-log FILE` | the FFmpeg `silencedetect` log, a text file | |
| `--media-duration SECONDS` | a positive time, so `00:14:00` works. Only with `--silence-log`, and then required | |
| `--min-offset SECONDS` | a time, may be negative | -60 |
| `--max-offset SECONDS` | a time | 60 |
| `--no-scale` | keep the scale at 1 | scale search on |
| `--max-splits SPLITS` | 0 to 10 | 0 |
| `--split-penalty SCORE` | 0 or more | 0.1 |

The output, input and batch options of [cli.md](cli.md) apply. For each input, stderr gets the result lines before the write:

```
de.srt: scale 1.04271, offset -2.31 s, score 0.89
de.srt: from 0 s: offset -2.31 s
de.srt: from 414.32 s: offset 147.7 s
```

Then the report line `de.srt -> synced.srt` goes to stdout, as for `convert`.

- **Result line**: the scale has at most 5 decimals, the offset at most 3 and the score at most 2. With 2 or more segments, 1 line per segment follows.
- **Low score**: below 0.5, the line `de.srt: the score is below 0.5, so the files likely do not match.` follows. The exit code stays 0.
- **Checks**: exit 2 before any file is read, in this order. Both or none of `--reference` and `--silence-log`, or `--media-duration` alone: `Pass one of --reference FILE and --silence-log FILE.` Then `--silence-log` without `--media-duration`: `Pass --media-duration with --silence-log.` Then `--reference-track` without `--reference`: `Pass --reference with --reference-track.` Then the values of `--media-duration` and `--reference-track`, then the offset options.
- **Silence log**: it loads once before the first input. A missing file prints `Error: missing.log: The file does not exist.` and exits 3. A log that fails prints `Error: bad.log: line 2: ...` and exits 3, with no `--lenient` hint. No output may replace the log.
- **Reference read**: content detection picks the format, so `--from` does not apply. `--encoding`, `--lenient`, `--input-fps` and `--fps` apply. Its warnings print with its path. A missing or broken reference prints `Error: PATH: MESSAGE` and exits 3 before the first input.
