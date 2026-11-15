package stbox_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/dual"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr/glyph"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/refsync"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/resegment"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/translation"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/translation/deepl"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v5/inspect/diff"
	"github.com/yama6a/subtitle-toolbox-go/v5/inspect/stats"
	"github.com/yama6a/subtitle-toolbox-go/v5/inspect/validation"
)

func ExampleLoad() {
	s, info, err := stbox.Load("testdata/srt/real/own_missing_empty_line.srt", stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.Format, info.SourceEncoding, s.Len(), "cues")
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// srt UTF-8 4 cues
	// 1s 2.5s The bus to the airport is full.
	// 3s 5s The next one leaves at four.
	// 5.5s 7s Tickets are on sale inside.
	// 8s 10s Mind the step.
}

func ExampleParse() {
	content := "1\n00:00:01,500 --> 00:00:04,000\n<i>Fish & Chips</i>\n\n" +
		"2\n00:00:05,000 --> 00:00:06,000\nAT&amp T\n\n" +
		"3\n00:00:07,000 --> 00:00:0"
	r := strings.NewReader(content)
	s, info, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{Lenient: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	for _, w := range info.Warnings {
		fmt.Println(w.Action, "line", w.Line, w.Message)
	}
	// Output:
	// 1.5s 4s [<i>Fish &amp; Chips</i>]
	// 5s 6s [AT&amp;amp T]
	// skipped line 10 The time "00:00:0" is not valid.
}

func ExampleWrite() {
	s := &stbox.Subtitle{}
	if err := s.AddCues(
		stbox.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>"}},
		stbox.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"<c.yellow>Now</c> open"}, Position: stbox.Position{Alignment: stbox.TopCenter}},
	); err != nil {
		panic(err)
	}
	_, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// 1
	// 00:00:01,500 --> 00:00:04,000
	// <i>Fish & Chips</i>
	//
	// 2
	// 00:00:05,000 --> 00:00:06,000
	// {\an8}<font color="#ffff00">Now</font> open
}

func ExampleDetect() {
	for _, content := range []string{
		"\ufeffWEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nRain\r\n",
		"1\n00:00:01,000 --> 00:00:04,000\nHello\n",
		`{"version": 1, "cues": []}`,
		"Hello\n",
	} {
		d, ok := stbox.Detect([]byte(content))
		f := d.Format
		fmt.Printf("%q %t\n", f, ok)
	}
	// Output:
	// "vtt" true
	// "srt" true
	// "json" true
	// "" false
}

func ExampleNewData() {
	data := stbox.NewData(
		stbox.Field{Key: "language", Value: "en"},
		stbox.Field{Key: "frames", Value: 25},
	)
	language, _ := data.String("language")
	fmt.Println(language, data.Len())
	// Output: en 2
}

func ExampleTracks() {
	b, err := os.ReadFile("testdata/mkv/mkvmerge.mkv")
	if err != nil {
		fmt.Println(err)
		return
	}
	tracks, err := stbox.Tracks(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, t := range tracks {
		fmt.Println(t.Container, t.Number, t.CodecID, t.Format)
	}
	// Output:
	// matroska 2 S_TEXT/UTF8 srt
	// matroska 3 S_TEXT/ASS ass
}

func ExampleLoad_toWebVTT() {
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
}

func ExampleLoad_shift() {
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
}

func ExampleLoad_frameRate() {
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
}

func ExampleLoad_syncToSpeech() {
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
}

func ExampleLoad_removeHearingImpaired() {
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
}

func ExampleLoad_fixErrors() {
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
}

func ExampleTracks_extract() {
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
}

func ExampleLoad_ocr() {
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
	if _, err := ocr.Apply(context.Background(), s, engine, ocr.English); err != nil {
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
}

func ExampleLoad_resegment() {
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
}

func ExampleLoad_translate() {
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
	if _, err := translation.Apply(context.Background(), s, engine, "en", "de", translation.Options{}); err != nil {
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
}

func ExampleLoad_dual() {
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
}

func ExampleDetect_folder() {
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
	// SOURCES.md skipped: stbox: unsupported format: no format detected: format detection found no subtitle format
	// station_de.srt                 srt  6 cues, 32 words, 31.5 characters per line
	// station_en.srt                 srt  5 cues, 35 words, 30.0 characters per line
	// station_stack.ass              ass  6 cues, 67 words, 30.8 characters per line
	// station_stack.srt              srt  6 cues, 67 words, 30.8 characters per line
	// station_stack.vtt              vtt  6 cues, 67 words, 30.8 characters per line
	// station_top_bottom.ass         ass  11 cues, 67 words, 30.8 characters per line
	// station_top_bottom.srt         srt  11 cues, 67 words, 30.8 characters per line
	// station_top_bottom.vtt         vtt  11 cues, 67 words, 30.8 characters per line
	// station_top_bottom_aligned.srt srt  11 cues, 67 words, 30.8 characters per line
}

func ExampleSubtitle_fromScratch() {
	s := &stbox.Subtitle{}
	if err := s.AddCues(
		stbox.Cue{Start: 2 * time.Second, End: 5 * time.Second, Lines: []string{"Welcome to the harbour tour."}},
		stbox.Cue{Start: 5500 * time.Millisecond, End: 9 * time.Second, Lines: []string{"Our first stop is the", "old lighthouse."}},
	); err != nil {
		panic(err)
	}
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
}

func ExampleLoad_syncToReference() {
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
}

func ExampleLoad_validate() {
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
}

func ExampleLoad_diff() {
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
}

func ExampleParse_encoding() {
	latin1 := []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9 au lait\n")

	r := bytes.NewReader(latin1)
	s, info, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.SourceEncoding, s.Len()) // Windows-1252 1

	_, info, err = stbox.Parse(r, r.Size(), stbox.ReadOptions{Encoding: "cp1250"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.SourceEncoding) // cp1250, as the caller wrote it
	// Output:
	// Windows-1252 1
	// cp1250
}

func ExampleParse_error() {
	content := "1\n00:00:01,000 --> 00:00:04,000\nHello\n\n2\n00:00:05,000 => 00:00:07,000\nWorld\n"

	r := strings.NewReader(content)
	_, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{Format: stbox.SubRip})
	var perr *stbox.ParseError
	if errors.As(err, &perr) {
		fmt.Println(perr.Format, perr.Line)
	}
	fmt.Println(errors.Is(err, stbox.ErrParse))
	fmt.Println(err)
	// Output:
	// srt 6
	// true
	// srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".
}

func ExampleWrite_lineEnding() {
	srt := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"

	r := strings.NewReader(srt)
	s, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	var out bytes.Buffer
	_, err = stbox.Write(&out, s, stbox.WebVTT, stbox.WriteOptions{LineEnding: stbox.CRLF})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q\n", out.String())
	// Output:
	// "\ufeffWEBVTT\r\n\r\n1\r\n00:00:01.000 --> 00:00:02.000\r\nHello\r\n"
}

func ExampleWord() {
	c := stbox.Cue{
		End:   5 * time.Second,
		Lines: []string{"The beach"},
		Words: []stbox.Word{
			{Text: "The", Start: 0, End: 240 * time.Millisecond},
			{Text: "beach", Start: 240 * time.Millisecond, End: time.Second},
		},
	}
	fmt.Println(c.TaggedLines())
	// Output: [<00:00:00.000>The <00:00:00.240>beach]
}

func ExampleEncodings() {
	names := stbox.Encodings()
	fmt.Println(names[:4])
	fmt.Println(slices.Contains(names, "Windows-1252"), slices.Contains(names, "CP1125"))
	// Output:
	// [UTF-8 UTF-16LE UTF-16BE UTF-16]
	// true false
}

func ExampleReadOptions_Validate() {
	err := stbox.ReadOptions{LastCueDuration: new(-time.Second)}.Validate()
	fmt.Println(errors.Is(err, stbox.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: ReadOptions.LastCueDuration: the last cue duration must be 0 or more, got -1s
}

func ExampleFormatFromPath() {
	for _, path := range []string{"movie.SUB", "notes.txt", "clip.xml", "movie"} {
		f, ok := stbox.FormatFromPath(path)
		fmt.Printf("%s: %q %t\n", path, f, ok)
	}
	// Output:
	// movie.SUB: "microdvd" true
	// notes.txt: "txt" true
	// clip.xml: "ttml" true
	// movie: "" false
}

func ExampleFormats() {
	for _, f := range stbox.Formats() {
		fmt.Println(f, f.Extensions())
	}
	// Output:
	// ass [ass ssa]
	// cavena890 [890]
	// csv [csv]
	// ffmeta-chapters [ffmeta]
	// html [html htm]
	// itt [itt]
	// json [json]
	// assemblyai [json]
	// aws-transcribe [json]
	// deepgram [json]
	// google-speech [json]
	// lrc [lrc]
	// mcc [mcc]
	// microdvd [sub]
	// mpsub [mpsub]
	// pac [pac fpc]
	// pgs [sup]
	// sami [smi sami]
	// sbv [sbv]
	// scc [scc]
	// srt [srt]
	// stl [stl]
	// spruce-stl [stl]
	// subviewer [sub]
	// ttml [ttml dfxp xml]
	// dcp [xml]
	// tsv [tsv]
	// txt [txt]
	// mpl2 [txt]
	// tmplayer [txt]
	// qttext [txt]
	// vobsub [idx]
	// vtt [vtt]
	// whisper [json]
	// youtube [srv3 json3 srv1 ytt]
	// ogm-chapters [txt]
	// podcast-chapters [json]
	// podcast-transcript [json]
	// youtube-chapters [txt]
}

func ExampleNewImage() {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	im, err := stbox.NewImage(img, 100, 900, 1920, 1080)
	fmt.Println(im.X, im.Y, im.Width, im.Height, im.ScreenWidth, im.ScreenHeight, err)
	// Output: 100 900 4 2 1920 1080 <nil>
}

func ExampleEncodePNG() {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	data, err := stbox.EncodePNG(img)
	fmt.Println(string(data[1:4]), err)
	// Output: PNG <nil>
}

func ExampleDecodePNG() {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 128, B: 1, A: 64})
	data, _ := stbox.EncodePNG(img)
	got, err := stbox.DecodePNG(data)
	fmt.Println(got.NRGBAAt(0, 0), err)
	// Output: {255 128 1 64} <nil>
}

func ExampleDecodeImage() {
	im, _ := stbox.NewImage(image.NewNRGBA(image.Rect(0, 0, 3, 2)), 0, 0, 720, 576)
	got, err := stbox.DecodeImage(im)
	fmt.Println(got.Bounds().Dx(), got.Bounds().Dy(), err)
	// Output: 3 2 <nil>
}
