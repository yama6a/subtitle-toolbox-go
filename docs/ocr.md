# OCR

OCR (optical character recognition) fills the lines of image cues, so a Blu-ray or DVD subtitle can become SubRip or WebVTT. Example: a subtitle has an image cue at 1 s without lines. `ocr.Run(ctx, s, engine, "")` calls the engine for that cue and sets its lines to `Next stop: Bergen`. The image stays.

<!-- example: ocr/ExampleRun -->
```go
s := &model.Subtitle{}
s.AddCue(model.Cue{
	Start: time.Second,
	End:   3 * time.Second,
	Image: &model.Image{PNG: []byte("PNG"), X: 640, Y: 940, Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080},
})
engine := &ocrtest.Engine{Text: ocr.Text{Lines: []string{"Next stop: Bergen"}}}

report, err := ocr.Run(context.Background(), s, engine, ocr.English)
if err != nil {
	fmt.Println(err)
	return
}

cue, _ := s.Cue(0)
fmt.Println(cue.Lines, cue.Image != nil, len(report.Texts))
fmt.Println(engine.Calls[0].Language)
// Output:
// [Next stop: Bergen] true 1
// eng
```

## The engine

An `ocr.Engine` has one method. `Recognize(ctx, im, language)` reads the PNG of one cue image and returns an `ocr.Text`. A caller can plug in any OCR service by implementing it.

- `Text.Lines` holds plain text or core markup, such as `<i>Hello</i>`.
- `Text.Confidence` is a number from 0 to 1, or nil when the engine gives none. `Text.Validate` returns an error that matches `ErrInvalidArgument` for any other value, NaN included.
- `language` is a Tesseract model name such as `deu` or `deu+eng`. The constants of package `ocr`, such as `ocr.German`, hold the 110 standard names. Any other string works, for example a custom model. `""` lets the engine use its own default. An engine that has no languages ignores it.

An engine that fails on an image returns an error. `Run` wraps it.

## Run

`Run` visits the cues in order. It calls the engine for each cue that has an image and no lines, one cue at a time. It skips text cues and image cues that already have lines. A subtitle without such a cue never calls the engine.

- **Lines**: each cue gets the lines of the text, cleaned as `Cue.AddLine` cleans them. Empty lines are dropped. The image stays.
- **Language**: `Run` passes it to the engine unchanged.
- **Report**: `Report.Texts` lists `CueText` values in cue order. Each holds the cue index and the text as the engine returned it, with empty lines and the confidence.
- **Cancel**: `Run` checks `ctx.Err()` before each engine call and returns it wrapped.
- **Errors**: an error of the engine, or a `Text` that fails `Validate`, returns a `*CueError` with the cue index. It matches `ErrRecognize` and wraps the cause. Cues read before the error keep their new lines, and the returned report lists them.

## An engine for an OCR web service

This engine posts the PNG of each cue to a service and reads a JSON answer such as `{"lines": ["Next stop: Bergen"]}`. The source is `ocr/engine_example_test.go`, where `ExampleEngine` runs it against a test server.

```text
type webEngine struct {
	url string
}

func (e webEngine) Recognize(ctx context.Context, im model.Image, language ocr.Language) (ocr.Text, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url+"?lang="+url.QueryEscape(string(language)), bytes.NewReader(im.PNG))
	if err != nil {
		return ocr.Text{}, err
	}
	req.Header.Set("Content-Type", "image/png")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ocr.Text{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ocr.Text{}, fmt.Errorf("the service answered %s", resp.Status)
	}
	var answer struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return ocr.Text{}, err
	}
	return ocr.Text{Lines: answer.Lines}, nil
}
```

## PGS files

`stbox.Load` reads a Blu-ray `.sup` file as image cues without lines, see [formats.md](formats.md#pgs). `Run` then fills their lines. Each display set that shows objects is one cue, with its bitmap as a PNG in `Cue.Image`. A display set that shows 2 objects gives one cue with one image, so the engine reads both lines together. The forced flag of the objects is `Cue.Forced`, so a caller can pick the forced cues of a track before it runs OCR.

## PGS colour reduction

`stbox.Write` with `stbox.PGS` writes image cues as a `.sup` file, see [formats.md](formats.md#pgs). All images that show at the same time share one palette of at most 256 colours. The writer reduces the colours only when the shown images together hold more than 256 colours. Then a median cut gives 255 opaque colours and entry 0 as transparent. One image with 256 colours stays as it is. Two images with 200 different colours each are reduced.

## Tesseract

Package `ocr/tesseract` reads cue images with the `tesseract` program. It runs one process per image through `os/exec`, so the library needs no cgo. Example: for a 1920x1080 cue the engine draws the image as dark text on white with a 10-pixel border, writes it as a PGM file and runs `tesseract <file> stdout -l deu --psm 6 -c tessedit_create_tsv=1`. It builds the lines from the TSV word rows.

```text
engine, err := tesseract.New(tesseract.Options{Language: ocr.German})
```

| Field | Default | Meaning |
|:--- |:--- |:--- |
| `Language` | `eng` | Model for cues where `Recognize` gets no language, for example `deu` or `deu+eng` |
| `PageSegmentationMode` | nil: 6, one block of text | The `--psm` value from 0 to 13. A pointer, because 0 is a legal mode |
| `Program` | `tesseract` | Path or name of the program, looked up in `PATH` |
| `Scale` | 0: 2 on screens below 720 lines, else 1 | Factor from 1 to 8 that scales the image up before OCR |
| `NoInvert` | false | Keep the grey levels. By default light text becomes dark on white |
| `Threshold` | 0: keep grey levels | From 1 to 255. Grey levels below it become black, the others white |

- **Checks**: `Recognize` and `RequireLanguages` first check that the program runs and that the data of every part of the language is installed. A failure is an `*InvalidArgumentError` with an install hint, for example `apt install tesseract-ocr` for Debian and Ubuntu, `apk add tesseract-ocr tesseract-ocr-data-eng` for Alpine, `dnf install tesseract` for Fedora and `brew install tesseract` for macOS. `Installed(ctx, program)` reports whether the program runs.
- **Text**: the words of one TSV line become one line, escaped as core markup. Tesseract reports no italic, so the lines hold no `<i>` tags. The confidence is the mean word confidence from 0 to 1.
- **Errors**: a program that exits with a code other than 0, or that cannot start, gives an error that matches `ErrRecognize`.
- **Tests**: tests with a fake program run everywhere. `TestTesseractAccuracy` needs a real Tesseract with `eng` and `rus` data. It skips without one, unless `STBOX_REQUIRE_TESSERACT=1` is set. The CI job `tesseract` sets it.

## Glyph engine

Package `ocr/glyph` reads cue images with [go-glyph-ocr](https://github.com/yama6a/go-glyph-ocr), a pure Go engine. It needs no Tesseract and no cgo, so it works in the distroless image. Example: `glyph.New(glyph.Options{})` reads the first cue of `text_1080p.sup` as `The train to Bergen leaves at 7:45.`.

<!-- example: ocr/glyph/ExampleNew -->
```go
s, _, err := stbox.Load("../../testdata/pgs/text_1080p.sup", stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
engine, err := glyph.New(glyph.Options{})
if err != nil {
	fmt.Println(err)
	return
}
if _, err := ocr.Run(context.Background(), s, engine, ""); err != nil {
	fmt.Println(err)
	return
}

cue, _ := s.Cue(0)
fmt.Println(cue.Lines)
// Output:
// [The train to Bergen leaves at 7:45.]
```

| Field | Default | Meaning |
|:--- |:--- |:--- |
| `Database` | nil: `glyphocr.SubtitleFonts()` | The glyphs to match. `glyphocr.Latin()` or a `.nocr` file from `glyphocr.LoadDatabase` also work |
| `Recognizer` | none | The options of go-glyph-ocr, passed to `glyphocr.NewRecognizer` unchanged, for example `glyphocr.ItalicSlant(0.2)` or `glyphocr.NoLineContext()` |

- **Databases**: the subtitle fonts database holds 2,007 glyphs of DejaVu Sans, Liberation Sans and Noto Sans, then the Latin glyphs. It reads 98.3% of the characters of the test sets right. The Latin database alone reads 85.0%. The [go-glyph-ocr README](https://github.com/yama6a/go-glyph-ocr#readme) lists every option.
- **One stream per engine**: the recognizer learns the glyph heights of the images it reads. Use one engine for each subtitle stream. An engine is not safe for concurrent use. Engines can share one `*glyphocr.Database`, because matching only reads it. `Engine.Database` returns it.
- **Text**: the engine splits each line into words at the space characters of go-glyph-ocr. A word is italic when more than half of its characters are. Italic words in a row become one run: `<i>The wind turns north tonight.</i>`. A run ends before the first upright word: `<i>Snow</i> is expected on Friday.` Markup characters are escaped. The confidence is the mean of all characters, unknown characters counting 0. An image without ink gives no lines and a confidence of 0.
- **Language**: ignored.
- **Errors**: `New` returns a `*model.InvalidArgumentError` for an invalid recognizer option. `Recognize` returns the context error before it starts, and an error that matches `ErrRecognize` for an image that is no PNG.

## Choosing an engine

Package `ocr/auto` picks an engine. `auto.Choose(ctx, auto.Options{})` returns `ocr.EngineTesseract` when the `tesseract` program runs, else `ocr.EngineGlyph`. An explicit `Options.Engine` wins. An explicit Tesseract that is missing returns the `*InvalidArgumentError` with the install hint of `tesseract.RequireInstalled`. `auto.New` builds the chosen engine with its default options. `Options.TesseractProgram` sets the program.

## The convert command

`stbox convert film.sup --ocr --to srt` runs `Run` with the chosen engine. The `ocr` option group is described in [cli.md](cli.md#ocr). `--ocr-database FILE` loads a `.nocr` file once per run and selects the glyph engine.

## VobSub input

A VobSub read gives image cues, so OCR can read DVD subtitles. See [VobSub in formats.md](formats.md#vobsub). Example: `stbox.Load("movie.idx", stbox.ReadOptions{})` reads `movie.idx` and `movie.sub` into image cues without lines.

## Tests

Package `ocr/ocrtest` holds `Engine`, a fake that returns the same `Text` for every image and records each call with its image and language.

`ocr/glyph` reads the PGS and VobSub fixtures and compares the SubRip output with `pgs/text_1080p.ocr.srt` and `vobsub/text-pal.ocr.srt`. With the Latin database and no line context it reads the I and l errors that `fixing` repairs, as `fixing/text_1080p.ocr.srt` and `fixing/text-pal.ocr.srt` keep them.

## Differences from PHP

- A Tesseract program that cannot start gives its start error. PHP reports `exits with code -1`. See [php-differences.md](php-differences.md).
- The cue gets the cleaned lines. The report holds the raw result of the engine. PHP `docs/ocr.md` says the library drops empty lines, which only the cue setter does.
- A `Text` with a confidence of NaN fails `Validate`. See [php-differences.md](php-differences.md).
- PHP `docs/ocr.md` says lenient mode keeps the cues before framing damage of a PGS file and is silent on other damage. Go skips only the display set with damage inside a segment, see [lenient-parsing.md](lenient-parsing.md#pgs).
- PHP `docs/ocr.md` says the PGS writer reduces "an image with more than 255 colors". The rule is the one above: the shown images together, above 256 colours.
- The PGS writer returns an `*UnwritableError` for a PNG that does not decode. PHP lets the decoder exception escape.
