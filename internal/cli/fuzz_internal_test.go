package cli

import (
	"errors"
	"strings"
	"testing"
)

// FuzzParseArgs feeds the words of the input, split at NUL, to the parser with the options of convert.
func FuzzParseArgs(f *testing.F) {
	for _, seed := range []string{
		"trip.srt\x00--to\x00vtt",
		"--to=vtt\x00-o\x00-\x00--\x00--help",
		"-2.5\x00--bom=1\x00--to",
		"--line-ending\x00crlf\x00--line-ending\x00lf",
		"-ofile\x00---\x00--=\x00-",
	} {
		f.Add(seed)
	}
	opts := (&stubConvert{}).options()
	f.Fuzz(func(t *testing.T, in string) {
		a, err := parseArgs(strings.Split(in, "\x00"), opts)
		if err != nil {
			var ue *usageError
			if !errors.As(err, &ue) {
				t.Fatalf("error %v is not a usage error", err)
			}
			return
		}
		for _, o := range opts {
			if o.value == "" && len(a.Values(o.name)) > 0 {
				t.Fatalf("flag --%s has values", o.name)
			}
			if !o.repeatable && len(a.Values(o.name)) > 1 {
				t.Fatalf("--%s has %d values", o.name, len(a.Values(o.name)))
			}
		}
	})
}
