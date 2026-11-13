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

## Tests

Package `ocr/ocrtest` holds `Engine`, a fake that returns the same `Text` for every image and records each call with its image and language.

## Differences from PHP

- The cue gets the cleaned lines. The report holds the raw result of the engine. PHP `docs/ocr.md` says the library drops empty lines, which only the cue setter does.
- A `Text` with a confidence of NaN fails `Validate`. See [php-differences.md](php-differences.md).
- PHP `docs/ocr.md` says lenient mode keeps the cues before framing damage of a PGS file and is silent on other damage. Go skips only the display set with damage inside a segment, see [lenient-parsing.md](lenient-parsing.md#pgs).
- PHP `docs/ocr.md` says the PGS writer reduces "an image with more than 255 colors". The rule is the one above: the shown images together, above 256 colours.
- The PGS writer returns an `*UnwritableError` for a PNG that does not decode. PHP lets the decoder exception escape.
