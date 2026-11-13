# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/chapters/ffmetadata/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `ffmpeg_mp4_export.ffmeta` | Written for this repository in the shape of `ffmpeg -i in.mp4 -f ffmetadata` output: MP4 brand tags, `TIMEBASE=1/1000`, an escaped `;` | MIT |
| `m4b_audiobook.ffmeta` | Written for this repository in the shape of an M4B audiobook export: a `[STREAM]` section, `TIMEBASE=1/44100`, a value continued on the next line, escaped `=` | MIT |
| `hand_written_crlf.ffmeta` | Written for this repository in the shape of a hand-made file: CR LF, comments, empty lines, no `TIMEBASE`, missing `END` lines | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
