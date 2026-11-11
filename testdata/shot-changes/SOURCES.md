# Sources

| File | Source | License |
|:--- |:--- |:--- |
| `own_garden_24fps.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: BOM, CR LF, italic tags, one overlap and gaps of 7 frames at 24 fps. | MIT |
| `own_garden_24fps_timed.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_garden_24fps.srt` after `ShotChangeTiming::apply()` at 24 fps with the shot changes of `own_ffmpeg_showinfo.log`. | MIT |
| `own_ffmpeg_showinfo.log` | Written for this repository in the shape of the log of `ffmpeg -i in.mp4 -vf "select='gt(scene,0.3)',showinfo" -f null -` from ffmpeg 6.1: banner, stream info, one showinfo line per selected frame and the progress line. | MIT |
| `own_scenes.txt` | Written for this repository. The shot changes of `own_ffmpeg_showinfo.log`, one per line, in seconds and as hh:mm:ss.mmm. | MIT |
| `own_garden_24fps_window_6.srt`, `own_garden_24fps_min_gap_3.srt`, `own_garden_24fps_min_duration_12.srt`, `own_garden_24fps_no_chain.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_garden_24fps.srt --to srt --video-fps 24 --snap-shot-changes own_scenes.txt` with one of `--snap-window-frames 6`, `--snap-min-gap-frames 3`, `--snap-min-duration-frames 12` and `--no-snap-chain`, checked by hand against `own_garden_24fps_timed.srt` | MIT |
