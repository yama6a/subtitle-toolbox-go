package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/hls"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

const defaultPlaylist = "subs.m3u8"

// hlsCommand cuts a subtitle into WebVTT segments and writes the playlist of the segments.
type hlsCommand struct {
	dir      string
	playlist string
	opts     hls.SegmentOptions
}

var _ fileCommand = &hlsCommand{}

func (*hlsCommand) name() string { return "hls" }

func (*hlsCommand) summary() string {
	return "Cut a subtitle into WebVTT segments and write an HLS playlist for them."
}

func (*hlsCommand) usage() []string {
	return []string{"<input> --output-dir DIR [--segment SECONDS] [options]"}
}

func (*hlsCommand) details() string {
	return "Each segment starts with an X-TIMESTAMP-MAP header and holds every cue that overlaps it, with its full times. " +
		"The playlist is a VOD media playlist."
}

func (*hlsCommand) options() []option {
	return slices.Concat([]option{
		valueOption("output-dir", "DIR", "Directory for the segments and the playlist. Creates it when it is missing. "+
			"No playlist or segment file may exist."),
		valueOption("segment", "SECONDS", "Duration of a segment. Default: "+formatNumber(hls.DefaultSegmentDuration.Seconds())+"."),
		valueOption("playlist", "NAME", "File name of the playlist. Default: "+defaultPlaylist+"."),
		valueOption("pattern", "PATTERN", "File name of a segment, with %d for its number from 0. Default: "+
			hls.DefaultFileNamePattern+"."),
		valueOption("mpegts", "TICKS", "90 kHz MPEG-2 timestamp at which subtitle time 0 plays. Default: "+
			strconv.Itoa(hls.DefaultMPEGTS)+"."),
		valueOption("local", "SECONDS", "WebVTT cue time that maps to --mpegts. Default: 0."),
		valueOption("media-duration", "SECONDS", "Duration of the video, so the playlist covers it all. "+
			"Default: the end of the last cue."),
	}, inputOptions(false, reportFPSHelp))
}

func (h *hlsCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, h)
}

func (h *hlsCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	dir, ok := a.Value("output-dir")
	if !ok || dir == "" {
		return usage("Pass --output-dir DIR.")
	}
	if target := r.c.realTarget(dir); exists(target) && !isDir(target) {
		return usage("The --output-dir " + dir + " is a file. Pass a directory.")
	}
	h.dir = strings.TrimRight(dir, `/\`)
	h.playlist = defaultPlaylist
	if v, ok := a.Value("playlist"); ok {
		h.playlist = v
	}
	if err := h.prepareOptions(a); err != nil {
		return err
	}
	return h.opts.Validate()
}

func (h *hlsCommand) prepareOptions(a *args) error {
	var err error
	if h.opts.SegmentDuration, _, err = a.PositiveTime("segment"); err != nil {
		return err
	}
	m := hls.TimestampMap{MPEGTS: hls.DefaultMPEGTS}
	if ticks, ok, err := a.Int("mpegts", 0); err != nil {
		return err
	} else if ok {
		m.MPEGTS = ticks
	}
	if m.Local, _, err = a.NonNegativeTime("local"); err != nil {
		return err
	}
	h.opts.Map = &m
	h.opts.FileNamePattern, _ = a.Value("pattern")
	h.opts.MediaDuration, _, err = a.PositiveTime("media-duration")
	return err
}

// checkInputs runs before the read, so it creates no file.
func (h *hlsCommand) checkInputs(r *runner, inputs []string) error {
	if len(inputs) != 1 {
		return usage("The hls command takes one input file, got " + strconv.Itoa(len(inputs)) + ".")
	}
	playlist := r.c.realTarget(h.dir + "/" + h.playlist)
	if h.isSegment(r.c, playlist) {
		return usage("The playlist " + h.playlist + " has the name of a segment. Pass another --playlist or --pattern.")
	}
	if inputs[0] != stdinArg {
		if realPath, err := filepath.EvalSymlinks(r.c.path(inputs[0])); err == nil {
			abs, _ := filepath.Abs(realPath)
			if abs == playlist || h.isSegment(r.c, abs) {
				return usage("The output would overwrite the input " + inputs[0] +
					". Pass another --output-dir, --playlist or --pattern.")
			}
		}
	}

	if first := h.firstExisting(r.c, playlist); first != "" {
		return usage(first + " exists. The tool never overwrites a file. " +
			"Remove the playlist and the segments, or pass another --output-dir.")
	}
	return nil
}

// firstExisting returns the path of the first output that exists: the playlist, else a file of the segment folder
// that matches the pattern. Only the read tells the segment count, so any matching file counts as a segment.
func (h *hlsCommand) firstExisting(c *console, playlist string) string {
	if exists(playlist) {
		return h.dir + "/" + h.playlist
	}
	dir, _ := h.segmentDirectory()
	realDir := c.realTarget(dir)
	entries, err := os.ReadDir(realDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if h.isSegment(c, c.realTarget(realDir+"/"+e.Name())) {
			return dir + "/" + e.Name()
		}
	}
	return ""
}

// isSegment reports whether realPath is a file that a segment of this run could have, for any segment number.
func (h *hlsCommand) isSegment(c *console, realPath string) bool {
	dir, pattern := h.segmentDirectory()
	prefix := c.realTarget(dir) + string(filepath.Separator)
	name, ok := strings.CutPrefix(realPath, prefix)
	return ok && matchesPattern(pattern, name)
}

// segmentDirectory returns the directory of the segments and the pattern of a segment path in it. The directory
// is --output-dir plus the part of --pattern up to the last slash before the first verb, so "../seg%d.vtt" gives
// "DIR/..".
func (h *hlsCommand) segmentDirectory() (dir, pattern string) {
	pattern = h.opts.FileNamePattern
	if pattern == "" {
		pattern = hls.DefaultFileNamePattern
	}
	verb := strings.Index(strings.ReplaceAll(pattern, "%%", "  "), "%")
	slash := strings.LastIndex(pattern[:verb], "/")
	if slash < 0 {
		return h.dir, pattern
	}
	return h.dir + "/" + pattern[:slash], pattern[slash+1:]
}

// matchesPattern reports whether name is the pattern printed with a number of 0 or more. So sub01.vtt does not
// match sub%d.vtt, because that pattern prints 1 as sub1.vtt.
func matchesPattern(pattern, name string) bool {
	verb := strings.Index(strings.ReplaceAll(pattern, "%%", "  "), "%")
	end := verb + strings.Index(pattern[verb:], "d") + 1
	prefix := strings.ReplaceAll(pattern[:verb], "%%", "%")
	suffix := strings.ReplaceAll(pattern[end:], "%%", "%")
	digits, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return false
	}
	digits, ok = strings.CutSuffix(digits, suffix)
	// sscanf of PHP skips the spaces that a width such as %3d pads with. The comparison below rejects other padding.
	if digits = strings.TrimLeft(digits, " "); !ok || !timecode.IsDigits(digits) {
		return false
	}
	n, err := strconv.Atoi(digits)
	return err == nil && fmt.Sprintf(pattern, n) == name
}

func (*hlsCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (h *hlsCommand) process(_ context.Context, r *runner, in *input) error {
	rendition, err := hls.Segment(in.sub, h.opts)
	if err != nil {
		return err
	}
	err = rendition.EachSegment(func(name string, content []byte) error {
		return r.files.create(h.dir+"/"+name, content)
	})
	if err != nil {
		return err
	}
	path := h.dir + "/" + h.playlist
	if err := r.files.create(path, rendition.Playlist()); err != nil {
		return err
	}
	r.report(in.label() + " -> " + path + ", " + strconv.Itoa(rendition.Count()) + " segments\n")
	return nil
}

func (*hlsCommand) finish(*runner) error { return nil }
