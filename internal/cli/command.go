package cli

import (
	"context"
	"slices"
	"strings"
)

// command is one stbox command.
type command interface {
	name() string
	// summary is the one sentence of the global help and of the command help.
	summary() string
	// usage returns the usage lines after "stbox NAME".
	usage() []string
	// details is the text after the summary in the command help, or "".
	details() string
	// options lists the options in help order, without -h.
	options() []option
	run(ctx context.Context, a *args, c *console) error
}

// topicHelper is a command whose help reads the topic after --help, such as the option groups of convert.
type topicHelper interface {
	help(topic string) (string, error)
}

func helpOption() option {
	return option{name: "help", short: "h", help: "Show this help."}
}

func helpOf(cmd command, topic string) (string, error) {
	if h, ok := cmd.(topicHelper); ok {
		return h.help(topic)
	}
	return commandHelp(cmd), nil
}

// commandHelp is the help of a command without topics: the header, then the options and -h.
func commandHelp(cmd command) string {
	return helpHeader(cmd) + "\nOptions:\n" + optionTable(slices.Concat(cmd.options(), []option{helpOption()}))
}

// helpHeader returns the usage lines, the summary and the details of cmd.
func helpHeader(cmd command) string {
	var b strings.Builder
	for i, line := range cmd.usage() {
		prefix := "Usage: "
		if i > 0 {
			prefix = "       "
		}
		b.WriteString(usageLine(prefix+programName+" "+cmd.name(), line))
	}
	b.WriteString("\n" + wrap(cmd.summary()))
	if d := cmd.details(); d != "" {
		b.WriteString("\n" + wrap(d))
	}
	return b.String()
}

func optionTable(opts []option) string {
	rows := make([][]string, len(opts))
	for i, o := range opts {
		rows[i] = []string{o.synopsis(), o.help}
	}
	return table(rows, 2, 2)
}
