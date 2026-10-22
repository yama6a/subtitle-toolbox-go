# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/ass/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_aegisub.ass` | Written for this repository in the shape of an Aegisub 3.2 file: UTF-8 BOM, LF, `[Aegisub Project Garbage]`, three styles, `Comment:` events with a karaoke template, `\k`, `\kf` and `\ko` karaoke, override tags and `\N`. | MIT |
| `own_comment_before_earlier_event.ass` | Written for this repository. `Dialogue:` events out of time order, and a `Comment:` event before an event that starts earlier than the event before it. | MIT |
| `own_ffmpeg.ass` | Written for this repository in the shape of the header that FFmpeg writes when it converts SubRip to ASS: no BOM, LF, short colours such as `&Hffffff`, `{\c&H0000ff&}` and `{\c}`. | MIT |
| `own_signs_crlf.ass` | Written for this repository in the shape of an Aegisub file saved on Windows: UTF-8 BOM, CR LF, events out of time order, `\pos`, `\fad`, `\t`, a `\p1` drawing, a `{...}` note, `\n`, `\h`, `\r`, and `[Fonts]` and `[Graphics]` sections. The embedded data is placeholder text, not a font or an image. | MIT |
| `own_ssa_v4.ssa` | Written for this repository in the shape of a SubStation Alpha v4.00 file: CR LF, `[V4 Styles]` with decimal colours, `Marked=` columns, four-digit margins and a `{\a6}` legacy alignment tag. | MIT |
| `own_style_flags.ass` | Written for this repository in the shape of an Aegisub file with named styles: an italic `Thoughts` style at the top, a bold, underlined and struck-out `Sign` style at the top left, an event with an unknown style name, inline `\an2` and `\i0` that override the style, and `\r` with and without a style name. | MIT |
| `own_script_info_only.ass` | Written for this repository in the shape of a new, empty Aegisub 3.2 project: UTF-8 BOM, LF, `[Script Info]` and `[V4+ Styles]` with one style, and no `[Events]` section. | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each `.to.*` file equals the PHP write golden of the same name at the pinned commit. The `.out.*` files have no PHP golden: they hold the events in time order and an added `[Events]` section | MIT |
