package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// infoCommand prints the format, the source encoding, the cue count, the warnings and the metadata of each input.
type infoCommand struct{ report reportOutput }

var _ fileCommand = &infoCommand{}

// infoEntry is one object of the info --json list. Field order is the JSON key order.
type infoEntry struct {
	File     string          `json:"file"`
	Format   stbox.Format    `json:"format"`
	Encoding *string         `json:"encoding"` // nil prints null for a format without a source encoding
	Metadata orderedMetadata `json:"metadata"`
	Warnings []warningJSON   `json:"warnings"`
}

// orderedMetadata writes its keys in byte order and prints {} when empty.
type orderedMetadata map[string]string

// MarshalJSON writes the keys in byte order, which encoding/json does for a map. The method makes the order a
// rule, prints {} for a nil map and keeps <, > and & as they are.
func (m orderedMetadata) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]string(m)); err != nil {
		return nil, fmt.Errorf("encode the metadata: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func (*infoCommand) name() string { return "info" }

func (*infoCommand) summary() string {
	return "Print the format, the encoding and the cue count of subtitle files."
}

func (*infoCommand) usage() []string { return []string{"<input>... [--json] [options]"} }

func (*infoCommand) details() string { return "" }

func (*infoCommand) options() []option {
	return slices.Concat([]option{flagOption("json", jsonHelp)}, inputOptions())
}

func (c *infoCommand) run(ctx context.Context, a *args, console *console) error {
	return runFiles(ctx, a, console, c)
}

func (c *infoCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	c.report.prepare(a, r)
	return nil
}

func (*infoCommand) checkInputs(*runner, []string) error          { return nil }
func (*infoCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (c *infoCommand) process(_ context.Context, r *runner, in *input) error {
	metadata := in.sub.AllMetadata()
	entry := infoEntry{
		File:     in.arg,
		Format:   in.info.Format,
		Metadata: metadata,
		Warnings: warningsJSON(in.info.Warnings),
	}
	rows := [][]string{{"Format:", string(in.info.Format)}}
	if enc := in.info.SourceEncoding; enc != "" {
		entry.Encoding = &enc
		rows = append(rows, []string{"Encoding:", enc})
	}
	rows = append(rows, []string{"Cues:", strconv.Itoa(in.sub.Len())})
	if n := len(in.info.Warnings); n > 0 {
		rows = append(rows, []string{"Warnings:", strconv.Itoa(n)})
	}
	for _, key := range slices.Sorted(maps.Keys(metadata)) {
		rows = append(rows, []string{"Metadata " + key + ":", metadata[key]})
	}
	text := in.label() + "\n" + table(rows, 2, 1)
	if r.succeeded > 0 {
		text = "\n" + text
	}
	c.report.emit(r, text, entry)
	return nil
}

func (c *infoCommand) finish(r *runner) error { return c.report.finish(r) }
