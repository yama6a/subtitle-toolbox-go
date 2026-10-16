package cli

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2/v2"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// group is one option group of convert.
type group struct {
	name    string
	summary string
	options []option
	// build checks the options of the group and calls add with its edit when a trigger option is set. It reads no
	// file.
	build func(a *args, add func(edit)) error
}

// edit is the work of one group on one input.
type edit interface {
	// apply returns s after the edit, or a new subtitle. label names the input in messages.
	apply(ctx context.Context, s *stbox.Subtitle, c *console, label string) (*stbox.Subtitle, error)
}

// step is one library call of an edit. It returns s or a new subtitle.
type step func(s *stbox.Subtitle) (*stbox.Subtitle, error)

// steps is an edit that runs its library calls in order.
type steps []step

var _ edit = steps{}

func (st steps) apply(_ context.Context, s *stbox.Subtitle, _ *console, _ string) (*stbox.Subtitle, error) {
	for _, call := range st {
		var err error
		if s, err = call(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func inPlace(call func(s *stbox.Subtitle) error) step {
	return func(s *stbox.Subtitle) (*stbox.Subtitle, error) {
		return s, call(s)
	}
}

func addSteps(add func(edit), st steps) {
	if len(st) > 0 {
		add(st)
	}
}

// flagStep is a library call that runs when the command line holds the flag name.
type flagStep struct {
	name string
	call func(s *stbox.Subtitle) error
}

func flagSteps(a *args, flags []flagStep) steps {
	var st steps
	for _, f := range flags {
		if a.Has(f.name) {
			st = append(st, inPlace(f.call))
		}
	}
	return st
}

// convertGroups lists the groups in pipeline order.
func convertGroups() []group {
	return []group{forcedGroup(), replaceGroup(), textGroup(), structureGroup(), retimeGroup(), timingGroup()}
}

func needs(a *args, needed string, opts ...string) error {
	for _, o := range opts {
		if a.Has(o) && !a.Has(needed) {
			return usage("Pass --" + needed + " with --" + o + ".")
		}
	}
	return nil
}

func needsOneOf(a *args, needed []string, opt string) error {
	if !a.Has(opt) {
		return nil
	}
	for _, n := range needed {
		if a.Has(n) {
			return nil
		}
	}
	list := "--" + needed[len(needed)-1]
	if len(needed) > 1 {
		list = "--" + strings.Join(needed[:len(needed)-1], ", --") + " or " + list
	}
	return usage("Pass " + list + " with --" + opt + ".")
}

func forcedGroup() group {
	return group{
		name:    "forced",
		summary: "Keep only the forced cues.",
		options: []option{flagOption("forced-only", "Keep only the forced cues, for example the translations of signs.")},
		build: func(a *args, add func(edit)) error {
			if a.Has("forced-only") {
				add(steps{func(s *stbox.Subtitle) (*stbox.Subtitle, error) { return s.ForcedCuesOnly(), nil }})
			}
			return nil
		},
	}
}

func replaceGroup() group {
	return group{
		name:    "replace",
		summary: "Replace words or patterns in the text.",
		options: []option{
			{
				name: "replace", value: "FROM=TO", repeatable: true,
				help: "Replace FROM with TO in the text between tags, for example --replace colour=color. Repeatable.",
			},
			flagOption("replace-regex",
				`Read each FROM of --replace as a regular expression with delimiters, such as /\.{4,}/. TO can use $1.`),
			flagOption("replace-ignore-case", "Match FROM of --replace in any case."),
		},
		build: buildReplace,
	}
}

func buildReplace(a *args, add func(edit)) error {
	if err := needs(a, "replace", "replace-regex", "replace-ignore-case"); err != nil {
		return err
	}
	regex := a.Has("replace-regex")
	var st steps
	for _, pair := range a.Values("replace") {
		from, to, ok := strings.Cut(pair, "=")
		if !ok || from == "" {
			return usage("The option --replace needs FROM=TO, got " + quote(pair) + ".")
		}
		opts := model.ReplaceOptions{Regex: regex, IgnoreCase: a.Has("replace-ignore-case")}
		if regex {
			p, hasGroup, err := regexReplacement(from)
			if err != nil {
				return err
			}
			from, to = p.pattern, convertReplacement(to, hasGroup)
			opts.IgnoreCase = opts.IgnoreCase || p.ignoreCase
		}
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ReplaceText(from, to, opts) }))
	}
	addSteps(add, st)
	return nil
}

// regexReplacement converts from and compiles it once to find a bad pattern before the first read. ReplaceText
// compiles it again for each input. hasGroup reports whether the pattern has the group number n.
func regexReplacement(from string) (p pcrePattern, hasGroup func(n int) bool, err error) {
	invalid := "The option --replace has the invalid regular expression " + quote(from) + "."
	p, badFlag, ok := convertPCRE(from)
	switch {
	case badFlag != "":
		return p, nil, usage(invalid + " The flag " + badFlag + " is not supported.")
	case !ok:
		return p, nil, usage(invalid)
	}
	options := regexp2.None
	if p.ignoreCase {
		options = regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(p.pattern, options, regexp2.OptionMaintainCaptureOrder())
	if err != nil {
		return p, nil, usage(invalid)
	}
	groups := re.GetGroupNumbers()
	return p, func(n int) bool { return slices.Contains(groups, n) }, nil
}

func textGroup() group {
	return group{
		name:    "text",
		summary: "Change the case, remove tags, fix right-to-left text.",
		options: []option{
			valueOption("case", "MODE", "Change the case of the text between tags: upper, lower or sentence."),
			flagOption("strip-tags", "Remove all formatting tags, such as <i> and <font>, from the cue text."),
			valueOption("rtl", "MODE", "Right-to-left text: fix wraps each Arabic or Hebrew line in Unicode embedding "+
				"marks, clean removes all bidi controls."),
		},
		build: buildText,
	}
}

func buildText(a *args, add func(edit)) error {
	mode, hasCase, err := a.Choice("case", string(model.CaseUpper), string(model.CaseLower), string(model.CaseSentence))
	if err != nil {
		return err
	}
	rtl, _, err := a.Choice("rtl", "fix", "clean")
	if err != nil {
		return err
	}
	var st steps
	if hasCase {
		language, _ := a.Value("language")
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ChangeCase(model.CaseMode(mode), language) }))
	}
	if a.Has("strip-tags") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil }))
	}
	switch rtl {
	case "fix":
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.FixRightToLeft(); return nil }))
	case "clean":
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.RemoveBidiControls(); return nil }))
	}
	addSteps(add, st)
	return nil
}

func structureGroup() group {
	return group{
		name:    "structure",
		summary: "Join, wrap and unwrap cues.",
		options: []option{
			flagOption("structure-unwrap",
				"Join the lines of each cue with a space. Each dialogue dash line starts a line of its own."),
			flagOption("structure-merge-short", "Join cues shorter than "+timecode.Seconds(model.DefaultMinDuration)+
				" s with a neighbor at most "+timecode.Seconds(model.DefaultMergeMaxGap)+" s away. The joined cue must fit "+
				timecode.Seconds(model.DefaultMaxDuration)+" s, --structure-max-cpl and --structure-max-lines."),
			flagOption("structure-wrap", "Break lines longer than --structure-max-cpl characters."),
			valueOption("structure-max-cpl", "CHARS", "Maximum characters per line for --structure-wrap and "+
				"--structure-merge-short. Default: "+strconv.Itoa(model.DefaultMaxCharactersPerLine)+"."),
			valueOption("structure-max-lines", "LINES", "Maximum number of lines per cue for --structure-wrap and "+
				"--structure-merge-short. Default: "+strconv.Itoa(model.DefaultMaxLinesPerCue)+"."),
			flagOption("structure-merge-duplicates", "Join same-text cues that overlap or touch."),
			flagOption("structure-merge-same-time",
				"Join cues with the same start and end into one cue with the lines of all of them."),
		},
		build: buildStructure,
	}
}

func buildStructure(a *args, add func(edit)) error {
	limited := []string{"structure-wrap", "structure-merge-short"}
	for _, opt := range []string{"structure-max-cpl", "structure-max-lines"} {
		if err := needsOneOf(a, limited, opt); err != nil {
			return err
		}
	}
	cpl, _, err := a.PositiveInt("structure-max-cpl")
	if err != nil {
		return err
	}
	lines, _, err := a.PositiveInt("structure-max-lines")
	if err != nil {
		return err
	}
	// 0 for an option that the call does not set makes the library default apply.
	limits := model.CueLimits{MaxCharactersPerLine: int(cpl), MaxLinesPerCue: int(lines)}
	wrapCPL := cmp.Or(int(cpl), model.DefaultMaxCharactersPerLine)
	wrapLines := cmp.Or(int(lines), model.DefaultMaxLinesPerCue)
	// The run order of docs/cli.md, which is not the order of the help.
	addSteps(add, flagSteps(a, []flagStep{
		{"structure-unwrap", func(s *stbox.Subtitle) error { s.UnwrapLines(); return nil }},
		{"structure-merge-same-time", func(s *stbox.Subtitle) error { return s.MergeSameTimeCues(0) }},
		{"structure-merge-short", func(s *stbox.Subtitle) error {
			return s.MergeShortCues(model.MergeShortCuesOptions{Limits: limits})
		}},
		{"structure-wrap", func(s *stbox.Subtitle) error { return s.WrapLines(wrapCPL, wrapLines) }},
		{"structure-merge-duplicates", func(s *stbox.Subtitle) error { return s.RemoveDuplicateCues(0) }},
	}))
	return nil
}

func retimeGroup() group {
	return group{
		name:    "retime",
		summary: "Shift and scale the times, or change the frame rate.",
		options: []option{
			valueOption("shift", "SECONDS",
				"Time to add to every time, for example 2.5 or 00:00:02.500, or -2.5 to show the cues earlier."),
			valueOption("shift-after", "SECONDS", "Shift only the cues that start at this time or later."),
			valueOption("shift-before", "SECONDS", "Shift only the cues that start before this time."),
			valueOption("scale", "FACTOR", "Multiply every time by this factor. Must be greater than 0."),
			{
				name: "sync", value: "OLD=NEW", repeatable: true,
				help: "Move the time OLD to NEW and correct the times between. OLD is a time, first, last or #N, " +
					"the start of cue N. Repeatable.",
			},
			valueOption("from-fps", "RATE", "Frame rate of the video that the subtitle fits now. Needs --to-fps."),
			valueOption("to-fps", "RATE", "Frame rate of the video that the subtitle must fit. Needs --from-fps."),
		},
		build: buildRetime,
	}
}

// retimeValues holds the values of the retime options. A nil pointer is an option that the call does not set.
type retimeValues struct {
	shift, shiftAfter, shiftBefore *time.Duration
	scale, fromFPS, toFPS          *float64
	sync                           []syncPoint
}

func buildRetime(a *args, add func(edit)) error {
	if err := needs(a, "shift", "shift-after", "shift-before"); err != nil {
		return err
	}
	if a.Has("sync") && (a.Has("shift") || a.Has("scale")) {
		return usage("Pass --sync without --shift and --scale.")
	}
	v, err := readRetime(a)
	if err != nil {
		return err
	}
	if v.shiftAfter != nil && v.shiftBefore != nil && *v.shiftBefore <= *v.shiftAfter {
		return usage("The option --shift-before must be after --shift-after.")
	}
	if (v.fromFPS == nil) != (v.toFPS == nil) {
		return usage("Pass --from-fps and --to-fps together.")
	}
	addSteps(add, v.steps())
	return nil
}

// readRetime reads the values in the check order of docs/cli.md: the times, the numbers, then the sync points.
func readRetime(a *args) (retimeValues, error) {
	var v retimeValues
	for _, t := range []struct {
		name string
		to   **time.Duration
	}{{"shift", &v.shift}, {"shift-after", &v.shiftAfter}, {"shift-before", &v.shiftBefore}} {
		d, ok, err := a.Time(t.name)
		if err != nil {
			return v, err
		}
		if ok {
			*t.to = &d
		}
	}
	for _, n := range []struct {
		name string
		to   **float64
	}{{"scale", &v.scale}, {"from-fps", &v.fromFPS}, {"to-fps", &v.toFPS}} {
		f, ok, err := a.PositiveFloat(n.name)
		if err != nil {
			return v, err
		}
		if ok {
			*n.to = &f
		}
	}
	var err error
	v.sync, err = syncPoints(a.Values("sync"))
	return v, err
}

// steps returns the calls in the run order: sync, shift, scale, frame rate.
func (v retimeValues) steps() steps {
	var st steps
	if v.sync != nil {
		st = append(st, inPlace(func(s *stbox.Subtitle) error {
			points, err := resolveSync(s, v.sync)
			if err != nil {
				return err
			}
			return s.SyncByPoints(points)
		}))
	}
	if v.shift != nil {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ShiftRange(*v.shift, v.shiftAfter, v.shiftBefore) }))
	}
	if v.scale != nil {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.Scale(*v.scale) }))
	}
	if v.fromFPS != nil {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ConvertFrameRate(*v.fromFPS, *v.toFPS) }))
	}
	return st
}

// syncPoint is one --sync value. The OLD side is first, last, a cue number or a time.
type syncPoint struct {
	text string
	// first and last name the start of the first or the last cue.
	first, last bool
	// cue is the 1-based number of the cue whose start OLD is, or 0.
	cue int
	// old is the OLD time when OLD is a time.
	old time.Duration
	new time.Duration
}

func (p syncPoint) hasTime() bool { return !p.first && !p.last && p.cue == 0 }

// cueNumber is #N of a --sync OLD. 9 digits keep N far below the int limit.
var cueNumber = regexp.MustCompile(`^#0*([1-9][0-9]{0,8})$`)

func syncPoints(values []string) ([]syncPoint, error) {
	var points []syncPoint
	for _, v := range values {
		p, err := parseSyncPoint(v)
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, checkSyncOrder(points)
}

func parseSyncPoint(v string) (syncPoint, error) {
	old, newTime, ok := strings.Cut(v, "=")
	if !ok {
		return syncPoint{}, usage("The option --sync needs OLD=NEW, got " + quote(v) + ".")
	}
	p := syncPoint{text: v, first: old == "first", last: old == "last"}
	var err error
	switch {
	case p.first || p.last:
	case strings.HasPrefix(old, "#"):
		m := cueNumber.FindStringSubmatch(old)
		if m == nil {
			return syncPoint{}, usage("The option --sync needs a cue number from 1 after #, got " + quote(v) + ".")
		}
		p.cue, _ = strconv.Atoi(m[1])
	default:
		if p.old, err = parseTime("sync", old); err != nil {
			return syncPoint{}, err
		}
	}
	p.new, err = parseTime("sync", newTime)
	return p, err
}

// checkSyncOrder fails when 2 points with an OLD time do not increase in both times.
func checkSyncOrder(points []syncPoint) error {
	var previous *syncPoint
	for i := range points {
		p := &points[i]
		if !p.hasTime() {
			continue
		}
		if previous != nil && (p.old <= previous.old || p.new <= previous.new) {
			return usage("The --sync points must increase in both times, got " + quote(previous.text) + " before " +
				quote(p.text) + ".")
		}
		previous = p
	}
	return nil
}

// resolveSync turns first, last and #N into the start times of the cues of s. Its errors fail this input only.
func resolveSync(s *stbox.Subtitle, points []syncPoint) ([]model.SyncPoint, error) {
	n := s.Len()
	if n == 0 {
		return nil, usage("The --sync points need a subtitle with cues.")
	}
	resolved := make([]syncPoint, len(points))
	for i, p := range points {
		index := -1
		switch {
		case p.first:
			index = 0
		case p.last:
			index = n - 1
		case p.cue > n:
			return nil, usage("The --sync point " + quote(p.text) + " names cue " + strconv.Itoa(p.cue) +
				", but the subtitle has " + strconv.Itoa(n) + " cues.")
		case p.cue > 0:
			index = p.cue - 1
		}
		if index >= 0 {
			c, _ := s.Cue(index)
			p.old = c.Start
		}
		resolved[i] = syncPoint{text: p.text, old: p.old, new: p.new}
	}
	if err := checkSyncOrder(resolved); err != nil {
		return nil, err
	}
	out := make([]model.SyncPoint, len(resolved))
	for i, p := range resolved {
		out[i] = model.SyncPoint{Old: p.old, New: p.new}
	}
	return out, nil
}

func timingGroup() group {
	return group{
		name:    "timing",
		summary: "Fix overlaps, short cues and long cues, and add lead-in and lead-out.",
		options: []option{
			flagOption("timing-fix-overlaps", "End each cue at least --timing-min-gap seconds before the next cue starts."),
			valueOption("timing-min-duration", "SECONDS", "Show each cue for at least this time where the next cue allows it."),
			valueOption("timing-lead-in", "SECONDS", "Start each cue this time earlier where the previous cue allows it."),
			valueOption("timing-lead-out", "SECONDS",
				"End each cue this time later where the next cue allows it. The lead-out comes before the lead-in."),
			valueOption("timing-max-duration", "SECONDS",
				"End each cue at most this time after its start. Runs after the other timing fixes."),
			valueOption("timing-min-gap", "SECONDS", "Gap between cues for --timing-fix-overlaps, "+
				"--timing-min-duration and the lead options. Default: "+timecode.Seconds(defaultMinGap)+"."),
		},
		build: buildTiming,
	}
}

const defaultMinGap time.Duration = 0

func buildTiming(a *args, add func(edit)) error {
	gapUsers := []string{"timing-fix-overlaps", "timing-min-duration", "timing-lead-in", "timing-lead-out"}
	if err := needsOneOf(a, gapUsers, "timing-min-gap"); err != nil {
		return err
	}
	var minDuration, leadIn, leadOut, maxDuration time.Duration
	minGap := defaultMinGap
	for _, v := range []struct {
		get  func(name string) (time.Duration, bool, error)
		name string
		to   *time.Duration
	}{
		{a.PositiveTime, "timing-min-duration", &minDuration},
		{a.NonNegativeTime, "timing-min-gap", &minGap},
		{a.NonNegativeTime, "timing-lead-in", &leadIn},
		{a.NonNegativeTime, "timing-lead-out", &leadOut},
		{a.PositiveTime, "timing-max-duration", &maxDuration},
	} {
		d, ok, err := v.get(v.name)
		if err != nil {
			return err
		}
		if ok {
			*v.to = d
		}
	}
	st := flagSteps(a, []flagStep{
		{"timing-fix-overlaps", func(s *stbox.Subtitle) error { return s.FixOverlaps(minGap) }},
		{"timing-min-duration", func(s *stbox.Subtitle) error { return s.ExtendShortCues(minDuration, minGap) }},
	})
	if a.Has("timing-lead-in") || a.Has("timing-lead-out") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.AddLeadInOut(leadIn, leadOut, minGap) }))
	}
	if a.Has("timing-max-duration") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.LimitLongCues(maxDuration) }))
	}
	addSteps(add, st)
	return nil
}
