package cli

import (
	"context"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// formatsCommand lists the formats that the library can read or write.
type formatsCommand struct{}

var _ command = formatsCommand{}

func (formatsCommand) name() string { return "formats" }

func (formatsCommand) summary() string {
	return "List the format names and file extensions for --from and --to."
}

func (formatsCommand) usage() []string { return []string{""} }

func (formatsCommand) details() string {
	return "When two formats share an extension, the first one in the list owns it. " +
		"So .json is the JSON of this library. Pass --from to read another one."
}

func (formatsCommand) options() []option { return nil }

func (formatsCommand) run(_ context.Context, a *args, c *console) error {
	if len(a.positionals) > 0 {
		return usage("The formats command takes no arguments.")
	}
	rows := [][]string{{"Name", "Extensions", "Read", "Write"}}
	for _, f := range stbox.Formats() {
		if !f.CanRead() && !f.CanWrite() {
			continue
		}
		exts := f.Extensions()
		for i, e := range exts {
			exts[i] = "." + e
		}
		rows = append(rows, []string{string(f), strings.Join(exts, " "), yesNo(f.CanRead()), yesNo(f.CanWrite())})
	}
	c.out(table(rows, 0, 2))
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
