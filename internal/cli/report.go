package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

const jsonHelp = "Print JSON: a list with one object for each input file."

// reportOutput is the part of a report command that prints a text block for each input as soon as it is done,
// or one JSON array of entries at the end with --json.
type reportOutput struct {
	json    bool
	entries []any
}

// prepare reads --json. With --json, the batch summary goes to stderr.
func (o *reportOutput) prepare(a *args, r *runner) {
	o.json = a.Has("json")
	o.entries = []any{}
	r.reportToStderr = o.json
}

// emit prints the text of one input now, or keeps its JSON entry for finish.
func (o *reportOutput) emit(r *runner, text string, entry any) {
	if o.json {
		o.entries = append(o.entries, entry)
		return
	}
	r.c.out(text)
}

// finish prints the JSON array and LF. An array without entries prints [].
func (o *reportOutput) finish(r *runner) error {
	if !o.json {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(o.entries); err != nil {
		return fmt.Errorf("encode the JSON report: %w", err)
	}
	r.c.out(buf.String())
	return nil
}

// warningJSON is one parse warning of a --json entry. Field order is the JSON key order.
type warningJSON struct {
	LineNumber *int   `json:"lineNumber"` // nil prints null for a warning without a line
	BlockIndex *int   `json:"blockIndex"` // nil prints null for a warning without a block
	Message    string `json:"message"`
	Action     string `json:"action"`
}

// warningsJSON returns the warnings in the shape of --json, never nil, so an empty list prints [].
func warningsJSON(warnings []stbox.ParseWarning) []warningJSON {
	out := make([]warningJSON, len(warnings))
	for i, w := range warnings {
		out[i] = warningJSON{Message: w.Message, Action: w.Action.String()}
		if w.Line != stbox.NoLine {
			out[i].LineNumber = &w.Line
		}
		if w.Block != stbox.NoBlock {
			out[i].BlockIndex = &w.Block
		}
	}
	return out
}
