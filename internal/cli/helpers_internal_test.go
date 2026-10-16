package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

const testVersion = "1.2.3"

// The stand-ins of convert and info run the shared parts of a write command and of a report command, without
// edits and without statistics. testCommands drops a stand-in when commands holds the real command.
type stubConvert struct{ out writeOutput }

var _ fileCommand = &stubConvert{}

func (*stubConvert) name() string    { return "convert" }
func (*stubConvert) summary() string { return "Write subtitle files in the format of --to." }
func (*stubConvert) usage() []string {
	return []string{"<input> [--to FORMAT] [-o FILE] [options]", "<input>... [--to FORMAT] --output-dir DIR [options]"}
}
func (*stubConvert) details() string { return "" }
func (*stubConvert) options() []option {
	return slices.Concat(outputOptions("Output format. Default: the input format."), inputOptions(true))
}

func (s *stubConvert) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, s)
}

func (s *stubConvert) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	return s.out.prepare(a, r)
}

func (s *stubConvert) checkInputs(r *runner, inputs []string) error {
	return s.out.checkInputs(r, inputs, inputs)
}

func (*stubConvert) loadSideFiles(context.Context, *runner) error { return nil }

func (s *stubConvert) process(_ context.Context, r *runner, in *input) error {
	return s.out.write(r, in, nil)
}

func (*stubConvert) finish(*runner) error { return nil }

type stubInfo struct{ report reportOutput }

var _ fileCommand = &stubInfo{}

type stubInfoEntry struct {
	File     string        `json:"file"`
	Format   stbox.Format  `json:"format"`
	Warnings []warningJSON `json:"warnings"`
}

func (*stubInfo) name() string    { return "info" }
func (*stubInfo) summary() string { return "Print the format of subtitle files." }
func (*stubInfo) usage() []string { return []string{"<input>... [--json] [options]"} }
func (*stubInfo) details() string { return "" }
func (*stubInfo) options() []option {
	return slices.Concat([]option{flagOption("json", jsonHelp)}, inputOptions(true))
}

func (s *stubInfo) run(ctx context.Context, a *args, c *console) error { return runFiles(ctx, a, c, s) }

func (s *stubInfo) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	s.report.prepare(a, r)
	return nil
}

func (*stubInfo) checkInputs(*runner, []string) error          { return nil }
func (*stubInfo) loadSideFiles(context.Context, *runner) error { return nil }

func (s *stubInfo) process(_ context.Context, r *runner, in *input) error {
	text := in.label() + "\n" + table([][]string{{"Format:", string(in.info.Format)}}, 2, 1)
	s.report.emit(r, text, stubInfoEntry{File: in.arg, Format: in.info.Format, Warnings: warningsJSON(in.info.Warnings)})
	return nil
}

func (s *stubInfo) finish(r *runner) error { return s.report.finish(r) }

// testCommands returns the commands of the binary, and the stand-ins of the commands that the binary lacks.
func testCommands() []command {
	cmds := commands()
	for _, stub := range []command{&stubConvert{}, &stubInfo{}} {
		if !slices.ContainsFunc(cmds, func(c command) bool { return c.name() == stub.name() }) {
			cmds = append(cmds, stub)
		}
	}
	return cmds
}

// result is what one call prints and its exit code.
type result struct {
	Code   int
	Stdout string
	Stderr string
}

func usageResult(cmd, msg string) result {
	hint := "stbox help"
	if cmd != "" {
		hint += " " + cmd
	}
	return result{Code: exitUsage, Stderr: "Error: " + msg + "\nRun \"" + hint + "\" for the usage.\n"}
}

// call holds the settings of one test call.
type call struct {
	dir   string
	stdin string
	ctx   context.Context //nolint:containedctx // a test call carries the context of its run
	// beforeCreate gets the path of each output file as given, right before its create.
	beforeCreate func(path string)
	commands     []command
}

func (cl call) run(t *testing.T, words ...string) result {
	t.Helper()

	recordOptions(words)
	ctx := cl.ctx
	if ctx == nil {
		ctx = t.Context()
	}
	commands := cl.commands
	if commands == nil {
		commands = testCommands()
	}
	var stdout, stderr bytes.Buffer
	env := Env{
		Stdin: strings.NewReader(cl.stdin), Stdout: &stdout, Stderr: &stderr, Dir: cl.dir,
		Getenv: func(string) string { return "" }, Version: testVersion,
	}
	err := (&app{commands: commands, beforeCreate: cl.beforeCreate}).run(ctx, words, env)
	return result{Code: ExitCode(err), Stdout: stdout.String(), Stderr: stderr.String()}
}

// newDir returns a temporary directory that holds copies of the files of testdata/cli.
func newDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	entries, err := os.ReadDir(testfiles.Path(t, "cli"))
	require.NoError(t, err)
	for _, e := range entries {
		if e.Name() == "SOURCES.md" {
			continue
		}
		copyFile(t, testfiles.Path(t, "cli/"+e.Name()), filepath.Join(dir, e.Name()))
	}
	return dir
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()

	content, err := os.ReadFile(from)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(to), 0o755))
	require.NoError(t, os.WriteFile(to, content, 0o600)) //nolint:gosec // the test writes below its own t.TempDir
}

// runCLI runs words in a new directory with the CLI fixtures.
func runCLI(t *testing.T, words ...string) result {
	t.Helper()

	return call{dir: newDir(t)}.run(t, words...)
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(content)
}

// snapshot returns the hash of every file and the name of every directory below dir, so a test can assert
// that a call wrote nothing.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	fsys := os.DirFS(dir)
	require.NoError(t, fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			files[path] = "dir"
			return nil
		}
		content, err := fs.ReadFile(fsys, path)
		require.NoError(t, err)
		sum := sha256.Sum256(content)
		files[path] = hex.EncodeToString(sum[:])
		return nil
	}))
	return files
}

// convertAs returns the fixture name of testdata/cli in format f, as the library writes it with default options.
func convertAs(t *testing.T, name string, f stbox.Format) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, "cli/"+name), stbox.ReadOptions{})
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, stbox.Write(&buf, s, f, stbox.WriteOptions{}))
	return buf.String()
}

// usedOptions records the options that the tests pass to each command, for TestMain.
var usedOptions = struct {
	sync.Mutex
	byCommand map[string]map[string]bool
}{byCommand: make(map[string]map[string]bool)}

func recordOptions(words []string) {
	if len(words) == 0 {
		return
	}
	cmd, rest := words[0], words[1:]
	if cmd == "help" {
		return
	}
	usedOptions.Lock()
	defer usedOptions.Unlock()
	if usedOptions.byCommand[cmd] == nil {
		usedOptions.byCommand[cmd] = make(map[string]bool)
	}
	for _, w := range rest {
		if w == "--" {
			break
		}
		if w == "-o" {
			w = "--output"
		}
		if name, ok := strings.CutPrefix(w, "--"); ok {
			name, _, _ = strings.Cut(name, "=")
			usedOptions.byCommand[cmd][name] = true
		}
	}
}

// TestMain checks after a full run that every option of every command appears in at least one call. A run that
// selects or lists tests skips the check.
func TestMain(m *testing.M) {
	code := m.Run()
	selected := false
	for _, name := range []string{"test.run", "test.skip", "test.list"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			selected = true
		}
	}
	if missing := unusedOptions(); code == 0 && !selected && len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "options without a test call:", strings.Join(missing, ", "))
		code = 1
	}
	os.Exit(code)
}

func unusedOptions() []string {
	usedOptions.Lock()
	defer usedOptions.Unlock()
	var missing []string
	for _, cmd := range testCommands() {
		for _, o := range cmd.options() {
			if !usedOptions.byCommand[cmd.name()][o.name] {
				missing = append(missing, cmd.name()+" --"+o.name)
			}
		}
	}
	return missing
}
