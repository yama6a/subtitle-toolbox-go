package tesseract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/markup"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/tesseract"
)

// scripts holds the programs that TestMain writes before any test runs. A script that a parallel test writes
// can fail to start with "text file busy" while another test forks.
var scripts struct {
	dir                                                                   string
	fake, logged, loggedFail                                              string
	hangs, flaky, fresh, versionOnStderr, otherProgram, versionThenFailed string
}

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if runtime.GOOS == "windows" {
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "stbox-tesseract-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	wd, _ := os.Getwd()
	fake := filepath.Join(wd, "..", "..", "testdata", "ocr", "fake-tesseract", "tesseract")
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil { //nolint:gosec // the test script must run
			panic(err)
		}
		return p
	}
	scripts.dir = dir
	scripts.fake = fake
	scripts.logged = write("logged", `FAKE_TESSERACT_LOG="`+dir+`/logged.path" exec "`+fake+`" "$@"`)
	scripts.loggedFail = write("logged-fail", `case "$1" in --*) exec "`+fake+`" "$@";; esac; echo "$1" > "`+dir+`/logged-fail.path"; echo "Error during processing." >&2; exit 1`)
	// The sleep is a child that keeps standard output open after the kill, as a shell wrapper would.
	const versions = `case "$1" in --version) echo "tesseract 5"; exit 0;; esac; `
	scripts.hangs = write("hangs", versions+`case "$1" in --list-langs) printf 'List\neng\n'; exit 0;; esac; sleep 5`)
	scripts.flaky = write("flaky", versions+`[ -e "`+dir+`/flaky.ready" ] || sleep 5; printf 'List\neng\n'`)
	scripts.fresh = write("fresh", `exec "`+fake+`" "$@"`)
	scripts.versionOnStderr = write("stderr", `echo "tesseract 4.0.0" >&2`)
	scripts.otherProgram = write("other", `echo "ImageMagick 7"`)
	scripts.versionThenFailed = write("exit1", `echo "tesseract 5"; exit 1`)
	return m.Run()
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake program is a shell script")
	}
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func testImage(t *testing.T, screenHeight int) model.Image {
	t.Helper()
	return model.Image{PNG: testPNG(t, 40, 20), X: 7, Y: 9, Width: 40, Height: 20, ScreenWidth: 1920, ScreenHeight: screenHeight}
}

func TestTesseractOptionsValidate(t *testing.T) {
	t.Parallel()

	mode := func(n int) *int { return &n }
	tests := []struct {
		name string
		opts tesseract.Options
		want string
	}{
		{"mode 14", tesseract.Options{PageSegmentationMode: mode(14)}, "tesseract.Options.PageSegmentationMode: The page segmentation mode must be from 0 to 13, got 14."},
		{"mode -1", tesseract.Options{PageSegmentationMode: mode(-1)}, "tesseract.Options.PageSegmentationMode: The page segmentation mode must be from 0 to 13, got -1."},
		{"scale 0.5", tesseract.Options{Scale: 0.5}, "tesseract.Options.Scale: The scale must be from 1 to 8, got 0.5."},
		{"scale 9", tesseract.Options{Scale: 9}, "tesseract.Options.Scale: The scale must be from 1 to 8, got 9."},
		{"threshold 256", tesseract.Options{Threshold: 256}, "tesseract.Options.Threshold: The threshold must be from 1 to 255, got 256."},
		{"threshold -1", tesseract.Options{Threshold: -1}, "tesseract.Options.Threshold: The threshold must be from 1 to 255, got -1."},
		{"mode 0 is legal", tesseract.Options{PageSegmentationMode: mode(0), Scale: 8, Threshold: 255}, ""},
		{"zero value", tesseract.Options{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tesseract.New(tt.opts)
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestTesseractInstalled(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	tests := []struct {
		name    string
		program string
		want    bool
	}{
		{"fake tesseract", scripts.fake, true},
		{"version on standard error", scripts.versionOnStderr, true},
		{"other program", scripts.otherProgram, false},
		{"exit code 1", scripts.versionThenFailed, false},
		{"missing program", filepath.Join(scripts.dir, "missing"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tesseract.Installed(context.Background(), tt.program))
		})
	}
}

func TestTesseractRecognize(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	mode := func(n int) *int { return &n }
	tests := []struct {
		name     string
		opts     tesseract.Options
		language ocr.Language
		screen   int
		want     []string
	}{
		{"defaults on a 1080 line screen", tesseract.Options{}, "", 1080, []string{"eng psm6", "60x40"}},
		{"language and mode reach the program", tesseract.Options{Language: "deu", PageSegmentationMode: mode(3)}, "", 1080, []string{"deu psm3", "60x40"}},
		{"the language of Recognize wins", tesseract.Options{Language: "deu"}, "eng+osd", 1080, []string{"eng+osd psm6", "60x40"}},
		{"mode 0", tesseract.Options{PageSegmentationMode: mode(0)}, "", 1080, []string{"eng psm0", "60x40"}},
		{"scale 2 below 720 lines", tesseract.Options{}, "", 576, []string{"eng psm6", "100x60"}},
		{"explicit scale", tesseract.Options{Scale: 3}, "", 576, []string{"eng psm6", "140x80"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.opts.Program = scripts.fake
			e, err := tesseract.New(tt.opts)
			require.NoError(t, err)
			got, err := e.Recognize(context.Background(), testImage(t, tt.screen), tt.language)
			require.NoError(t, err)
			require.Len(t, got.Lines, 2)
			assert.Equal(t, tt.want[0], got.Lines[0])
			assert.Equal(t, tt.want[1], got.Lines[1])
			require.NotNil(t, got.Confidence)
			assert.InDelta(t, 0.8, *got.Confidence, 1e-9)
		})
	}
}

func TestTesseractRecognizeErrors(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	missing := filepath.Join(scripts.dir, "missing")
	tests := []struct {
		name        string
		program     string
		language    ocr.Language
		image       model.Image
		wantMsg     string
		wantRecog   bool
		wantInvalid bool
		logFile     string
	}{
		{
			name: "missing program", program: missing, wantInvalid: true,
			wantMsg: `the program "` + missing + `" is missing. Install Tesseract with: apt install tesseract-ocr (Debian, Ubuntu)`,
		},
		{
			name: "missing language", program: scripts.fake, language: "fra+deu+rus", wantInvalid: true,
			wantMsg: `Cannot run OCR with Tesseract in the language "fra+deu+rus": the language data of fra, rus is missing. Install it, for example with apt install tesseract-ocr-fra. The installed languages are: deu, eng, osd.`,
		},
		{
			name: "failing program", program: scripts.loggedFail, wantRecog: true, logFile: "logged-fail.path",
			wantMsg: "Tesseract exits with code 1 for the cue image at 7, 9: Error during processing.",
		},
		{
			name: "broken PNG", program: scripts.fake, wantRecog: true, wantInvalid: true, image: model.Image{PNG: []byte("no png"), X: 7, Y: 9},
			wantMsg: "Cannot read the cue image at 7, 9:",
		},
		{name: "success removes the file", program: scripts.logged, logFile: "logged.path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, err := tesseract.New(tesseract.Options{Program: tt.program})
			require.NoError(t, err)
			im := tt.image
			if im.PNG == nil {
				im = testImage(t, 1080)
			}
			_, err = e.Recognize(context.Background(), im, tt.language)
			if tt.wantMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantMsg)
				assert.Equal(t, tt.wantRecog, errors.Is(err, ocr.ErrRecognize))
				assert.Equal(t, tt.wantInvalid, errors.Is(err, model.ErrInvalidArgument))
			} else {
				require.NoError(t, err)
			}
			if tt.logFile != "" {
				path, err := os.ReadFile(filepath.Join(scripts.dir, tt.logFile))
				require.NoError(t, err)
				_, err = os.Stat(strings.TrimSpace(string(path))) //nolint:gosec // the path comes from the test script
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestTesseractCancel(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	deadline := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 300*time.Millisecond)
	}
	t.Run("deadline during the run returns the context error", func(t *testing.T) {
		t.Parallel()

		e, err := tesseract.New(tesseract.Options{Program: scripts.hangs})
		require.NoError(t, err)
		ctx, cancel := deadline()
		defer cancel()
		start := time.Now()
		_, err = e.Recognize(ctx, testImage(t, 1080), "")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.NotErrorIs(t, err, ocr.ErrRecognize)
		assert.Less(t, time.Since(start), 4*time.Second, "WaitDelay must stop the wait for the child")
	})
	t.Run("deadline during the language list is not cached", func(t *testing.T) {
		t.Parallel()

		e, err := tesseract.New(tesseract.Options{Program: scripts.flaky})
		require.NoError(t, err)
		ctx, cancel := deadline()
		defer cancel()
		require.ErrorIs(t, e.RequireLanguages(ctx, ""), context.DeadlineExceeded)

		require.NoError(t, os.WriteFile(filepath.Join(scripts.dir, "flaky.ready"), nil, 0o600))
		require.NoError(t, e.RequireLanguages(context.Background(), ""))
	})
	t.Run("cancelled context is no missing program", func(t *testing.T) {
		t.Parallel()

		e, err := tesseract.New(tesseract.Options{Program: scripts.fresh})
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = e.RequireLanguages(ctx, "")
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, model.ErrInvalidArgument)
		assert.False(t, tesseract.Installed(ctx, scripts.fresh))
	})
}

func TestTesseractRequireLanguages(t *testing.T) {
	t.Parallel()
	skipOnWindows(t)

	e, err := tesseract.New(tesseract.Options{Program: scripts.fake, Language: "deu+eng"})
	require.NoError(t, err)
	require.NoError(t, e.RequireLanguages(context.Background(), ""))
	require.ErrorIs(t, e.RequireLanguages(context.Background(), "xyz"), model.ErrInvalidArgument)
}

// TestTesseractAccuracy reads the PGS and VobSub fixtures with a real Tesseract. It skips when the program is missing,
// unless STBOX_REQUIRE_TESSERACT=1 is set.
func TestTesseractAccuracy(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("tesseract"); err != nil {
		if os.Getenv("STBOX_REQUIRE_TESSERACT") == "1" {
			t.Fatal("tesseract is not installed")
		}
		t.Skip("tesseract is not installed")
	}

	tests := []struct {
		name     string
		file     string
		idx      string
		drawn    string
		language ocr.Language
	}{
		{"PGS 1080p, 44 to 60 px", "pgs/text_1080p.sup", "", "ocr/drawn/text_1080p.json", ocr.English},
		{"VobSub 576p, 24 to 30 px", "vobsub/text-pal.sub", "vobsub/text-pal.idx", "ocr/drawn/text-pal.json", ocr.English},
		{"PGS 1080p, Cyrillic", "pgs/text_cyrillic_1080p.sup", "", "ocr/drawn/text_cyrillic_1080p.json", ocr.Russian},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s *model.Subtitle
			var err error
			if tt.idx == "" {
				s, _, err = pgs.Read(bytes.NewReader(testfiles.Read(t, tt.file)), model.CommonReadOptions{}, pgs.ReadOptions{})
			} else {
				idx := string(testfiles.Read(t, tt.idx))
				s, _, err = vobsub.Read(bytes.NewReader(testfiles.Read(t, tt.file)), model.CommonReadOptions{}, vobsub.ReadOptions{IDX: idx})
			}
			require.NoError(t, err)
			var drawn [][]string
			require.NoError(t, json.Unmarshal(testfiles.Read(t, tt.drawn), &drawn))

			e, err := tesseract.New(tesseract.Options{})
			require.NoError(t, err)
			_, err = ocr.Run(context.Background(), s, e, tt.language)
			require.NoError(t, err)
			require.Equal(t, len(drawn), s.Len())

			var errs, length int
			for i := range s.Len() {
				cue, _ := s.Cue(i)
				want := []rune(plain(drawn[i]))
				errs += editDistance(want, []rune(plain(cue.Lines)))
				length += len(want)
			}
			accuracy := 1 - float64(errs)/float64(length)
			assert.GreaterOrEqual(t, accuracy, 0.99, "%d errors in %d characters", errs, length)
		})
	}
}

func plain(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = markup.PlainText(l)
	}
	return strings.Join(out, "\n")
}

func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ra := range a {
		cur := make([]int, len(b)+1)
		cur[0] = i + 1
		for j, rb := range b {
			cost := 1
			if ra == rb {
				cost = 0
			}
			cur[j+1] = min(prev[j+1]+1, cur[j]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
