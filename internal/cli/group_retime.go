package cli

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

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
	scale                          *float64
	fromFPS, toFPS                 *stbox.FrameRate
	sync                           []syncPoint
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildRetime(a *args) (edit, error) {
	if err := needs(a, "shift", "shift-after", "shift-before"); err != nil {
		return nil, err
	}
	if a.Has("sync") && (a.Has("shift") || a.Has("scale")) {
		return nil, usage("Pass --sync without --shift and --scale.")
	}
	v, err := readRetime(a)
	if err != nil {
		return nil, err
	}
	if v.shiftAfter != nil && v.shiftBefore != nil && *v.shiftBefore <= *v.shiftAfter {
		return nil, usage("The option --shift-before must be after --shift-after.")
	}
	if (v.fromFPS == nil) != (v.toFPS == nil) {
		return nil, usage("Pass --from-fps and --to-fps together.")
	}
	return stepsEdit(v.steps()), nil
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
	scale, ok, err := a.PositiveFloat("scale")
	if err != nil {
		return v, err
	}
	if ok {
		v.scale = &scale
	}
	for _, n := range []struct {
		name string
		to   **stbox.FrameRate
	}{{"from-fps", &v.fromFPS}, {"to-fps", &v.toFPS}} {
		r, ok, err := a.FrameRate(n.name)
		if err != nil {
			return v, err
		}
		if ok {
			*n.to = &r
		}
	}
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
