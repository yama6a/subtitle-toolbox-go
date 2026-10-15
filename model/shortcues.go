package model

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

// The defaults of CueLimits and MergeShortCuesOptions. The line, lines and maximum duration defaults are the
// Netflix limits for English.
const (
	// DefaultMaxCharactersPerLine is the default of CueLimits.MaxCharactersPerLine.
	DefaultMaxCharactersPerLine = 42
	// DefaultMaxLinesPerCue is the default of CueLimits.MaxLinesPerCue.
	DefaultMaxLinesPerCue = 2
	// DefaultMinDuration is the default of CueLimits.MinDuration.
	DefaultMinDuration = time.Second
	// DefaultMaxDuration is the default of CueLimits.MaxDuration.
	DefaultMaxDuration = 7 * time.Second
	// DefaultMergeMaxGap is the default of MergeShortCuesOptions.MaxGap.
	DefaultMergeMaxGap = 250 * time.Millisecond
)

var (
	voiceTagStartRegexp = regexp.MustCompile(`(?i)^` + markupcore.VoiceTagPattern)
	voiceTagsRegexp     = regexp.MustCompile(`(?i)` + markupcore.VoiceTagPattern + `|</v>`)
	voiceTagEndRegexp   = regexp.MustCompile(`(?i)</v>$`)
)

// CueLimits holds the size and time limits of one cue.
type CueLimits struct {
	// MaxCharactersPerLine is the most visible characters of one line. 0 means DefaultMaxCharactersPerLine.
	MaxCharactersPerLine int
	// MaxLinesPerCue is the most lines of one cue. 0 means DefaultMaxLinesPerCue.
	MaxLinesPerCue int
	// MinDuration is the shortest duration of a cue. Nil means DefaultMinDuration. 0 makes no cue short by its duration.
	MinDuration *time.Duration
	// MaxDuration is the longest duration of a cue. 0 means DefaultMaxDuration.
	MaxDuration time.Duration
	// MaxCharactersPerSecond is the most visible characters of all lines per second. 0 means no limit.
	MaxCharactersPerSecond float64
}

// Validate returns an error that matches ErrInvalidArgument when a limit is out of range after the defaults apply.
func (l CueLimits) Validate() error {
	_, err := l.resolved()
	return err
}

// resolved returns l with the defaults in place of the zero values and the durations rounded to whole
// milliseconds, after it checks the given values.
func (l CueLimits) resolved() (cueLimits, error) {
	r := cueLimits{
		maxCharactersPerLine:   cmp.Or(l.MaxCharactersPerLine, DefaultMaxCharactersPerLine),
		maxLinesPerCue:         cmp.Or(l.MaxLinesPerCue, DefaultMaxLinesPerCue),
		minDuration:            DefaultMinDuration,
		maxDuration:            cmp.Or(l.MaxDuration, DefaultMaxDuration),
		maxCharactersPerSecond: l.MaxCharactersPerSecond,
	}
	if l.MinDuration != nil {
		r.minDuration = *l.MinDuration
	}
	if err := r.check(); err != nil {
		return cueLimits{}, err
	}
	r.minDuration, r.maxDuration = r.minDuration.Round(time.Millisecond), r.maxDuration.Round(time.Millisecond)
	return r, nil
}

func (r cueLimits) check() error {
	if r.maxCharactersPerLine < 1 || r.maxLinesPerCue < 1 {
		option := "CueLimits.MaxCharactersPerLine"
		if r.maxCharactersPerLine >= 1 {
			option = "CueLimits.MaxLinesPerCue"
		}
		return &InvalidArgumentError{Option: option, Msg: fmt.Sprintf(
			"The maximum characters per line and the maximum lines must be at least 1, got %d and %d.",
			r.maxCharactersPerLine, r.maxLinesPerCue)}
	}
	if r.minDuration < 0 {
		return &InvalidArgumentError{Option: "CueLimits.MinDuration", Msg: fmt.Sprintf(
			"The minimum duration must not be negative, got %s.", timecode.Seconds(r.minDuration))}
	}
	if err := r.checkMaximums(); err != nil {
		return err
	}
	if r.minDuration > r.maxDuration {
		return &InvalidArgumentError{Option: "CueLimits.MinDuration", Msg: fmt.Sprintf(
			"The minimum duration must not be greater than the maximum duration, got %s and %s.",
			timecode.Seconds(r.minDuration), timecode.Seconds(r.maxDuration))}
	}
	return nil
}

func (r cueLimits) checkMaximums() error {
	cps := r.maxCharactersPerSecond
	if r.maxDuration >= 0 && cps >= 0 && !math.IsNaN(cps) && !math.IsInf(cps, 0) {
		return nil
	}
	option := "CueLimits.MaxDuration"
	if r.maxDuration > 0 {
		option = "CueLimits.MaxCharactersPerSecond"
	}
	cpsText := "no limit"
	if cps != 0 {
		cpsText = timecode.Number(cps)
	}
	return &InvalidArgumentError{Option: option, Msg: fmt.Sprintf(
		"The maximum duration and the maximum characters per second must be greater than 0, got %s and %s.",
		timecode.Seconds(r.maxDuration), cpsText)}
}

type cueLimits struct {
	maxCharactersPerLine   int
	maxLinesPerCue         int
	minDuration            time.Duration
	maxDuration            time.Duration
	maxCharactersPerSecond float64
}

// MergeShortCuesOptions holds the settings of MergeShortCues.
type MergeShortCuesOptions struct {
	// Limits holds the limits of a joined cue. A cue shorter than Limits.MinDuration is short.
	Limits CueLimits
	// MaxGap is the longest time from the end of one cue to the start of the next that a join bridges.
	// Nil means DefaultMergeMaxGap.
	MaxGap *time.Duration
	// MinCharacters makes a cue with fewer visible characters short too. 0 turns the rule off.
	MinCharacters int
	// KeepSentenceEnds stops a join when the first cue ends with ".", "?" or "!". False joins across sentence ends.
	KeepSentenceEnds bool
	// MergeSameSpeakerAnyDuration makes every cue a candidate, joins only cues of one <v> speaker, and ignores
	// Limits.MaxDuration. False joins short cues only.
	MergeSameSpeakerAnyDuration bool
}

// Validate returns an error that matches ErrInvalidArgument when a setting is out of range after the defaults apply.
func (o MergeShortCuesOptions) Validate() error {
	_, err := o.resolved()
	return err
}

func (o MergeShortCuesOptions) resolved() (shortCueMerge, error) {
	limits, err := o.Limits.resolved()
	if err != nil {
		var e *InvalidArgumentError
		if errors.As(err, &e) {
			e.Option = "MergeShortCuesOptions.Limits." + strings.TrimPrefix(e.Option, "CueLimits.")
		}
		return shortCueMerge{}, err
	}
	m := shortCueMerge{limits: limits, maxGap: DefaultMergeMaxGap, opts: o}
	if o.MaxGap != nil {
		m.maxGap = *o.MaxGap
	}
	if m.maxGap < 0 {
		return m, &InvalidArgumentError{Option: "MergeShortCuesOptions.MaxGap", Msg: fmt.Sprintf(
			"The maximum gap must not be negative, got %s.", timecode.Seconds(m.maxGap))}
	}
	if o.MinCharacters < 0 {
		return m, &InvalidArgumentError{Option: "MergeShortCuesOptions.MinCharacters", Msg: fmt.Sprintf(
			"The minimum characters must not be negative, got %d.", o.MinCharacters)}
	}
	m.maxGap = m.maxGap.Round(time.Millisecond)
	return m, nil
}

type shortCueMerge struct {
	limits cueLimits
	maxGap time.Duration
	opts   MergeShortCuesOptions
}

// MergeShortCues joins each short cue with the next cue, or else with the previous cue, when the joined cue fits
// the limits of opts. A cue is short when it lasts less than opts.Limits.MinDuration or has fewer visible
// characters than opts.MinCharacters. A joined cue that is still short joins again. The joined cue keeps the
// start, the ID, the alignment, the forced flag and the format data of the first cue, ends at the later end, and
// gets the lines wrapped to the limits. Each comment before a joined cue moves before the result. The checks use
// the durations of opts rounded to whole milliseconds. MergeShortCues returns an error that matches
// ErrInvalidArgument when opts.Validate fails.
func (s *Subtitle) MergeShortCues(opts MergeShortCuesOptions) error {
	m, err := opts.resolved()
	if err != nil {
		return err
	}
	n := len(s.cues)
	if n == 0 {
		return nil
	}
	out, firstOld := m.merge(s.cues)
	if len(out) == n {
		return nil
	}

	newIndex := make([]int, n)
	for k, first := range firstOld {
		end := n
		if k+1 < len(firstOld) {
			end = firstOld[k+1]
		}
		for old := first; old < end; old++ {
			newIndex[old] = k
		}
	}
	anchors := s.commentAnchors()
	s.cues = out
	s.index = nil
	s.rebuildComments(anchors, func(old int) int { return newIndex[old] })
	return nil
}

// merge returns the cues after the joins, and for each the index in cues of the first cue joined into it.
// Out holds the cues before the current cue, so a join with the previous cue takes the last cue of out.
func (m shortCueMerge) merge(cues []Cue) (out []Cue, firstOld []int) {
	out, firstOld = make([]Cue, 0, len(cues)), make([]int, 0, len(cues))
	cur, curFirst, next := cues[0], 0, 1
	for {
		candidate := m.opts.MergeSameSpeakerAnyDuration || m.isShort(cur)
		if candidate && next < len(cues) {
			if joined, ok := m.join(cur, cues[next]); ok {
				cur = joined
				next++
				continue
			}
		}
		if last := len(out) - 1; candidate && last >= 0 {
			if joined, ok := m.join(out[last], cur); ok {
				cur, curFirst = joined, firstOld[last]
				out, firstOld = out[:last], firstOld[:last]
				continue
			}
		}
		out, firstOld = append(out, cur), append(firstOld, curFirst)
		if next == len(cues) {
			return out, firstOld
		}
		cur, curFirst = cues[next], next
		next++
	}
}

func (m shortCueMerge) isShort(c Cue) bool {
	return c.End-c.Start < m.limits.minDuration ||
		(m.opts.MinCharacters > 0 && linewrap.VisibleCharacters(c.Lines) < m.opts.MinCharacters)
}

// join returns first with the later end and the lines of both cues, or false when the joined cue breaks a limit.
func (m shortCueMerge) join(first, second Cue) (Cue, bool) {
	names := speakers(first)
	if !m.mayJoin(first, second, names) {
		return Cue{}, false
	}
	duration := max(first.End, second.End) - first.Start
	if second.Start-first.End > m.maxGap || (!m.opts.MergeSameSpeakerAnyDuration && duration > m.limits.maxDuration) {
		return Cue{}, false
	}

	lines, ok := oneVoiceTag(first, second, names)
	if !ok {
		lines = slices.Concat(first.Lines, second.Lines)
	}
	lines, ok = linewrap.WrapToFit(lines, m.limits.maxCharactersPerLine, m.limits.maxLinesPerCue)
	if !ok || !linewrap.FitsCharactersPerSecond(lines, duration, m.limits.maxCharactersPerSecond) {
		return Cue{}, false
	}
	first.End = max(first.End, second.End)
	first.Lines = cleanLines(lines)
	return first, true
}

// mayJoin applies the join check of JoinCues, the speaker rule and the sentence end rule.
func (m shortCueMerge) mayJoin(first, second Cue, names []string) bool {
	return canJoin(first, second) &&
		(!m.opts.MergeSameSpeakerAnyDuration || len(names) > 0) &&
		(!m.opts.KeepSentenceEnds || !endsWithStop(first))
}

// oneVoiceTag returns the lines of both cues under the opening <v> tag of first, with a closing tag when the text
// of second ends with one. Both cues must start with a <v> tag, and names must hold 1 speaker.
func oneVoiceTag(first, second Cue, names []string) ([]string, bool) {
	if len(names) != 1 {
		return nil, false
	}
	tag := voiceTagStartRegexp.FindString(first.Text())
	if tag == "" || !voiceTagStartRegexp.MatchString(second.Text()) {
		return nil, false
	}
	lines := slices.Concat(first.Lines, second.Lines)
	for i, l := range lines {
		lines[i] = voiceTagsRegexp.ReplaceAllString(l, "")
	}
	lines[0] = tag + lines[0]
	if voiceTagEndRegexp.MatchString(second.Text()) {
		lines[len(lines)-1] += "</v>"
	}
	return lines, true
}

func endsWithStop(c Cue) bool {
	text := strings.TrimRight(markupcore.PlainText(c.Text()), phpTrimSet)
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!")
}
