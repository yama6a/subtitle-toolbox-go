# Library examples

Each section is one task that a developer has, with code that runs as written. A test runs every block and checks the printed output. The code shows the body of a function and leaves out the imports. Add the root package `stbox "github.com/yama6a/subtitle-toolbox-go/v4"` and every other package that the code names.

The file paths point into the `testdata` folder of this repository. Replace them with your own files. [examples-cli.md](examples-cli.md) does the same tasks with the `stbox` command.

## Convert SubRip to WebVTT for an HTML5 video player

The `<track>` element of a `<video>` tag reads WebVTT, not SubRip. `Load` detects the format and the encoding of the input. `Write` writes the WebVTT. To write a file, pass `os.Create("station.en.vtt")` in place of `os.Stdout`. WebVTT output starts with a UTF-8 BOM by default, and `BOMRemove` leaves it out.

<!-- example: ExampleLoad_toWebVTT -->
```go
s, _, err := stbox.Load("testdata/dual/station_en.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.WebVTT, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// WEBVTT
//
// 1
// 00:00:01.000 --> 00:00:04.000
// The train to Hamburg leaves from platform 4.
//
// 2
// 00:00:04.500 --> 00:00:07.250
// It is ten minutes late today.
//
// 3
// 00:00:08.000 --> 00:00:11.000
// The bakery next to the station opens at six.
// They sell fresh rolls.
//
// 4
// 00:00:12.000 --> 00:00:13.500
// <i>Rain is coming.</i>
//
// 5
// 00:00:16.000 --> 00:00:19.000
// Take an umbrella with you.
```

## Fix a subtitle that runs late

A German subtitle shows every line 1.2 s after the speech. `Shift` with a negative duration moves every cue earlier. A time that would fall below 0 becomes 0. [editing.md](editing.md#retiming) covers a shift of a range of cues and a sync by known points.

<!-- example: ExampleLoad_shift -->
```go
s, _, err := stbox.Load("testdata/dual/station_de.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
if err := s.Shift(-1200 * time.Millisecond); err != nil {
	fmt.Println(err)
	return
}
for _, c := range s.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
// Output:
// 0s 2.7s Der Zug nach Hamburg fährt von Gleis 4.
// 3.2s 6.2s Er hat heute zehn Minuten Verspätung.
// 6.7s 8.4s Die Bäckerei am Bahnhof öffnet um sechs.
// 8.4s 9.9s Sie verkauft frische Brötchen.
// 12s 15.4s Morgen scheint wieder die Sonne.
// 18s 19.8s Gute Reise!
```

## Fix drift from a PAL speed-up

A subtitle was timed for a 25 fps release. The video runs at 23.976 fps, so the subtitle drifts more the longer the film runs. `ConvertFrameRate` scales every time by 25 / 23.976, about 1.0427. The first cue of this file moves by 3.3 s, and the last cue moves by about 33 s. [frame-rates.md](frame-rates.md) lists the named frame rates.

<!-- example: ExampleLoad_frameRate -->
```go
s, _, err := stbox.Load("testdata/sync/own_target_de_25fps.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
first, _ := s.Cue(0)
fmt.Println("before:", first.Start)
if err := s.ConvertFrameRate(stbox.FrameRate25, stbox.FrameRate23976); err != nil {
	fmt.Println(err)
	return
}
first, _ = s.Cue(0)
fmt.Println("after: ", first.Start)
// Output:
// before: 1m16.738s
// after:  1m20.015s
```

## Sync a subtitle to a reference subtitle

A German file is late, and it also drifts against the 23.976 fps video. An English file for the same video is in sync. `refsync.Apply` compares the cue times of both files and ignores the text. It finds the scale and the offset, and it retimes the German file. The score is the share of time that both files cover, from 0 to 1.

<!-- example: ExampleLoad_syncToReference -->
```go
german, _, err := stbox.Load("testdata/sync/own_target_de_25fps.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
english, _, err := stbox.Load("testdata/sync/own_reference_en.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}

report, err := refsync.Apply(german, refsync.Options{Reference: english})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Printf("scale %.5f, offset %s, score %.2f\n", report.Scale, report.Offset, report.Score)
first, _ := german.Cue(0)
fmt.Println("first cue:", first.Start)
// Output:
// scale 1.04271, offset -2.3s, score 0.89
// first cue: 1m17.715s
```

The sync can also split at an ad break. See [sync.md](sync.md#splits).

## Sync a subtitle to the speech of the audio

No file is in sync, but the video has audio. FFmpeg writes the silences of the audio to a log:

```sh
ffmpeg -i bakery.mkv -af silencedetect=noise=-30dB:d=0.4 -f null - 2> silence.log
```

`SpeechFromSilenceLog` turns the silences into cues without text, and `refsync.Apply` uses them as the reference. The score is lower than against a subtitle, because speech starts later and ends earlier than its cue. This file scores 0.78.

<!-- example: ExampleLoad_syncToSpeech -->
```go
german, _, err := stbox.Load("testdata/sync/own_target_de_25fps.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
log, err := os.ReadFile("testdata/sync/own_ffmpeg_silencedetect.log")
if err != nil {
	fmt.Println(err)
	return
}
speech, err := refsync.SpeechFromSilenceLog(log, 14*time.Minute)
if err != nil {
	fmt.Println(err)
	return
}

report, err := refsync.Apply(german, refsync.Options{Reference: speech})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Printf("scale %.5f, offset %s, score %.2f\n", report.Scale, report.Offset, report.Score)
// Output:
// scale 1.04271, offset -2.3s, score 0.78
```

## Check a file against broadcast or streaming rules

A delivery spec limits the characters per line, the reading speed, the cue length and the gap between cues. A preset holds the limits of a spec. `validation.NetflixEnglish` holds the Netflix English rules, and `validation.BBC` holds the BBC rules. `Check` returns one `Violation` for each broken rule. `v.Cue` counts from 0, so the code prints `v.Cue+1`. [validation.md](validation.md) lists the 19 rules.

<!-- example: ExampleLoad_validate -->
```go
s, _, err := stbox.Load("testdata/cli/trip.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
rules, err := validation.NetflixEnglish(stbox.FrameRate23976)
if err != nil {
	fmt.Println(err)
	return
}
violations, err := validation.Check(s, rules)
if err != nil {
	fmt.Println(err)
	return
}
for _, v := range violations {
	fmt.Println("cue", v.Cue+1, v.Rule, v.Value)
}
// Output:
// cue 2 maxCharactersPerLine 57
// cue 2 maxCharactersPerSecond 27.6
// cue 2 noOverlap 0.5
// cue 3 maxCharactersPerSecond 42.5
// cue 3 minDuration 0.4
```

## Clean up a subtitle

A subtitle from the web has hearing-impaired annotations, such as `[TRAIN WHISTLE BLOWS]`, `(sighs)` and `CONDUCTOR:`. `hearingimpaired.Apply` removes them. It also removes a cue that has no text left. `Lyrics` removes the text between music symbols. The labels in lower case, such as `Baker:`, stay.

<!-- example: ExampleLoad_removeHearingImpaired -->
```go
s, _, err := stbox.Load("testdata/hearing-impaired/own_sdh.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := hearingimpaired.Apply(s, hearingimpaired.Options{Lyrics: true}); err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:03,500 --> 00:00:06,000
// The train to the coast
// is late again.
//
// 2
// 00:00:06,500 --> 00:00:09,000
// - Look at the snow!
// - Tickets, please.
//
// 3
// 00:00:12,500 --> 00:00:15,000
// Is the bakery open?
//
// 4
// 00:00:15,500 --> 00:00:18,000
// Baker: We open at six.
//
// 5
// 00:00:18,500 --> 00:00:21,000
// <i>The bread is still warm.</i>
//
// 6
// 00:00:21,500 --> 00:00:24,000
// Two loaves, please.
//
// 7
// 00:00:27,500 --> 00:00:30,000
// Note: the last train leaves at 11:40.
//
// 8
// 00:00:33,500 --> 00:00:36,000
// Fish &amp; chips {chuckles} for two.
//
// 9
// 00:00:36,500 --> 00:00:39,000
// Sit down, please.
//
// 10
// 00:00:39,500 --> 00:00:42,000
// It stopped raining.
```

A second file has typos from a bad download: `Quiet , please !`, `We landed late.It was cold,but dry.` and `-Did you call?`. `fixing.Apply` fixes the spacing, the dashes, the dots and the tags. `Language` sets the language rules. `WrapLines` breaks the lines that are longer than 42 characters, such as the fare in cue 7. See [text.md](text.md) and [editing.md](editing.md#line-wrapping).

<!-- example: ExampleLoad_fixErrors -->
```go
s, _, err := stbox.Load("testdata/fixing/web-errors.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := fixing.Apply(s, fixing.Options{Language: "en"}); err != nil {
	fmt.Println(err)
	return
}
if err := s.WrapLines(42, 2); err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:01,000 --> 00:00:03,200
// Hello <i>there</i>, how was the trip?
//
// 2
// 00:00:03,600 --> 00:00:06,000
// We landed late. It was cold, but dry.
//
// 3
// 00:00:06,400 --> 00:00:09,000
// <i>The station was closed,
// so we walked.</i>
//
// 4
// 00:00:09,400 --> 00:00:11,000
// Take the second exit.
//
// 5
// 00:00:11,400 --> 00:00:13,500
// - Did you call?
// - Not yet.
//
// 6
// 00:00:13,900 --> 00:00:16,000
// I waited... and waited...
//
// 7
// 00:00:16,400 --> 00:00:19,000
// The fare was 1.5 euros,
// e.g. on www.example.com.
//
// 8
// 00:00:19,400 --> 00:00:21,000
// Quiet, please!
//
// 9
// 00:00:21,400 --> 00:00:23,000
// Gate 4 is open now.
//
// 10
// 00:00:23,400 --> 00:00:25,500
// Mr. Berg is here. So am I.
//
// 11
// 00:00:25,900 --> 00:00:28,000
// <font color="#ffff00">Next stop:
// the harbour</font>
//
// 12
// 00:00:28,400 --> 00:00:30,000
// This line has no errors.
```

## Extract a subtitle track from an MKV or MP4 file

A video file can hold several subtitle tracks. `Tracks` lists them. `ReadOptions.Track` picks one by its number. The code picks the English track by its language. `Parse` and `Load` read MKV, WebM, MP4, MOV and MPEG-TS the same way as a subtitle file. The code reads an MKV file with an English SubRip track and a German ASS track. See [containers.md](containers.md).

<!-- example: ExampleTracks_extract -->
```go
f, err := os.Open("testdata/mkv/mkvmerge.mkv")
if err != nil {
	fmt.Println(err)
	return
}
defer f.Close()
info, err := f.Stat()
if err != nil {
	fmt.Println(err)
	return
}

tracks, err := stbox.Tracks(f, info.Size())
if err != nil {
	fmt.Println(err)
	return
}
track := 0
for _, t := range tracks {
	fmt.Println("track", t.Number, t.Language, t.Format)
	if t.Language == "en" {
		track = t.Number
	}
}

s, _, err := stbox.Parse(f, info.Size(), stbox.ReadOptions{Track: track})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// track 2 en srt
// track 3 de ass
// 1
// 00:00:01,200 --> 00:00:03,800
// The ferry to the island leaves at nine.
//
// 2
// 00:00:04,500 --> 00:00:07,250
// <i>Tickets are sold</i>
// at the kiosk by the pier.
//
// 3
// 00:00:08,000 --> 00:00:09,600
// Mind the step.
//
// 4
// 00:00:11,040 --> 00:00:14,000
// The café on deck opens at ten.
//
// 5
// 00:00:14,000 --> 00:00:16,480
// Bring a coat. The wind is cold
// on the open sea.
//
// 6
// 00:00:17,900 --> 00:00:19,500
// Welcome aboard.
```

## Read Blu-ray or DVD image subtitles with OCR

A Blu-ray stores subtitles as PGS images in a `.sup` file. A DVD stores them as VobSub images, which `Load` reads from the `.idx` file next to its `.sub` file. `Load` gives cues that hold an image and no text. `ocr.Run` reads the text of each image with an engine. The `glyph` engine is written in Go and needs no program on the machine. The `tesseract` engine needs the `tesseract` program and reads more languages. See [ocr.md](ocr.md).

<!-- example: ExampleLoad_ocr -->
```go
s, _, err := stbox.Load("testdata/pgs/text_1080p.sup", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
engine, err := glyph.New(glyph.Options{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := ocr.Run(context.Background(), s, engine, ocr.English); err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:01,000 --> 00:00:03,500
// The train to Bergen leaves at 7:45.
//
// 2
// 00:00:04,000 --> 00:00:07,000
// The bakery opens at six.
// Fresh bread is ready by seven.
//
// 3
// 00:00:07,500 --> 00:00:10,000
// Rain is likely after 3 p.m. today.
//
// 4
// 00:00:10,500 --> 00:00:13,000
// Platform 4, please.
// Watch your step.
//
// 5
// 00:00:13,500 --> 00:00:16,000
// <i>The wind turns north tonight.</i>
//
// 6
// 00:00:16,500 --> 00:00:19,000
// We sold 120 rolls before noon!
//
// 7
// 00:00:19,500 --> 00:00:22,500
// Is the 9:10 bus late again?
// Yes, by about five minutes.
//
// 8
// 00:00:23,000 --> 00:00:25,500
// {\an8}<i>Snow</i> is expected on Friday.
//
// 9
// 00:00:26,000 --> 00:00:28,500
// Two loaves of rye, one baguette.
//
// 10
// 00:00:29,000 --> 00:00:31,500
// The next stop is Central Station.
//
// 11
// 00:00:32,000 --> 00:00:35,000
// Temperatures stay near 18 degrees.
// Light clouds in the evening.
//
// 12
// 00:00:35,500 --> 00:00:38,000
// <i>Tickets cost 4.50 each.</i>
```

## Turn a Whisper transcript into readable subtitles

The first segment of this file runs 11.8 s and holds 3 sentences, and it becomes 3 cues. A viewer cannot read that in one cue. `WordTimestamps` keeps the time of each word. `resegment.ByWords` builds new cues from the words. Each cue holds one sentence, or what fits 2 lines of 42 characters. A pause of 0.6 s ends a cue. See [editing.md](editing.md#resegmenting) and [transcripts.md](transcripts.md).

<!-- example: ExampleLoad_resegment -->
```go
s, _, err := stbox.Load("testdata/resegmenting/own_whisper_long_segments.json", stbox.ReadOptions{
	Format:     stbox.Whisper,
	Transcript: transcript.ReadOptions{WordTimestamps: true},
})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := resegment.ByWords(s, resegment.Options{}); err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:00,000 --> 00:00:02,550
// Welcome to the city library tour.
//
// 2
// 00:00:02,550 --> 00:00:08,550
// The reading room is on the left, and
// the study rooms are on the second floor.
//
// 3
// 00:00:08,550 --> 00:00:11,800
// Please keep your voice down in both rooms.
//
// 4
// 00:00:12,400 --> 00:00:18,200
// The cafe on the ground floor opens at
// nine and closes at six every weekday.
//
// 5
// 00:00:20,200 --> 00:00:22,520
// Any questions before we start?
//
// 6
// 00:00:22,520 --> 00:00:24,000
// Then let us begin.
```

## Translate a subtitle with DeepL

`translation.Translate` sends the text of the cues to an engine and puts the translation back. The times stay. Tags such as `<i>` and `<b>` stay in place. The first sentence of this file runs over 2 cues. It goes out as one text. The translation splits again over the same 2 cues, and the code prints them.

The test starts a local server that answers with a saved DeepL response. So it needs no key and no network. With a real key, leave out `BaseURL` and pass `APIKey: os.Getenv("DEEPL_API_KEY")`. See [translation.md](translation.md).

<!-- example: ExampleLoad_translate -->
```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "testdata/translation/deepl_de.json")
}))
defer srv.Close()

s, _, err := stbox.Load("testdata/translation/own_station.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
engine, err := deepl.New(deepl.Options{APIKey: "secret:fx", BaseURL: srv.URL})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := translation.Translate(context.Background(), s, engine, "en", "de", translation.Options{}); err != nil {
	fmt.Println(err)
	return
}
for i, c := range s.All() {
	if i == 2 {
		break
	}
	fmt.Println(c.Start, c.Text())
}
// Output:
// 1s Der Zug nach Basel fährt
// 3.6s um <b>10:15</b> von Gleis 4 ab.
```

## Build dual subtitles for language learners

A learner wants the English line and the German line on screen together. `dual.FromPair` puts each German cue under the English cue that it overlaps most. A joined cue starts at the earlier start and ends at the later end of its cues. Cue 5 below starts at 13.2 s, because the German cue starts then, so it overlaps cue 4. `SecondaryStyle` sets the German lines in italics. `ModeTopBottom` shows the second language at the top of the screen instead. See [editing.md](editing.md#dual-subtitles).

<!-- example: ExampleLoad_dual -->
```go
english, _, err := stbox.Load("testdata/dual/station_en.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
german, _, err := stbox.Load("testdata/dual/station_de.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
both, err := dual.FromPair(english, german, dual.Options{SecondaryStyle: "i"})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, both, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:01,000 --> 00:00:04,000
// The train to Hamburg leaves from platform 4.
// <i>Der Zug nach Hamburg fährt von Gleis 4.</i>
//
// 2
// 00:00:04,400 --> 00:00:07,400
// It is ten minutes late today.
// <i>Er hat heute zehn Minuten Verspätung.</i>
//
// 3
// 00:00:07,900 --> 00:00:11,100
// The bakery next to the station opens at six.
// They sell fresh rolls.
// <i>Die Bäckerei am Bahnhof öffnet um sechs.</i>
// <i>Sie verkauft frische Brötchen.</i>
//
// 4
// 00:00:12,000 --> 00:00:13,500
// <i>Rain is coming.</i>
//
// 5
// 00:00:13,200 --> 00:00:19,000
// Take an umbrella with you.
// <i>Morgen scheint wieder die Sonne.</i>
//
// 6
// 00:00:19,200 --> 00:00:21,000
// <i>Gute Reise!</i>
```

## Compare two versions of a subtitle

A translator sends back a corrected file. `diff.Compare` pairs the cues of both files and lists each cue that was added, removed or changed. `diff.Text` prints the list as text. See [compare.md](compare.md).

<!-- example: ExampleLoad_diff -->
```go
v1, _, err := stbox.Load("testdata/diff/own_original.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
v2, _, err := stbox.Load("testdata/diff/own_edited.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
differences, err := diff.Compare(v1, v2, diff.Options{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Print(diff.Text(differences))
// Output:
// timing changed: old cue 2, new cue 2
// - 00:00:04.000 --> 00:00:06.200
//   This train calls at all stations
//   to the harbour.
// + 00:00:04.250 --> 00:00:06.450
//   This train calls at all stations
//   to the harbour.
//
// text changed: old cue 4, new cue 4
// - 00:00:10.500 --> 00:00:13.000
//   The next stop is Milbrook.
// + 00:00:10.500 --> 00:00:13.000
//   The next stop is Millbrook.
//
// text and timing changed: old cue 6, new cue 6
// - 00:00:17.000 --> 00:00:21.000
//   Tickets are on sale in coach three. Cards and coins are fine.
// + 00:00:17.000 --> 00:00:19.000
//   Tickets are on sale in coach three.
//
// added: new cue 7
// + 00:00:19.000 --> 00:00:21.000
//   Cards and coins are fine.
//
// removed: old cue 8
// - 00:00:25.000 --> 00:00:26.500
//   Thank you.
//
// text and timing changed: old cue 9, new cue 9
// - 00:00:28.000 --> 00:00:30.500
//   Rain is likley later today.
// + 00:00:28.000 --> 00:00:31.000
//   Rain is likely later today.
//
// text changed: old cue 10, new cue 10
// - 00:00:31.000 --> 00:00:33.000
//   <i>Mind the gap.</i>
// + 00:00:31.001 --> 00:00:33.000
//   Mind the gap.
//
// text changed: old cue 11, new cue 11
// - 00:00:34.000 --> 00:00:36.500
//   Bicycles go in the last coach,
//   next to the luggage rack.
// + 00:00:34.000 --> 00:00:36.500
//   Bicycles go in the last coach, next
//   to the luggage rack.
//
// added: new cue 13
// + 00:00:39.000 --> 00:00:40.000
//   The doors open on the left.
```

## Walk a folder and report statistics

A folder holds subtitle files in mixed formats. `Load` detects the format of each file. `stats.Of` counts the cues, the words and the characters per line. The code skips a file that `Load` cannot read and prints the reason. See [statistics.md](statistics.md) and [detection.md](detection.md).

<!-- example: ExampleDetect_folder -->
```go
err := filepath.WalkDir("testdata/dual", func(path string, d fs.DirEntry, err error) error {
	if err != nil || d.IsDir() {
		return err
	}
	s, info, err := stbox.Load(path, stbox.ReadOptions{})
	if err != nil {
		fmt.Println(d.Name(), "skipped:", err)
		return nil
	}
	st := stats.Of(s)
	fmt.Printf("%-30s %-4s %d cues, %d words, %.1f characters per line\n",
		d.Name(), info.Format, st.CueCount, st.WordCount, st.CharactersPerLine.Average)
	return nil
})
if err != nil {
	fmt.Println(err)
}
// Output:
// SOURCES.md skipped: stbox: unsupported format: no format detected: Format detection found no subtitle format.
// station_de.srt                 srt  6 cues, 32 words, 31.5 characters per line
// station_en.srt                 srt  5 cues, 35 words, 30.0 characters per line
// station_stack.ass              ass  6 cues, 67 words, 30.8 characters per line
// station_stack.srt              srt  6 cues, 67 words, 30.8 characters per line
// station_stack.vtt              vtt  6 cues, 67 words, 30.8 characters per line
// station_top_bottom.ass         ass  11 cues, 67 words, 30.8 characters per line
// station_top_bottom.srt         srt  11 cues, 67 words, 30.8 characters per line
// station_top_bottom.vtt         vtt  11 cues, 67 words, 30.8 characters per line
// station_top_bottom_aligned.srt srt  11 cues, 67 words, 30.8 characters per line
```

## Create a subtitle from scratch

A program that makes captions, such as a tour script, builds the cues itself. `AddCues` keeps the cues in start order. A string in `Lines` is one line of the cue. See [subtitle.md](subtitle.md).

<!-- example: ExampleSubtitle_fromScratch -->
```go
s := &stbox.Subtitle{}
s.AddCues(
	stbox.Cue{Start: 2 * time.Second, End: 5 * time.Second, Lines: []string{"Welcome to the harbour tour."}},
	stbox.Cue{Start: 5500 * time.Millisecond, End: 9 * time.Second, Lines: []string{"Our first stop is the", "old lighthouse."}},
)
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:02,000 --> 00:00:05,000
// Welcome to the harbour tour.
//
// 2
// 00:00:05,500 --> 00:00:09,000
// Our first stop is the
// old lighthouse.
```
