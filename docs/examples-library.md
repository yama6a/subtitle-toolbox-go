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

A German file is late, and it also drifts against the 23.976 fps video. An English file for the same video is in sync. `refsync.Apply` compares the cue times of both files and ignores the text. It finds the scale and the offset and retimes the German file. The score is the share of time that both files cover, from 0 to 1.

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

The code parses a string. Use `stbox.Load` for a file.

<!-- example: validation/ExampleNetflixEnglish -->
```go
const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
	"2\n00:00:02,500 --> 00:00:02,900\nANNA: We need two tickets for the long ride to the coast.\n"
r := strings.NewReader(content)
s, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
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
// cue 2 maxCharactersPerSecond 142.5
// cue 2 minDuration 0.4
// cue 2 noOverlap 0.5
```

## Clean up a subtitle

A subtitle from the web has hearing-impaired annotations, OCR typos and long lines. Three calls fix them in order. `hearingimpaired.Apply` removes `[BELL RINGS]`, `ANNA:` and `(sighs)`, and a cue with no text left goes. `fixing.Apply` fixes common errors such as a spaced question mark and `l` for `I`. `WrapLines` breaks each line at 24 characters. See [text.md](text.md) and [editing.md](editing.md#line-wrapping).

<!-- example: ExampleLoad_cleanUp -->
```go
s, _, err := stbox.Load("testdata/cli/trip.srt", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := hearingimpaired.Apply(s, hearingimpaired.Options{}); err != nil {
	fmt.Println(err)
	return
}
if _, err := fixing.Apply(s, fixing.Options{Language: "en"}); err != nil {
	fmt.Println(err)
	return
}
if err := s.WrapLines(24, 2); err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// 1
// 00:00:01,000 --> 00:00:03,000
// <i>The train</i>
// <i>leaves at noon.</i>
//
// 2
// 00:00:02,500 --> 00:00:05,000
// We need two tickets for
// the long ride to the coast.
//
// 3
// 00:00:06,000 --> 00:00:06,400
// Too late.
```

## Extract a subtitle track from an MKV or MP4 file

A video file can hold several subtitle tracks. `Tracks` lists them. `ReadOptions.Track` picks one by its number. `Parse` and `Load` read MKV, WebM, MP4, MOV and MPEG-TS the same way as a subtitle file. The code reads an MKV file with an English SubRip track and a German ASS track. See [containers.md](containers.md).

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
for _, t := range tracks {
	fmt.Println("track", t.Number, t.Format)
}

s, _, err := stbox.Parse(f, info.Size(), stbox.ReadOptions{Track: 2})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// track 2 srt
// track 3 ass
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

Whisper writes one long segment per breath, often with 3 sentences. A viewer cannot read that. `WordTimestamps` keeps the time of each word. `resegment.ByWords` builds new cues from the words. Each cue holds one sentence, or what fits 2 lines of 42 characters, and a pause of 0.6 s ends a cue. The first segment of this file runs 11.8 s and becomes 3 cues. See [editing.md](editing.md#resegmenting) and [transcripts.md](transcripts.md).

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

`translation.Translate` sends the text of the cues to an engine and puts the translation back. The times stay. Tags such as `<i>` and `<b>` stay in place. The first sentence of this file runs over 2 cues. It goes out as one text, and the translation splits again over the same 2 cues. The code prints these 2 cues.

The test starts a local server that answers with a saved DeepL response, so it needs no key and no network. With a real key, leave out `BaseURL` and pass `APIKey: os.Getenv("DEEPL_API_KEY")`. See [translation.md](translation.md).

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
for _, c := range s.Cues()[:2] {
	fmt.Println(c.Start, c.Text())
}
// Output:
// 1s Der Zug nach Basel fährt
// 3.6s um <b>10:15</b> von Gleis 4 ab.
```

## Build dual subtitles for language learners

A learner wants the English line and the German line on screen together. `dual.FromPair` puts each German cue under the English cue that it overlaps most. The English times win. Here the 2 German cues at 7.9 s and 9.6 s sit under one English cue, and `SecondaryStyle` sets the German lines in italics. `ModeTopBottom` shows the second language at the top of the screen instead. See [editing.md](editing.md#dual-subtitles).

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

A translator sends back a corrected file. `diff.Compare` pairs the cues of both files and lists each cue that was added, removed or changed. `diff.Text` prints the result as text. See [compare.md](compare.md).

<!-- example: diff/ExampleText -->
```go
var v1, v2 model.Subtitle
v1.AddCues(
	model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
	model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Milbrook."}},
	model.Cue{Start: 8 * time.Second, End: 9 * time.Second, Lines: []string{"Thank you."}},
)
v2.AddCues(
	model.Cue{Start: 0, End: 500 * time.Millisecond, Lines: []string{"Welcome."}},
	model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
	model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Millbrook."}},
)

differences, err := diff.Compare(&v1, &v2, diff.Options{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Print(diff.Text(differences))
// Output:
// added: new cue 1
// + 00:00:00.000 --> 00:00:00.500
//   Welcome.
//
// text changed: old cue 2, new cue 3
// - 00:00:04.000 --> 00:00:06.000
//   The next stop is Milbrook.
// + 00:00:04.000 --> 00:00:06.000
//   The next stop is Millbrook.
//
// removed: old cue 3
// - 00:00:08.000 --> 00:00:09.000
//   Thank you.
```

## Walk a folder and report statistics

A folder holds subtitle files in mixed formats. `Load` detects the format of each file. `stats.Of` counts the cues, the words and the characters per line. The code skips a file that `Load` cannot read and prints the reason. See [statistics.md](statistics.md) and [detection.md](detection.md).

<!-- example: ExampleDetect_folder -->
```go
err := filepath.WalkDir("testdata/dual", func(path string, d fs.DirEntry, err error) error {
	if err != nil || d.IsDir() || filepath.Ext(path) == ".md" {
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

A program that makes captions, such as a tour script or a quiz, builds the cues itself. `AddCues` keeps the cues in start order. A string in `Lines` is one line of the cue. See [subtitle.md](subtitle.md).

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
