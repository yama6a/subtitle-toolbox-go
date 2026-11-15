package stbox_test

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/dual"
	"github.com/yama6a/subtitle-toolbox-go/v4/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/glyph"
	"github.com/yama6a/subtitle-toolbox-go/v4/refsync"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
	"github.com/yama6a/subtitle-toolbox-go/v4/stats"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation/deepl"
)

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

func ExampleLoad_cleanUp() {
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
}

func ExampleSubtitle_fromScratch() {
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
}
