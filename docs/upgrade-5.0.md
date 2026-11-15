# Upgrade to 5.0

Version 5.0 moves the module path to `/v5` and groups the packages by purpose. The services live under `edit/` and `inspect/`, and the stream readers and writers live in the format packages. It also renames a few entry points and writes error messages in Go style. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v5@latest
```

The CLI installs from the new path.

```bash
go install github.com/yama6a/subtitle-toolbox-go/v5/cmd/stbox@latest
```

## Import paths

| 4.x | 5.0 |
|:--- |:--- |
| `github.com/yama6a/subtitle-toolbox-go/v4` (every package) | `github.com/yama6a/subtitle-toolbox-go/v5` |
| `github.com/yama6a/subtitle-toolbox-go/v4/markup` | `github.com/yama6a/subtitle-toolbox-go/v5/model/markup` |
| `github.com/yama6a/subtitle-toolbox-go/v4/hls` | `github.com/yama6a/subtitle-toolbox-go/v5/format/hls` |
| `github.com/yama6a/subtitle-toolbox-go/v4/stream` | `github.com/yama6a/subtitle-toolbox-go/v5/format/srt` and `github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt` |
| `github.com/yama6a/subtitle-toolbox-go/v4/imaging` | `github.com/yama6a/subtitle-toolbox-go/v5` |
| `github.com/yama6a/subtitle-toolbox-go/v4/dual` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/dual` |
| `github.com/yama6a/subtitle-toolbox-go/v4/fixing` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/fixing` |
| `github.com/yama6a/subtitle-toolbox-go/v4/hearingimpaired` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/hearingimpaired` |
| `github.com/yama6a/subtitle-toolbox-go/v4/karaoke` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/karaoke` |
| `github.com/yama6a/subtitle-toolbox-go/v4/profanity` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/profanity` |
| `github.com/yama6a/subtitle-toolbox-go/v4/refsync` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/refsync` |
| `github.com/yama6a/subtitle-toolbox-go/v4/resegment` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/resegment` |
| `github.com/yama6a/subtitle-toolbox-go/v4/speakers` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/speakers` |
| `github.com/yama6a/subtitle-toolbox-go/v4/timing` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/timing` |
| `github.com/yama6a/subtitle-toolbox-go/v4/ocr` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr` |
| `github.com/yama6a/subtitle-toolbox-go/v4/ocr/glyph` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr/glyph` |
| `github.com/yama6a/subtitle-toolbox-go/v4/ocr/ocrtest` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr/ocrtest` |
| `github.com/yama6a/subtitle-toolbox-go/v4/ocr/tesseract` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr/tesseract` |
| `github.com/yama6a/subtitle-toolbox-go/v4/translation` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/translation` |
| `github.com/yama6a/subtitle-toolbox-go/v4/translation/deepl` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/translation/deepl` |
| `github.com/yama6a/subtitle-toolbox-go/v4/translation/google` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/translation/google` |
| `github.com/yama6a/subtitle-toolbox-go/v4/translation/openai` | `github.com/yama6a/subtitle-toolbox-go/v5/edit/translation/openai` |
| `github.com/yama6a/subtitle-toolbox-go/v4/diff` | `github.com/yama6a/subtitle-toolbox-go/v5/inspect/diff` |
| `github.com/yama6a/subtitle-toolbox-go/v4/stats` | `github.com/yama6a/subtitle-toolbox-go/v5/inspect/stats` |
| `github.com/yama6a/subtitle-toolbox-go/v4/validation` | `github.com/yama6a/subtitle-toolbox-go/v5/inspect/validation` |

The packages of the other formats, `model`, and the containers keep their path below `/v5`. The internal packages moved too, and code outside the module cannot import them.

## Renamed and removed identifiers

| 4.x | 5.0 |
|:--- |:--- |
| `stream.ReadOptions` | `srt.StreamOptions` and `webvtt.StreamOptions` |
| `stream.SubRipReader`, `stream.NewSubRipReader` | `srt.StreamReader`, `srt.NewStreamReader` |
| `stream.SubRipWriter`, `stream.NewSubRipWriter` | `srt.StreamWriter`, `srt.NewStreamWriter` |
| `stream.WebVTTReader`, `stream.NewWebVTTReader` | `webvtt.StreamReader`, `webvtt.NewStreamReader` |
| `stream.WebVTTWriter`, `stream.NewWebVTTWriter` | `webvtt.StreamWriter`, `webvtt.NewStreamWriter` |
| `imaging.Decode` | `stbox.DecodeImage` |
| `imaging.DecodePNG`, `imaging.EncodePNG`, `imaging.NewImage` | `stbox.DecodePNG`, `stbox.EncodePNG`, `stbox.NewImage` |
| `ocr.Run` | `ocr.Apply`, same arguments and results |
| `translation.Translate` | `translation.Apply`, same arguments and results. `Engine.Translate` keeps its name |
| `translation.HTTPClient` | removed. Pass an `*http.Client`. For a fake, set its `Transport` to an `http.RoundTripper` |
| `deepl.Options.HTTPClient`, `google.Options.HTTPClient`, `openai.Options.HTTPClient` | type `*http.Client`. nil still means `translation.DefaultHTTPClient()` |
| `profanity.Options.MaskFunc` | removed. Use `Mask`. For another mask, run `Apply` with `MaskNone` and rewrite the matched words yourself |
| `ocr.EngineName`, `ocr.EngineTesseract`, `ocr.EngineGlyph` | removed. Use your own names |
| package `ocr/auto`: `auto.Options`, `auto.Choose`, `auto.New` | removed. Call `tesseract.Installed(ctx, "")`, then `tesseract.New` or `glyph.New` |
| `glyph.Engine.Database` | removed. Keep the database that you pass in `glyph.Options.Database` |
| `stbox.ErrUnknownFormat`, `model.ErrUnknownFormat` | `stbox.ErrNoFormatDetected`, `model.ErrNoFormatDetected` |
| `Format.IsAutoDetected()` | removed. Call `Detect`. It returns only formats with a signature or a JSON check |
| `transcript.DetectPodcast`, `transcript.DetectWhisper` | removed. Call `stbox.Detect`, or `DetectPodcastJSON` and `DetectWhisperJSON` with the top-level keys |
| `mkv.File.Extract(n, common)`, `mpegts.File.Extract(n, common)` | `Extract(n, common, mkv.ExtractOptions{})`, `Extract(n, common, mpegts.ExtractOptions{})` |
| `s.AddCue(c)` | `if err := s.AddCue(c); err != nil { ... }` |
| `s.AddCues(cs...)` | `if err := s.AddCues(cs...); err != nil { ... }` |
| `err := chapters.WriteOGM(...)`, and the other 3 chapter writers | `_, err := chapters.WriteOGM(...)`. The writer returns `(chapters.WriteReport, error)` |
| `err := transcript.WriteWhisper(...)`, and the other 6 transcript writers | `_, err := transcript.WriteWhisper(...)`. The writer returns `(transcript.WriteReport, error)` |
| `err := youtube.Write(...)` | `_, err := youtube.Write(...)`. The writer returns `(youtube.WriteReport, error)` |

`vobsub.WriteOptions`, `vobsub.WriteReport`, `plaintext.ReadOptions`, `model.FrameRate.ValidateFor` and `model.FrameRateFromFloatFor` are new. Callers need no change.

## Changed behaviour

| 4.x | 5.0 |
|:--- |:--- |
| An error message started in upper case and could end in a period, as in `stbox: cue not found: Cannot split cue 3: the cue does not exist.` | A message starts in lower case and ends without a period, as in `stbox: cue not found: cannot split cue 3: the cue does not exist`. Code that compares message text must change. `errors.Is` and `errors.As` need no change |
| `Subtitle.AddCue` stored a cue with an invalid `Position`, `Image` or `Words` | The call returns an error that matches `ErrInvalidArgument`, and the subtitle stays unchanged |
| `transcript.ReadWhisper` with word timestamps built a cue that `SetCue` rejected, when a word held a no-break space or an em space | The word text gets the same space rule as the cue text, so the cue passes `SetCue` |
