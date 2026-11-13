package tesseract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
)

//nolint:gochecknoglobals // installed languages per program, read once per process
var languages = struct {
	sync.Mutex
	byProgram map[string][]string
}{byProgram: map[string][]string{}}

// run starts program with args and returns its standard output and standard error.
// A non-zero exit code is not an error here, because only the callers know what it means.
func run(ctx context.Context, program string, args ...string) (stdout, stderr string, exit int, err error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.String(), errOut.String(), ee.ExitCode(), nil
	}
	return out.String(), errOut.String(), 0, err
}

// Installed reports whether program runs and prints a Tesseract version.
// Tesseract 4.0 and older print the version to standard error.
func Installed(ctx context.Context, program string) bool {
	stdout, stderr, exit, err := run(ctx, program, "--version")
	return err == nil && exit == 0 && strings.HasPrefix(strings.TrimLeft(stdout+stderr, " \t\r\n"), "tesseract")
}

const missingProgram = "Cannot run OCR with Tesseract: the program \"%s\" is missing. Install Tesseract with: " +
	"apt install tesseract-ocr (Debian, Ubuntu), apk add tesseract-ocr tesseract-ocr-data-eng (Alpine), " +
	"dnf install tesseract (Fedora), brew install tesseract (macOS), " +
	"or the installer from https://github.com/UB-Mannheim/tesseract/wiki (Windows)."

// RequireLanguages returns an error when the program or the data of a part of language is missing.
// An empty language checks Options.Language. The error is a *model.InvalidArgumentError.
func (e *Engine) RequireLanguages(ctx context.Context, language ocr.Language) error {
	if language == "" {
		language = e.language
	}
	installed, err := e.installedLanguages(ctx)
	if err != nil {
		return err
	}
	var missing []string
	for part := range strings.SplitSeq(string(language), "+") {
		if !slices.Contains(installed, part) {
			missing = append(missing, part)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &model.InvalidArgumentError{Msg: "Cannot run OCR with Tesseract in the language \"" + string(language) +
		"\": the language data of " + strings.Join(missing, ", ") + " is missing. Install it, for example with apt install tesseract-ocr-" +
		missing[0] + ". The installed languages are: " + strings.Join(installed, ", ") + "."}
}

func (e *Engine) installedLanguages(ctx context.Context) ([]string, error) {
	languages.Lock()
	defer languages.Unlock()
	if list, ok := languages.byProgram[e.program]; ok {
		return list, nil
	}
	if !Installed(ctx, e.program) {
		return nil, &model.InvalidArgumentError{Msg: fmt.Sprintf(missingProgram, e.program)}
	}
	stdout, stderr, _, err := run(ctx, e.program, "--list-langs")
	if err != nil {
		return nil, err
	}
	var list []string
	lines := strings.Split(strings.TrimSpace(stdout+stderr), "\n")
	for _, line := range lines[1:] {
		if line = strings.TrimSpace(line); line != "" {
			list = append(list, line)
		}
	}
	languages.byProgram[e.program] = list
	return list, nil
}
