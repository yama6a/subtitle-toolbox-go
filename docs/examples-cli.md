# Command line examples

Each section is one task, done with the `stbox` command. The output comes from a real run. A long output is cut, and a line with `...` marks the cut. [examples-library.md](examples-library.md) does the same tasks in Go. [cli.md](cli.md) lists every option.

No command overwrites a file. An output file such as `-o station.en.vtt` must not exist. Every command here that writes SubRip or WebVTT has `--no-bom`, so the output holds no UTF-8 BOM.

## Convert SubRip to WebVTT for an HTML5 video player

The `<track>` element of a `<video>` tag reads WebVTT, not SubRip. `--to vtt` sets the format. Without `-o`, the output goes to standard output.

```sh
stbox convert station.en.srt --to vtt --no-bom -o station.en.vtt
```

```text
station.en.srt -> station.en.vtt
```

The file `station.en.vtt` starts like this:

```text
WEBVTT

1
00:00:01.000 --> 00:00:04.000
The train to Hamburg leaves from platform 4.

2
00:00:04.500 --> 00:00:07.250
It is ten minutes late today.
...
```

## Fix a subtitle that runs late

A German subtitle shows every line 1.2 s after the speech. `--shift` with a negative number moves every cue earlier.

```sh
stbox convert station.de.srt --shift -1.2 --no-bom -o station.de.fixed.srt
```

```text
station.de.srt -> station.de.fixed.srt
```

The first cue moves from 1.2 s to 0 s:

```text
1
00:00:00,000 --> 00:00:02,700
Der Zug nach Hamburg fährt von Gleis 4.
```

## Fix drift from a PAL speed-up

A subtitle was timed for a 25 fps release, and the video runs at 23.976 fps. `--from-fps` is the frame rate that the subtitle fits now. `--to-fps` is the frame rate of the video. Pass both or neither.

```sh
stbox convert bakery.de.srt --from-fps 25 --to-fps 23.976 --no-bom -o bakery.de.23976.srt
```

```text
bakery.de.srt -> bakery.de.23976.srt
```

Every time grows by 25 / 23.976. The first cue moves from 1:16.738 to 1:20.015:

```text
1
00:01:20,015 --> 00:01:21,486
Die Bäckerei öffnet jeden Morgen um sechs.
```

## Sync a subtitle to a reference subtitle

The file is late and also drifts. `stbox sync` finds the scale and the offset against a file of the same film that is in sync. The text of the cues does not matter, so the reference can be in another language.

```sh
stbox sync bakery.de.srt --reference bakery.en.srt --no-bom -o bakery.de.synced.srt
```

```text
bakery.de.srt: scale 1.04271, offset -2.3 s, score 0.89
bakery.de.srt -> bakery.de.synced.srt
```

The line with the scale goes to standard error. The score runs from 0 to 1, and a score below 0.5 means that the files likely do not match. An ad break needs `--max-splits`. See [sync.md](sync.md#command-line).

## Sync a subtitle to the speech of the audio

Without a subtitle in sync, use the silences of the audio. FFmpeg writes them to a log.

```sh
ffmpeg -i bakery.mkv -af silencedetect=noise=-30dB:d=0.4 -f null - 2> silence.log
stbox sync bakery.de.srt --silence-log silence.log --media-duration 00:14:00 --no-bom -o bakery.de.speech.srt
```

```text
bakery.de.srt: scale 1.04271, offset -2.3 s, score 0.78
bakery.de.srt -> bakery.de.speech.srt
```

`--media-duration` is the length of the video. The score is lower than against a subtitle, because speech starts later and ends earlier than its cue. The same options also take a transcript with times, such as Whisper JSON, as `--reference`.

## Check a file against broadcast or streaming rules

`--preset netflix-en` has the limits of the Netflix English style guide. `--preset bbc` has the limits of the BBC guidelines. `stbox validate` prints one line for each broken rule, and exits with 1 when it finds one.

```sh
stbox validate trip.srt --preset netflix-en
```

```text
trip.srt: cue 2: maxCharactersPerLine 57, limit 42
trip.srt: cue 2: maxCharactersPerSecond 27.6, limit 20
trip.srt: cue 2: noOverlap 0.5
trip.srt: cue 3: maxCharactersPerSecond 42.5, limit 20
trip.srt: cue 3: minDuration 0.4, limit 0.833
```

The BBC preset checks the reading speed in words:

```sh
stbox validate trip.srt --preset bbc
```

```text
trip.srt: cue 2: maxCharactersPerLine 57, limit 37
trip.srt: cue 2: maxWordsPerMinute 336, limit 180
trip.srt: cue 2: minSecondsPerWord 0.179, limit 0.3
trip.srt: cue 3: maxWordsPerMinute 450, limit 180
trip.srt: cue 3: minSecondsPerWord 0.133, limit 0.3
```

A rule option overrides the preset, for example `--max-cpl 40`. `--json` prints the result for a script. See [validation.md](validation.md#command-line).

## Clean up a subtitle

`--sdh` removes hearing-impaired annotations, such as `[TRAIN WHISTLE BLOWS]`, `(sighs)` and `CONDUCTOR:`. It also removes a cue that has no text left. The labels in lower case, such as `Baker:`, stay.

```sh
stbox convert train.srt --sdh --no-bom
```

```text
1
00:00:03,500 --> 00:00:06,000
The train to the coast
is late again.

2
00:00:06,500 --> 00:00:09,000
- Look at the snow!
- Tickets, please.

3
00:00:12,500 --> 00:00:15,000
Is the bakery open?

4
00:00:15,500 --> 00:00:18,000
Baker: We open at six.

5
00:00:18,500 --> 00:00:21,000
<i>The bread is still warm.</i>

6
00:00:21,500 --> 00:00:24,000
Two loaves, please.

7
00:00:24,500 --> 00:00:27,000
♪ Rain on the window ♪

8
00:00:27,500 --> 00:00:30,000
Note: the last train leaves at 11:40.

9
00:00:33,500 --> 00:00:36,000
Fish &amp; chips {chuckles} for two.

10
00:00:36,500 --> 00:00:39,000
Sit down, please.

11
00:00:39,500 --> 00:00:42,000
- # The wheels go round #
- It stopped raining.
```

A second file has typos from a bad download: `Quiet , please !`, `We landed late.It was cold,but dry.` and `-Did you call?`. `--errors-fix` fixes the spacing, the dashes, the dots and the tags. `--language en` sets the language rules, and without it the OCR fixes such as `lt's` to `It's` do not run. `--structure-wrap` breaks lines longer than 42 characters, such as the fare in cue 7.

```sh
stbox convert web.srt --errors-fix --language en --structure-wrap --no-bom
```

```text
1
00:00:01,000 --> 00:00:03,200
Hello <i>there</i>, how was the trip?

2
00:00:03,600 --> 00:00:06,000
We landed late. It was cold, but dry.

3
00:00:06,400 --> 00:00:09,000
<i>The station was closed,
so we walked.</i>

4
00:00:09,400 --> 00:00:11,000
Take the second exit.

5
00:00:11,400 --> 00:00:13,500
- Did you call?
- Not yet.

6
00:00:13,900 --> 00:00:16,000
I waited... and waited...

7
00:00:16,400 --> 00:00:19,000
The fare was 1.5 euros,
e.g. on www.example.com.

8
00:00:19,400 --> 00:00:21,000
Quiet, please!

9
00:00:21,400 --> 00:00:23,000
Gate 4 is open now.

10
00:00:23,400 --> 00:00:25,500
Mr. Berg is here. So am I.

11
00:00:25,900 --> 00:00:28,000
<font color="#ffff00">Next stop:
the harbour</font>

12
00:00:28,400 --> 00:00:30,000
This line has no errors.
```

## Extract a subtitle track from an MKV or MP4 file

`info` lists the subtitle tracks of a video file. `--track` picks one. Without `--track`, a file with 2 or more subtitle tracks fails.

```sh
stbox info ferry.mkv
```

```text
ferry.mkv
  Container: matroska
  Track 2: S_TEXT/UTF8, en, "English", default
  Track 3: S_TEXT/ASS, de
```

```sh
stbox convert ferry.mkv --track 2 --to srt --no-bom -o ferry.en.srt
```

```text
ferry.mkv -> ferry.en.srt
```

The file `ferry.en.srt` starts like this:

```text
1
00:00:01,200 --> 00:00:03,800
The ferry to the island leaves at nine.
...
```

## Read Blu-ray or DVD image subtitles with OCR

A `.sup` file holds Blu-ray subtitles as images. `--ocr` reads their text. `--ocr-engine glyph` uses the engine that is built in. `--ocr-engine tesseract` reads more languages and needs the `tesseract` program, or the `-tesseract` container image. A DVD works the same way with the `.idx` file of the VobSub pair. See [ocr.md](ocr.md).

```sh
stbox convert film.sup --ocr --ocr-engine glyph --to srt --no-bom -o film.srt
```

```text
film.sup: OCR 12/12
film.sup -> film.srt
```

The file `film.srt` starts like this:

```text
1
00:00:01,000 --> 00:00:03,500
The train to Bergen leaves at 7:45.

2
00:00:04,000 --> 00:00:07,000
The bakery opens at six.
Fresh bread is ready by seven.
...
```

## Turn a Whisper transcript into readable subtitles

`--structure-resegment` builds new cues from the word times. Each cue holds one sentence, or what fits `--structure-max-lines` lines of `--structure-max-cpl` characters. `stbox` detects Whisper JSON from its content.

```sh
stbox convert talk.json --structure-resegment --to srt --no-bom -o talk.srt
```

```text
talk.json -> talk.srt
```

The first segment of `talk.json` runs 11.8 s and holds 3 sentences. It becomes the first 3 cues:

```text
1
00:00:00,000 --> 00:00:02,550
Welcome to the city library tour.

2
00:00:02,550 --> 00:00:08,550
The reading room is on the left, and
the study rooms are on the second floor.

3
00:00:08,550 --> 00:00:11,800
Please keep your voice down in both rooms.
...
```

## Translate a subtitle with DeepL

`stbox translate` sends the cue text to the service and keeps the times and the tags. The key comes from `--api-key` or from the variable `DEEPL_API_KEY`.

```sh
export DEEPL_API_KEY=your-key
stbox translate platform.srt --engine deepl --source-language en --target-language de --no-bom -o platform.de.srt
```

This page shows no output, because the command needs a key and the network. The test suite runs the same translation against a saved DeepL answer. [examples-library.md](examples-library.md#translate-a-subtitle-with-deepl) shows its result. `--engine google` and `--engine openai` work the same way. See [translation.md](translation.md).

## Build dual subtitles for language learners

`stbox dual` puts the lines of a second language under the cue of the first language that it overlaps most. A joined cue starts at the earlier start and ends at the later end of its cues. `--secondary-style i` sets the second language in italics.

```sh
stbox dual --primary station.en.srt --secondary station.de.srt --secondary-style i --no-bom -o station.dual.srt
```

```text
station.en.srt -> station.dual.srt
```

The file `station.dual.srt` starts like this:

```text
1
00:00:01,000 --> 00:00:04,000
The train to Hamburg leaves from platform 4.
<i>Der Zug nach Hamburg fährt von Gleis 4.</i>

2
00:00:04,400 --> 00:00:07,400
It is ten minutes late today.
<i>Er hat heute zehn Minuten Verspätung.</i>
...
```

`--mode top-bottom` shows the second language at the top of the screen. See [editing.md](editing.md#dual-subtitles).

## Compare two versions of a subtitle

A translator sends back a corrected file. `stbox diff` lists each cue that was added, removed or changed. The exit code is 1 when the files differ, and 0 when they are the same.

```sh
stbox diff episode_v1.srt episode_v2.srt
```

```text
timing changed: old cue 2, new cue 2
- 00:00:04.000 --> 00:00:06.200
  This train calls at all stations
  to the harbour.
+ 00:00:04.250 --> 00:00:06.450
  This train calls at all stations
  to the harbour.

text changed: old cue 4, new cue 4
- 00:00:10.500 --> 00:00:13.000
  The next stop is Milbrook.
+ 00:00:10.500 --> 00:00:13.000
  The next stop is Millbrook.

text and timing changed: old cue 6, new cue 6
...
```

The full output has 9 entries. See [compare.md](compare.md).

## Walk a folder and report statistics

A folder holds subtitle files in mixed formats. `stbox info` reads each file, detects its format and prints the statistics. A folder as argument reads every subtitle file in it. The command reports each file that fails and goes on with `--keep-going`.

```sh
stbox info season1
```

```text
season1/ep1.srt
  Format:                srt
  Encoding:              UTF-8
  Cues:                  6
  Words:                 32
  Characters:            189
  Display time:          14.1 s
  Span:                  19.8 s
  Characters per second: min 6.111, average 14.305, max 23.529
  Words per minute:      min 66.667, average 143.29, max 247.059
  Characters per line:   min 11, average 31.5, max 40
  Gaps:                  min 0, average 1.14, max 2.6 s
  Most used words:       die (2), der (1), zug (1), nach (1), hamburg (1),
                         fährt (1), von (1), gleis (1), 4 (1), er (1)

season1/ep2.srt
...
3 files: 3 succeeded, 0 failed.
```

`--json` prints the same data for a script. The same folder works with `convert`. `stbox convert season1 --to vtt --output-dir out` writes `out/ep1.vtt`, `out/ep2.vtt` and `out/ep3.vtt`. See [statistics.md](statistics.md).

## Create a subtitle from scratch

The command has no editor. Write the cues in any text format that you know, and let `stbox` convert them. A dash in place of a file name reads standard input.

```sh
printf '1\n00:00:01,000 --> 00:00:03,000\nWelcome aboard.\n' | stbox convert - --from srt --to vtt --no-bom
```

```text
WEBVTT

1
00:00:01.000 --> 00:00:03.000
Welcome aboard.
```

For a program that builds cues, use the library. See [examples-library.md](examples-library.md#create-a-subtitle-from-scratch).
