package cli

import (
	"bytes"
	"context"
	"strconv"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/timing"
)

// snapTriggers lists the options that turn the snap group on, in the order that the rate message names them.
//
//nolint:gochecknoglobals // the trigger names of the snap group, in message order
var snapTriggers = []string{
	"snap-shot-changes", "snap-window-frames", "snap-min-gap-frames", "snap-min-duration-frames", "no-snap-chain",
}

func snapGroup() group {
	return group{
		name:    "snap",
		summary: "Time cues to shot changes and close gaps shorter than --snap-window-frames.",
		options: []option{
			valueOption("snap-shot-changes", "FILE", "Time cues to the shot changes in this file. It holds one time per line "+
				"in seconds or hh:mm:ss.mmm, or the log of the FFmpeg showinfo filter."),
			valueOption("video-fps", "RATE", "Frame rate of the video, for the shot changes and the other snap options. "+
				"Required with them."),
			valueOption("snap-window-frames", "FRAMES", "Largest move to a shot change, and largest gap that closes. "+
				"Default: half the --video-fps, rounded to the nearest frame. A half frame rounds down. "+
				"12 at 24 and 25 fps, 15 at 30 fps."),
			valueOption("snap-min-gap-frames", "FRAMES", "Minimum gap between a cue and the next cue or shot change. "+
				"Default: "+strconv.Itoa(timing.DefaultMinGapFrames)+"."),
			valueOption("snap-min-duration-frames", "FRAMES", "No move makes a cue shorter than this. "+
				"Default: "+strconv.Itoa(timing.DefaultMinDurationFrames)+"."),
			flagOption("no-snap-chain", "Do not close gaps shorter than --snap-window-frames."),
		},
		build: buildSnap,
	}
}

// buildSnap runs the checks of docs/cli.md in order: the rate, the --video-fps rule, the rate rule, the
// --no-snap-chain rule, the frame counts, then Options.Validate.
func buildSnap(a *args, add func(edit)) error {
	rate, active, err := snapRate(a)
	if err != nil || !active {
		return err
	}
	e := &snapEdit{opts: timing.Options{FrameRate: rate, KeepGaps: a.Has("no-snap-chain")}}
	e.path, _ = a.Value("snap-shot-changes")
	for _, c := range []struct {
		name string
		to   **int
	}{
		{"snap-window-frames", &e.opts.SnapWindowFrames},
		{"snap-min-gap-frames", &e.opts.MinGapFrames},
		{"snap-min-duration-frames", &e.opts.MinDurationFrames},
	} {
		n, ok, err := a.Int(c.name, 0)
		if err != nil {
			return err
		}
		if ok {
			v := int(n)
			*c.to = &v
		}
	}
	if err := e.opts.Validate(); err != nil {
		return usage(message(err))
	}
	add(e)
	return nil
}

// snapRate returns the rate of the video and whether a trigger option turns the group on. It fails for a bad
// rate and for the 3 rate and chain rules that come before the frame counts.
func snapRate(a *args) (rate stbox.FrameRate, active bool, err error) {
	rate, hasRate, err := a.Rate("video-fps")
	if err != nil {
		return rate, false, err
	}
	first := ""
	for _, name := range snapTriggers {
		if a.Has(name) {
			first = name
			break
		}
	}
	switch {
	case first == "" && a.Has("video-fps"):
		return rate, false, usage("Pass --snap-shot-changes FILE with --video-fps.")
	case first == "":
		return rate, false, nil
	case !hasRate:
		return rate, false, usage("Pass --video-fps RATE or --fps RATE with --" + first + ".")
	case a.Has("no-snap-chain") && !a.Has("snap-shot-changes"):
		return rate, false, usage("Pass --snap-shot-changes FILE. With --no-snap-chain and no shot changes, snapping changes nothing.")
	}
	return rate, true, nil
}

// snapEdit runs timing.Apply with the shot changes of its file, which loadShots reads once per run.
type snapEdit struct {
	// path is the --snap-shot-changes file, "" without it.
	path string
	opts timing.Options
}

var _ edit = &snapEdit{}

func (e *snapEdit) apply(_ context.Context, s *stbox.Subtitle, _ *console, _ string) (*stbox.Subtitle, error) {
	_, err := timing.Apply(s, e.opts)
	return s, err //nolint:wrapcheck // the *InvalidArgumentError carries the message that the CLI prints
}

// loadShots reads the shot change file. Content with pts_time: is an FFmpeg log, other content is one time per line.
func (e *snapEdit) loadShots(c *console) error {
	if e.path == "" {
		return nil
	}
	content, err := readSideText(c, e.path)
	if err != nil {
		return err
	}
	read := timing.ShotChangesFromText
	if bytes.Contains(content, []byte("pts_time:")) {
		read = timing.ShotChangesFromFFmpegLog
	}
	var shots []time.Duration
	if shots, err = read(content); err != nil {
		return &fileError{msg: e.path + ": " + message(err)}
	}
	e.opts.ShotChanges = shots
	return nil
}
