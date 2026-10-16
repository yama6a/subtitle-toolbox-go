// Package cli is the command line framework of stbox: the dispatcher, the option parser, the help layout, the
// input loop, the output plan and the exit codes.
package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const programName = "stbox"

// Env holds what one call reads from its process.
type Env struct {
	// Stdin is standard input, read by the input argument "-".
	Stdin io.Reader
	// Stdout gets subtitle data, reports and help.
	Stdout io.Writer
	// Stderr gets warnings, failures and every other diagnostic.
	Stderr io.Writer
	// Dir is the directory for relative paths, "" for the directory of the process.
	Dir string
	// Getenv reads an environment variable: os.Getenv in main, a map lookup in tests.
	Getenv func(key string) string
	// Version is the version that --version and the help print.
	Version string
}

// Run runs one stbox call. It prints its own messages and returns nil for exit code 0, else an *ExitError.
func Run(ctx context.Context, args []string, env Env) error {
	return (&app{commands: commands()}).run(ctx, args, env)
}

// commands returns new instances of the commands of the binary, in help order: convert, info, validate, sync,
// diff, translate, dual, hls, formats. A command keeps the state of its run, so each call gets its own.
func commands() []command {
	return []command{
		&infoCommand{},
		formatsCommand{},
	}
}

// app holds the commands of the binary, in the order of the help.
type app struct {
	commands []command
	// beforeCreate runs right before each exclusive create of an output file. Tests create the file there.
	beforeCreate func(path string)
}

// console holds the streams and settings of one call.
type console struct {
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	dir          string
	getenv       func(key string) string
	version      string
	beforeCreate func(path string)
}

func (c *console) out(s string) { _, _ = io.WriteString(c.stdout, s) }

func (c *console) err(s string) { _, _ = io.WriteString(c.stderr, s) }

// path returns the path that the CLI opens for a path argument. Messages keep the argument as written.
func (c *console) path(p string) string {
	if c.dir == "" || filepath.IsAbs(p) {
		return p
	}
	return c.dir + string(filepath.Separator) + p
}

func (a *app) run(ctx context.Context, words []string, env Env) error {
	c := &console{
		stdin: env.Stdin, stdout: env.Stdout, stderr: env.Stderr, dir: env.Dir, getenv: env.Getenv,
		version: env.Version, beforeCreate: a.beforeCreate,
	}
	if len(words) == 0 || words[0] == "--help" || words[0] == "-h" {
		c.out(a.help(c.version))
		return nil
	}
	switch words[0] {
	case "--version", "-V":
		c.out(c.version + "\n")
		return nil
	case "help":
		return a.runHelp(c, words[1:])
	}
	i := a.index(words[0])
	if i < 0 {
		return usageFailure(c, "", "Unknown command "+quote(words[0])+".")
	}
	cmd := a.commands[i]
	if topic, ok := helpRequest(words[1:]); ok {
		return printHelp(c, cmd, topic)
	}
	parsed, err := parseArgs(words[1:], cmd.options())
	if err == nil {
		err = cmd.run(ctx, parsed, c)
	}
	return report(c, cmd, err)
}

func (a *app) runHelp(c *console, words []string) error {
	if len(words) == 0 || words[0] == "help" {
		c.out(a.help(c.version))
		return nil
	}
	i := a.index(words[0])
	if i < 0 {
		return usageFailure(c, "", "Unknown command "+quote(words[0])+".")
	}
	topic := ""
	if len(words) > 1 {
		topic = words[1]
	}
	return printHelp(c, a.commands[i], topic)
}

// index returns the index of the command name, or -1.
func (a *app) index(name string) int {
	return slices.IndexFunc(a.commands, func(cmd command) bool { return cmd.name() == name })
}

// helpRequest scans the words after a command name for --help or -h, up to "--". The next word is the help
// topic when it does not start with "-".
func helpRequest(words []string) (topic string, ok bool) {
	for i, word := range words {
		switch word {
		case "--":
			return "", false
		case "--help", "-h":
			if i+1 < len(words) && !strings.HasPrefix(words[i+1], "-") {
				return words[i+1], true
			}
			return "", true
		}
	}
	return "", false
}

func printHelp(c *console, cmd command, topic string) error {
	text, err := helpOf(cmd, topic)
	if err != nil {
		return usageFailure(c, cmd.name(), message(err))
	}
	c.out(text)
	return nil
}

// report prints the error of a command run and returns its *ExitError.
func report(c *console, cmd command, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*ExitError](err); ok {
		return err
	}
	if e, ok := errors.AsType[*usageError](err); ok {
		return usageFailure(c, cmd.name(), e.msg)
	}
	c.err("Error: " + message(err) + "\n")
	return &ExitError{Code: exitFile}
}

// usageFailure prints a usage error with the hint to the help of cmd, or to the global help for "".
func usageFailure(c *console, cmd, msg string) error {
	hint := programName + " help"
	if cmd != "" {
		hint += " " + cmd
	}
	c.err("Error: " + msg + "\nRun " + quote(hint) + " for the usage.\n")
	return &ExitError{Code: exitUsage}
}

func (a *app) help(version string) string {
	rows := make([][]string, 0, len(a.commands)+1)
	for _, cmd := range a.commands {
		rows = append(rows, []string{cmd.name(), cmd.summary()})
	}
	rows = append(rows, []string{"help", "Show the help of a command."})
	about := wrap(`Run "stbox help <command>" or "stbox <command> --help" for the options of a command. ` +
		"A file argument of - reads standard input. One input goes to standard output, or to -o FILE. " +
		"Several inputs need --output-dir DIR. The tool never overwrites a file.")
	exitCodes := table([][]string{
		{"0", "Success."},
		{"2", "Invalid arguments."},
		{"3", "A file could not be read or written."},
		{"130", "Stopped by Ctrl-C or SIGTERM. Finished outputs stay."},
	}, 2, 2)
	options := table([][]string{{"-h, --help", "Show this help."}, {"-V, --version", "Print the version."}}, 2, 2)
	return programName + " " + version + "\n" +
		"Converts, retimes, checks and fixes subtitle files.\n\n" +
		"Usage: " + programName + " <command> [<file>...] [options]\n\n" +
		"Commands:\n" + table(rows, 2, 2) + "\n" + about + "\nExit codes:\n" + exitCodes + "\nOptions:\n" + options
}

// exists reports whether path names a file, a directory or a symbolic link, also one whose target is missing.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
