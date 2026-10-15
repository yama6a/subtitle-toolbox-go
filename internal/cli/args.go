package cli

import (
	"flag"
	"io"
	"strings"
)

// option is one command line option.
type option struct {
	name       string // long name without dashes
	short      string // one letter or ""
	value      string // value name in the help, "" for a flag
	help       string
	repeatable bool
}

func flagOption(name, help string) option {
	return option{name: name, help: help}
}

func valueOption(name, value, help string) option {
	return option{name: name, value: value, help: help}
}

// synopsis returns the option as the help lists it, for example "-o, --output FILE" or "--bom".
func (o option) synopsis() string {
	s := "--" + o.name
	if o.short != "" {
		s = "-" + o.short + ", " + s
	}
	if o.value != "" {
		s += " " + o.value
	}
	return s
}

// collector is the flag.Value of one option. It keeps the raw strings in order.
type collector struct {
	opt    option
	values []string
	set    bool
}

var _ flag.Value = &collector{}

func (c *collector) String() string { return strings.Join(c.values, " ") }

func (c *collector) Set(v string) error {
	c.set = true
	if c.opt.value != "" {
		c.values = append(c.values, v)
	}
	return nil
}

func (c *collector) IsBoolFlag() bool { return c.opt.value == "" }

// args holds the parsed words of one call and the typed getters.
type args struct {
	positionals []string
	fs          *flag.FlagSet
}

// parseArgs reads the words after the command name. A pre-pass applies the PHP syntax and messages, so that
// flag.FlagSet.Parse gets only known options in the forms -name and -name=value. On its own, flag would stop
// at the first positional argument, accept -name for every long option, accept --flag=false and keep the
// last of 2 values.
func parseArgs(words []string, opts []option) (*args, error) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	p := &prePass{byName: make(map[string]option, len(opts)), byShort: make(map[string]option), given: make(map[string]bool)}
	for _, o := range opts {
		c := &collector{opt: o}
		fs.Var(c, o.name, o.help)
		p.byName[o.name] = o
		if o.short != "" {
			fs.Var(c, o.short, o.help)
			p.byShort[o.short] = o
		}
	}
	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == "--" {
			p.positionals = append(p.positionals, words[i+1:]...)
			break
		}
		if word == "-" || !strings.HasPrefix(word, "-") {
			p.positionals = append(p.positionals, word)
			continue
		}
		var err error
		if i, err = p.option(words, i); err != nil {
			return nil, err
		}
	}
	if err := fs.Parse(p.known); err != nil {
		return nil, usage(err.Error())
	}
	return &args{positionals: p.positionals, fs: fs}, nil
}

type prePass struct {
	byName, byShort map[string]option
	given           map[string]bool
	// known holds the option words for flag.FlagSet.Parse.
	known       []string
	positionals []string
}

// option reads the option word at words[i] and returns the index of its last word.
func (p *prePass) option(words []string, i int) (int, error) {
	o, value, hasValue, err := findOption(words[i], p.byName, p.byShort)
	switch {
	case err != nil:
		return i, err
	case o.value == "" && hasValue:
		return i, usage("The option --" + o.name + " takes no value.")
	case o.value == "":
		p.known = append(p.known, "-"+o.name)
		return i, nil
	case !hasValue && i+1 >= len(words):
		return i, usage("The option --" + o.name + " needs a value.")
	case !hasValue:
		i++
		value = words[i]
	}
	if p.given[o.name] && !o.repeatable {
		return i, usage("The option --" + o.name + " is given twice.")
	}
	p.given[o.name] = true
	p.known = append(p.known, "-"+o.name+"="+value)
	return i, nil
}

// findOption returns the option that word names, and the value after "=" of a long option.
func findOption(word string, byName, byShort map[string]option) (o option, value string, hasValue bool, err error) {
	name, ok := strings.CutPrefix(word, "--")
	if !ok {
		o, ok = byShort[word[1:]]
		if !ok {
			return option{}, "", false, usage("Unknown option " + word + ".")
		}
		return o, "", false, nil
	}
	name, value, hasValue = strings.Cut(name, "=")
	o, ok = byName[name]
	if !ok {
		return option{}, "", false, usage("Unknown option --" + name + ".")
	}
	return o, value, hasValue, nil
}

func (a *args) collector(name string) *collector {
	f := a.fs.Lookup(name)
	if f == nil {
		return nil
	}
	c, _ := f.Value.(*collector)
	return c
}

// Has reports whether the call gives the option name.
func (a *args) Has(name string) bool {
	c := a.collector(name)
	return c != nil && c.set
}

// Value returns the first value of the option name.
func (a *args) Value(name string) (string, bool) {
	values := a.Values(name)
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// Values returns every value of the option name, in the order of the call.
func (a *args) Values(name string) []string {
	c := a.collector(name)
	if c == nil {
		return nil
	}
	return c.values
}
